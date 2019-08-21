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

package checkandincrement

import (
	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
)

type Request struct {
	// Specifies the name of the experiment
	Name string `json:"name"`

	// Specifies a time interval and key-value pairs for retrieving and processing data pertaining to the baseline version
	Baseline Window `json:"baseline"`

	// Specifies a time interval and key-value pairs for retrieving and processing data pertaining to the canary version
	Canary Window `json:"canary"`

	// Parameters controlling the behavior of the analytics
	TrafficControl TrafficControl `json:"traffic_control"`

	// State returned by the server on the previous call
	LastState interface{} `json:"_last_state"`
}

type Window struct {
	// ISO8601 timestamp for the beginning of the time range of interest
	StartTime string `json:"start_time"`

	// ISO8601 timestamp for the end of the time range of interest; if omitted, current time is assumed
	EndTime string `json:"end_time"`

	// Key-value pairs identifying the data pertaining to a version
	Tags map[string]string `json:"tags"`
}

type TrafficControl struct {
	// Parameters controlling the behavior of the analytics
	WarmupRequestCount int `json:"warmup_request_count"`

	// Maximum percentage of traffic that the canary version will receive during the experiment; defaults to 50%
	MaxTrafficPercent float64 `json:"max_traffic_percent"`

	// Increment (in percent points) to be applied to the traffic received by the canary version
	// each time it passes the success criteria; defaults to 1 percent point
	StepSize float64 `json:"step_size"`

	// Determines how the traffic must be split at the end of the experiment; options:
	// "baseline": all traffic goes to the baseline version;
	// "canary": all traffic goes to the canary version;
	// "both": traffic is split across baseline and canary. Defaults to “canary”
	OnSuccess string `json:"on_success"`

	// List of criteria for assessing the canary version
	SuccessCriteria []SuccessCriterion `json:"success_criteria"`
}

type SuccessCriterion struct {
	// Name of the metric to which the criterion applies
	// example: iter8_latency
	MetricName string `json:"metric_name"`

	// 	Criterion type. Options:
	// "delta": compares the canary against the baseline version with respect to the metric;
	// "threshold": checks the canary with respect to the metric
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

	// Indicates whether or not this criterion is considered for traffic-control decisions;
	// defaults to true
	EnableTrafficControl bool `json:"enable_traffic_control"`

	// Indicates that this criterion is based on statistical confidence;
	// for instance, one can specify a 98% confidence that the criterion is satisfied;
	// if not specified, there is no confidence requirement
	Confidence float64 `json:"confidence"`
}

type Response struct {
	// Measurements and traffic recommendation for the baseline version
	Baseline MetricsTraffic `json:"baseline"`

	// Measurements and traffic recommendation for the baseline version
	Canary MetricsTraffic `json:"canary"`

	// Summary of the canary assessment based on success criteria
	Assessment Assessment `json:"assessment"`

	// State returned by the server, to be passed on the next call
	LastState interface{} `json:"_last_state"`
}

type Assessment struct {
	// Summary of the canary assessment based on success criteria
	Summary iter8v1alpha1.Summary `json:"summary"`

	// Summary of results for each success criterion
	SuccessCriteria []SuccessCriterionOutput `json:"success_criteria"`
}

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

type MetricsTraffic struct {
	// Measurements and traffic recommendation for the baseline version
	TrafficPercentage float64 `json:"traffic_percentage"`

	// Sate returned by the server, to be passed on the next call
	LastState interface{} `json:"_last_state"`
}
