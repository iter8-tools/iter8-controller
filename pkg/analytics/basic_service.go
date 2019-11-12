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
package analytics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
)

// BasicAnalyticsService provides a default implmenentation of interface AnalyticsService
type BasicAnalyticsService struct {
}

// Request ...
type Request struct {
	// Specifies the name of the experiment
	Name string `json:"name"`

	// Specifies a time interval and key-value pairs for retrieving and processing data pertaining to the baseline version
	Baseline Window `json:"baseline"`

	// Specifies a time interval and key-value pairs for retrieving and processing data pertaining to the candidate version
	Candidate Window `json:"candidate"`

	// Parameters controlling the behavior of the analytics
	TrafficControl TrafficControl `json:"traffic_control"`

	// State returned by the server on the previous call
	LastState interface{} `json:"_last_state"`
}

// Window ...
type Window struct {
	// ISO8601 timestamp for the beginning of the time range of interest
	StartTime string `json:"start_time"`

	// ISO8601 timestamp for the end of the time range of interest; if omitted, current time is assumed
	EndTime string `json:"end_time"`

	// Key-value pairs identifying the data pertaining to a version
	Tags map[string]string `json:"tags"`
}

// TrafficControl ...
type TrafficControl struct {
	// Parameters controlling the behavior of the analytics
	WarmupRequestCount int `json:"warmup_request_count"`

	// Maximum percentage of traffic that the candidate version will receive during the experiment; defaults to 50%
	MaxTrafficPercent float64 `json:"max_traffic_percent"`

	// Increment (in percent points) to be applied to the traffic received by the candidate version
	// each time it passes the success criteria; defaults to 1 percent point
	StepSize float64 `json:"step_size"`

	// List of criteria for assessing the candidate version
	SuccessCriteria []SuccessCriterion `json:"success_criteria"`
}

// SuccessCriterion ...
type SuccessCriterion struct {
	// Name of the metric to which the criterion applies
	// example: iter8_latency
	MetricName string `json:"metric_name"`

	// 	Criterion type. Options:
	// "delta": compares the candidate against the baseline version with respect to the metric;
	// "threshold": checks the candidate with respect to the metric
	Type iter8v1alpha1.ToleranceType `json:"type"`

	// MetricType of this metric
	MetricType string `json:"metric_type"`

	// Template specifies the query template for the metric
	Template string `json:"metric_query_template"`

	// SampleSizeTemplate specifies the query template for the sample size
	SampleSizeTemplate string `json:"metric_sample_size_query_template"`

	// Value to check
	Value float64 `json:"value"`

	// Minimum number of data points required to make a decision based on this criterion;
	// if not specified, there is no requirement on the sample size
	SampleSize int `json:"sample_size"`

	// Indicates whether or not the experiment must finish if this criterion is not satisfied;
	// defaults to false
	StopOnFailure bool `json:"stop_on_failure"`

	// Indicates that this criterion is based on statistical confidence;
	// for instance, one can specify a 98% confidence that the criterion is satisfied;
	// if not specified, there is no confidence requirement
	Confidence float64 `json:"confidence"`
}

// Response ...
type Response struct {
	// Measurements and traffic recommendation for the baseline version
	Baseline MetricsTraffic `json:"baseline"`

	// Measurements and traffic recommendation for the baseline version
	Candidate MetricsTraffic `json:"candidate"`

	// Summary of the candidate assessment based on success criteria
	Assessment Assessment `json:"assessment"`

	// State returned by the server, to be passed on the next call
	LastState interface{} `json:"_last_state"`
}

// Assessment ...
type Assessment struct {
	// Summary of the candidate assessment based on success criteria
	Summary iter8v1alpha1.Summary `json:"summary"`

	// Summary of results for each success criterion
	SuccessCriteria []SuccessCriterionOutput `json:"success_criteria"`
}

// SuccessCriterionOutput ...
type SuccessCriterionOutput struct {
	// Name of the metric to which the criterion applies
	// example: iter8_latency
	MetricName string `json:"metric_name"`

	// Overall summary based on all success criteria
	Conclusions []string `json:"conclusions"`

	// Indicates whether or not the success criterion for the corresponding metric has been met
	SuccessCriteriaMet bool `json:"success_criteria_met"`

	// Indicates whether or not the experiment must be aborted on the basis of the criterion for this metric
	AbortExperiment bool `json:"abort_experiment"`
}

// MetricsTraffic ...
type MetricsTraffic struct {
	// Measurements and traffic recommendation for the baseline version
	TrafficPercentage float64 `json:"traffic_percentage"`

	// Sate returned by the server, to be passed on the next call
	LastState interface{} `json:"_last_state"`
}

// MakeRequest ...
func (a BasicAnalyticsService) MakeRequest(instance *iter8v1alpha1.Experiment, baseline, experiment interface{}) (*Request, error) {
	spec := instance.Spec

	criteria := make([]SuccessCriterion, len(spec.Analysis.SuccessCriteria))
	for i, criterion := range spec.Analysis.SuccessCriteria {
		iter8metric, ok := instance.Metrics[criterion.MetricName]
		if !ok {
			// Metric template not found
			return nil, fmt.Errorf("Metric %s Not Available", criterion.MetricName)
		}
		criteria[i] = SuccessCriterion{
			MetricName:         criterion.MetricName,
			Type:               criterion.ToleranceType,
			Value:              criterion.Tolerance,
			Template:           iter8metric.QueryTemplate,
			SampleSizeTemplate: iter8metric.SampleSizeTemplate,
			MetricType:         iter8metric.Type,
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
		Name: instance.Name,
		Baseline: Window{
			StartTime: instance.ObjectMeta.GetCreationTimestamp().Format(time.RFC3339),
			EndTime:   now,
			Tags: map[string]string{
				destinationKey: baseVal,
				namespaceKey:   baseNsVal,
			},
		},
		Candidate: Window{
			StartTime: instance.ObjectMeta.GetCreationTimestamp().Format(time.RFC3339),
			EndTime:   now,
			Tags: map[string]string{
				destinationKey: experimentVal,
				namespaceKey:   experimentNsVal,
			},
		},
		TrafficControl: TrafficControl{
			MaxTrafficPercent: instance.Spec.TrafficControl.GetMaxTrafficPercentage(),
			StepSize:          instance.Spec.TrafficControl.GetStepSize(),
			SuccessCriteria:   criteria,
		},
		LastState: instance.Status.AnalysisState,
	}, nil
}

// Invoke ...
func (a BasicAnalyticsService) Invoke(log logr.Logger, endpoint string, payload *Request, path string) (*Response, error) {
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

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// GetPath ...
func (a BasicAnalyticsService) GetPath() string {
	return ""
}
