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

	"github.com/iter8-tools/iter8-controller/pkg/analytics/algorithm"
	"github.com/iter8-tools/iter8-controller/pkg/analytics/api"
	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
)

const (
	destinationWorkloadKey          = "destination_workload"
	destinationWorkloadNamespaceKey = "destination_workload_namespace"

	destinationServiceNameKey      = "destination_service_name"
	destinationServiceNamespaceKey = "destination_service_namespace"
)

// MakeRequest generates request payload to analytics
func MakeRequest(instance *iter8v1alpha1.Experiment, impl algorithm.Interface) (*api.Request, error) {
	spec := instance.Spec

	criteria := make([]api.SuccessCriterion, len(spec.Analysis.SuccessCriteria))
	for i, criterion := range spec.Analysis.SuccessCriteria {
		iter8metric, ok := instance.Metrics[criterion.MetricName]
		if !ok {
			// Metric template not found
			return nil, fmt.Errorf("Metric %s Not Available", criterion.MetricName)
		}
		apiSC := api.SuccessCriterion{
			api.SCKeyMetricName:         criterion.MetricName,
			api.SCKeyType:               criterion.ToleranceType,
			api.SCKeyValue:              criterion.Tolerance,
			api.SCKeyTemplate:           iter8metric.QueryTemplate,
			api.SCKeySampleSizeTemplate: iter8metric.SampleSizeTemplate,
			api.SCKeyIsCounter:          iter8metric.IsCounter,
			api.SCKeyAbsentValue:        iter8metric.AbsentValue,
			api.SCKeyStopOnFailure:      criterion.GetStopOnFailure(),
		}

		var err error
		if apiSC, err = impl.SupplementSuccessCriteria(criterion, apiSC); err != nil {
			return nil, err
		}
		criteria[i] = apiSC
	}

	now := time.Now().Format(time.RFC3339)

	tc := api.TrafficControl{
		api.TCKeyMaxTrafficPercent: instance.Spec.TrafficControl.GetMaxTrafficPercentage(),
		api.TCKeySuccessCriteria:   criteria,
	}

	tc = impl.SupplementTrafficControl(instance, tc)

	if rw := instance.Spec.Analysis.Reward; rw != nil {
		iter8metric, ok := instance.Metrics[rw.MetricName]
		if !ok {
			// Metric template not found
			return nil, fmt.Errorf("Metric %s Not Available", rw.MetricName)
		}
		reward := api.SuccessCriterion{
			api.SCKeyMetricName:         rw.MetricName,
			api.SCKeyTemplate:           iter8metric.QueryTemplate,
			api.SCKeySampleSizeTemplate: iter8metric.SampleSizeTemplate,
			api.SCKeyIsCounter:          iter8metric.IsCounter,
			api.SCKeyAbsentValue:        iter8metric.AbsentValue,
		}

		if rw.MinMax != nil {
			reward[api.SCKeyMinMax] = rw.MinMax
		}

		tc[api.TCKeyReward] = reward
	}

	destinationKey := destinationWorkloadKey
	destinationNamespaceKey := destinationWorkloadNamespaceKey

	if instance.Spec.TargetService.Kind == "Service" {
		destinationKey = destinationServiceNameKey
		destinationNamespaceKey = destinationServiceNamespaceKey
	}

	serviceNamespace := instance.ServiceNamespace()
	return &api.Request{
		Name: instance.Name,
		Baseline: api.Window{
			StartTime: time.Unix(0, instance.Status.StartTimestamp).Format(time.RFC3339),
			EndTime:   now,
			Tags: map[string]string{
				destinationKey:          instance.Spec.TargetService.Baseline,
				destinationNamespaceKey: serviceNamespace,
			},
		},
		Candidate: api.Window{
			StartTime: time.Unix(0, instance.Status.StartTimestamp).Format(time.RFC3339),
			EndTime:   now,
			Tags: map[string]string{
				destinationKey:          instance.Spec.TargetService.Candidate,
				destinationNamespaceKey: serviceNamespace,
			},
		},
		LastState:      instance.Status.AnalysisState,
		TrafficControl: tc,
	}, nil
}

// Invoke sends payload to endpoint and gets response back
func Invoke(log logr.Logger, endpoint string, payload interface{}, impl algorithm.Interface) (*api.Response, error) {
	path := impl.GetPath()

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

	var response api.Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}
