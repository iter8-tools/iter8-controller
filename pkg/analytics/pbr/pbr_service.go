/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package pbr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/iter8-tools/iter8-controller/pkg/analytics"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
)

// PbrAnalyticsService provides a default implmenentation of interface AnalyticsService
type PbrAnalyticsService struct {
	analytics.BasicAnalyticsService
}

// // Request ...
// type Request struct {
// 	// Specifies the name of the experiment
// 	Name string `json:"name"`

// 	// Specifies a time interval and key-value pairs for retrieving and processing data pertaining to the baseline version
// 	Baseline analytics.Window `json:"baseline"`

// 	// Specifies a time interval and key-value pairs for retrieving and processing data pertaining to the candidate version
// 	Candidate analytics.Window `json:"candidate"`

// 	// Parameters controlling the behavior of the analytics
// 	TrafficControl TrafficControl `json:"traffic_control"`

// 	// State returned by the server on the previous call
// 	LastState interface{} `json:"_last_state"`
// }

// Request ...
type Request struct {
	analytics.RequestCommon
	// Parameters controlling the behavior of the analytics
	TrafficControl TrafficControl `json:"traffic_control"`
}

// TrafficControl ...
type TrafficControl struct {
	analytics.TrafficControl

	// PosteriorSampleSize required sample size
	PosteriorSampleSize int `json:"posterior_sample_size"`

	// NumberOfTrials number of values sampled per iteration from each distribution
	NumberOfTrials float64 `json:"no_of_trials"`
}

// MakeRequest ...
func (a PbrAnalyticsService) MakeRequest(instance *iter8v1alpha1.Experiment, baseline, experiment interface{}) (interface{}, error) {
	spec := instance.Spec

	criteria := make([]analytics.SuccessCriterion, len(spec.Analysis.SuccessCriteria))
	for i, criterion := range spec.Analysis.SuccessCriteria {
		iter8metric, ok := instance.Metrics[criterion.MetricName]
		if !ok {
			// Metric template not found
			return nil, fmt.Errorf("Metric %s Not Available", criterion.MetricName)
		}
		criteria[i] = analytics.SuccessCriterion{
			MetricName:         criterion.MetricName,
			Type:               criterion.ToleranceType,
			Value:              criterion.Tolerance,
			Template:           iter8metric.QueryTemplate,
			SampleSizeTemplate: iter8metric.SampleSizeTemplate,
			IsCounter:          iter8metric.IsCounter,
			AbsentValue:        iter8metric.AbsentValue,
		}

		criteria[i].SampleSize = criterion.GetSampleSize()
		criteria[i].StopOnFailure = criterion.GetStopOnFailure()
	}
	now := time.Now().Format(time.RFC3339)
	destinationKey, namespaceKey, baseVal, experimentVal, baseNsVal, experimentNsVal := "", "", "", "", "", ""
	switch instance.Spec.TargetService.APIVersion {
	case "v1":
		destinationKey = "destination_workload"
		namespaceKey = "destination_service_namespace"
		baseVal = baseline.(*appsv1.Deployment).GetName()
		experimentVal = experiment.(*appsv1.Deployment).GetName()
		baseNsVal = baseline.(*appsv1.Deployment).GetNamespace()
		experimentNsVal = experiment.(*appsv1.Deployment).GetNamespace()
	case "serving.knative.dev/v1alpha1":
		destinationKey = "destination_service_name"
		namespaceKey = "destination_service_namespace"
		baseVal = baseline.(*corev1.Service).GetName()
		experimentVal = experiment.(*corev1.Service).GetName()
		baseNsVal = baseline.(*corev1.Service).GetNamespace()
		experimentNsVal = experiment.(*corev1.Service).GetNamespace()
	default:
		return nil, fmt.Errorf("Unsupported API Version %s", instance.Spec.TargetService.APIVersion)
	}

	return &Request{
		RequestCommon: analytics.RequestCommon{
			Name: instance.Name,
			Baseline: analytics.Window{
				StartTime: instance.ObjectMeta.GetCreationTimestamp().Format(time.RFC3339),
				EndTime:   now,
				Tags: map[string]string{
					destinationKey: baseVal,
					namespaceKey:   baseNsVal,
				},
			},
			Candidate: analytics.Window{
				StartTime: instance.ObjectMeta.GetCreationTimestamp().Format(time.RFC3339),
				EndTime:   now,
				Tags: map[string]string{
					destinationKey: experimentVal,
					namespaceKey:   experimentNsVal,
				},
			},
			LastState: instance.Status.AnalysisState,
		},
		TrafficControl: TrafficControl{
			TrafficControl: analytics.TrafficControl{
				MaxTrafficPercent: instance.Spec.TrafficControl.GetMaxTrafficPercentage(),
				StepSize:          instance.Spec.TrafficControl.GetStepSize(),
				SuccessCriteria:   criteria,
			},
			PosteriorSampleSize: 1000,
			NumberOfTrials:      10.0,
		},
	}, nil
}

// Invoke ...
func (a PbrAnalyticsService) Invoke(log logr.Logger, endpoint string, payload interface{}, path string) (*analytics.Response, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	log.Info("post", "URL", endpoint+path, "request", string(data))

	raw, err := http.Post(endpoint+path, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	defer raw.Body.Close()
	body, err := ioutil.ReadAll(raw.Body)

	log.Info("post", "URL", endpoint+path, "response", string(body))

	if raw.StatusCode >= 400 {
		return nil, fmt.Errorf("%v", string(body))
	}

	var response analytics.Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// GetPath ...
func (a PbrAnalyticsService) GetPath() string {
	return ""
}
