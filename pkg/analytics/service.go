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
	"strings"
	"time"

	"github.com/go-logr/logr"

	"github.com/iter8-tools/iter8-controller/pkg/analytics/api/v1alpha2"
	iter8v1alpha2 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha2"
)

const (
	destinationKey = "destination_workload"
	namespaceKey   = "destination_service_namespace"
)

// MakeRequest generates request payload to analytics
func MakeRequest(instance *iter8v1alpha2.Experiment) (*v1alpha2.Request, error) {
	// identify and define list of candidates
	candidates := make([]v1alpha2.Version, len(instance.Spec.Service.Candidates))
	for i, candidate := range instance.Spec.Candidates {
		candidates[i].ID = candidate
		candidates[i].VersionLabels = map[string]string{
			namespaceKey:   instance.ServiceNamespace(),
			destinationKey: candidate,
		}
	}

	// identify and define list of criteria
	criteria := make([]v1alpha2.Criterion, len(instance.Spec.Criteria))
	for i, criterion := range instance.Spec.Criteria {
		isReward := false
		if nil != criterion.IsReward {
			isReward = *criterion.IsReward
		}
		criteria[i] = v1alpha2.Criterion{
			ID:       criterion.Metric,
			MetricID: criterion.Metric,
			IsReward: &isReward,
		}
		if nil != criterion.Threshold {
			criteria[i].Threshold = &v1alpha2.Threshold{
				Type:  criterion.Threshold.Type,
				Value: criterion.Threshold.Value,
			}
		}
	}

	// identify and define metrics
	counterMetrics := make([]v1alpha2.CounterMetric, len(instance.Spec.Metrics.CounterMetrics))
	for i, metric := range instance.Spec.Metrics.CounterMetrics {
		counterMetrics[i] = v1alpha2.CounterMetric{
			Name:          metric.Name,
			QueryTemplate: metric.QueryTemplate,
		}
		if nil != metric.PreferredDirection {
			counterMetrics[i].PreferredDirection = metric.PreferredDirection
		}
	}
	ratioMetrics := make([]v1alpha2.RatioMetric, len(instance.Spec.Metrics.RatioMetrics))
	for i, metric := range instance.Spec.Metrics.RatioMetrics {
		ratioMetrics[i] = v1alpha2.RatioMetric{
			Name:        metric.Name,
			Numerator:   metric.Numerator,
			Denominator: metric.Denominator,
		}
		if nil != metric.PreferredDirection {
			counterMetrics[i].PreferredDirection = metric.PreferredDirection
		}
		if nil != metric.ZeroToOne {
			ratioMetrics[i].ZeroToOne = metric.ZeroToOne
		}
	}

	request := &v1alpha2.Request{
		StartTime:   instance.Status.StartTimestamp.Format(time.RFC3339),
		ServiceName: instance.Spec.Service.Name,
		Baseline: v1alpha2.Version{
			ID: instance.Spec.Service.Baseline,
			VersionLabels: map[string]string{
				namespaceKey:   instance.ServiceNamespace(),
				destinationKey: instance.Spec.Service.Baseline,
			},
		},
		MetricSpecs: v1alpha2.Metrics{
			CounterMetrics: counterMetrics,
			RatioMetrics:   ratioMetrics,
		},
		Candidate: candidates,
		Criteria:  criteria,
		TrafficControl: &v1alpha2.TrafficControl{
			MaxIncrement: float32(instance.Spec.GetMaxIncrements()),
			Strategy:     instance.Spec.GetStrategy(),
		},
	}

	if nil != instance.Status.CurrentIteration {
		request.IterationNumber = instance.Status.CurrentIteration
	}

	if nil != instance.Status.AnalysisState {
		request.LastState = instance.Status.AnalysisState
	}

	return request, nil
}

// Invoke sends payload to endpoint and gets response back
func Invoke(log logr.Logger, endpoint string, payload interface{}) (*v1alpha2.Response, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(endpoint, "/") {
		endpoint += "assessment"
	} else {
		endpoint += "/assessment"
	}
	log.Info("post", "URL", endpoint, "request", string(data))

	raw, err := http.Post(endpoint, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	defer raw.Body.Close()
	body, err := ioutil.ReadAll(raw.Body)

	log.Info("post", "URL", endpoint, "response", string(body))

	if raw.StatusCode >= 400 {
		return nil, fmt.Errorf("%v", string(body))
	}

	var response v1alpha2.Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}
