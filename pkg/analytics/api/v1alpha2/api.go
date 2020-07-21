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
package v1alpha2

// Request defines payload to analytics service
type Request struct {
	// Name of experiment
	Name string `json:"name"`

	// ISO8601 timestamp for the beginning of the time range of interest
	StartTime string `json:"start_time"`

	// Name of the service
	ServiceName string `json:"service_name"`

	// Current iteration of the experiment
	IterationNumber *int32 `json:"iteration_number,omitempty"`

	// All metric specification
	MetricSpecs Metrics `json:"metric_specs"`

	// Criteria to be assessed for each version in this experiment
	Criteria []Criterion `json:"criteria"`

	// Baseline verison details
	Baseline Version `json:"baseline"`

	// Candidate versions details
	Candidate []Version `json:"candidates"`

	// State returned by the server on the previous call
	LastState interface{} `json:"last_state,omitempty"`

	// Parameters controlling the behavior of the analytics
	TrafficControl *TrafficControl `json:"traffic_control,omitempty"`
}

// Version specifies details of a version
type Version struct {
	// Name of the version
	ID string `json:"id"`

	// labels for the version
	VersionLabels map[string]string `json:"version_labels"`
}

// CounterMetric is the definition of Counter Metric
type CounterMetric struct {
	// Unique identifier
	Name string `json:"name"`

	// Direction indicating which values are "better"
	//+kubebuilder:validation:Enum={lower,higher}
	PreferredDirection *string `json:"preferred_direction,omitempty"`

	// Descriptive short name
	DescriptiveShortName *string `json:"descriptive_short_name,omitempty"`

	// Query template of this metric
	QueryTemplate string `json:"query_template"`
}

// RatioMetric is the definiton of Ratio Metric
type RatioMetric struct {
	// Unique identifier
	Name string `json:"name"`

	// Direction indicating which values are "better"
	//+kubebuilder:validation:Enum={lower,higher}
	PreferredDirection *string `json:"preferred_direction,omitempty"`

	// Descriptive short name
	DescriptiveShortName *string `json:"descriptive_short_name,omitempty"`

	// Counter metric used in numerator
	Numerator string `json:"numerator"`

	// Counter metric used in denominator
	Denominator string `json:"denominator"`

	// Boolean flag indicating if the value of this metric is always in the range 0 to 1
	// +optional
	ZeroToOne *bool `json:"zero_to_one,omitempty"`
}

// Metrics details
type Metrics struct {
	CounterMetrics []CounterMetric `json:"counter_metrics"`
	RatioMetrics   []RatioMetric   `json:"ratio_metrics"`
}

// Threshold details
type Threshold struct {
	Type  string  `json:"threshold_type"`
	Value float32 `json:"value"`
}

// Criterion includes an assessment details for each version
type Criterion struct {
	ID        string     `json:"id"`
	MetricID  string     `json:"metric_id"`
	IsReward  *bool      `json:"is_reward,omitempty"`
	Threshold *Threshold `json:"threshold,omitempty"`
}

// TrafficControl details
type TrafficControl struct {
	// Maximum possible increment in a candidate's traffic during the initial phase of the experiment
	MaxIncrement float32 `json:"max_increment"`

	// Traffic split algorithm to use during the experiment
	Strategy string `json:"strategy"`
}

// Response from analytics
type Response struct {
	// Timestamp when assessment is made
	Timestamp string `json:"timestamp"`

	// Assessment for baseline
	BaselineAssessment VersionAssessment `json:"baseline_assessment"`

	// Assessment for candidates
	CandidateAssessments []CandidateAssessment `json:"candidate_assessments"`

	// TrafficSplitRecommendation contains traffic split recommendations for versions from each algorithm
	// The map is interprted as {name_of_algorithm: {name_of_version: traffic_amount}}
	TrafficSplitRecommendation map[string]map[string]int32 `json:"traffic_split_recommendation"`

	// Assessment for winner
	WinnerAssessment `json:"winner_assessment"`

	// Status of analytics engine
	Status *[]string `json:"status,omitempty"`

	// Human-readable explanation of the status
	StatusInterpretations *map[string]string `json:"status_interpretations,omitempty"`

	// Last recorded state from analytics service
	LastState *interface{} `json:"last_state,omitempty"`
}

// VersionAssessment contains assessment details for a version
type VersionAssessment struct {
	ID                   string                `json:"id"`
	WinProbability       float32               `json:"win_probability"`
	RequestCount         int32                 `json:"request_count"`
	CriterionAssessments []CriterionAssessment `json:"criterion_assessments,omitempty"`
}

// CriterionAssessment contains assessment for a version
type CriterionAssessment struct {
	// Id of version
	ID string `json:"id"`

	// ID of metric
	MetricID string `json:"metric_id"`

	//Statistics for this metric
	Statistics *Statistics `json:"statistics,omitempty"`

	// Assessment of how well this metric is doing with respect to threshold.
	// Defined only for metrics with a threshold
	ThresholdAssessment *ThresholdAssessment `json:"threshold_assessment,omitempty"`
}

// Statistics for a metric
type Statistics struct {
	Value           *float32         `json:"value,omitempty"`
	RatioStatistics *RatioStatistics `json:"ratio_statitics,omitempty"`
}

// RatioStatistics is statistics for a ratio metric
type RatioStatistics struct {
	ImprovementOverBaseline       Interval `json:"improvement_over_baseline"`
	ProbabilityOfBeatingBaseline  float32  `json:"probability_of_beating_baseline"`
	ProbabilityOfBeingBestVersion float32  `json:"probability_of_being_best_version"`
	CredibleInterval              Interval `json:"credible_interval"`
}

// Interval for probability
type Interval struct {
	Lower float32 `json:"lower"`
	Upper float32 `json:"upper"`
}

// ThresholdAssessment is the assessment for a threshold
type ThresholdAssessment struct {
	// A flag indicating whether threshold is breached
	ThresholdBreached bool `json:"threshold_breached"`
	// Probability of satisfying the threshold.
	// Defined only for ratio metrics. This is currently computed based on Bayesian estimation
	ProbabilityOfSatisfyingTHreshold float32 `json:"probability_of_satisfying_threshold"`
}

// CandidateAssessment contains assessment for a candidate
type CandidateAssessment struct {
	VersionAssessment `json:",inline"`
	// A flag indicates whether traffic to this candidate should be cutoff
	Rollback bool `json:"rollback"`
}

// WinnerAssessment contains assessment for a winner version
type WinnerAssessment struct {
	// Indicates whether or not a clear winner has emerged
	// This is currently computed based on Bayesian estimation and uses posterior_probability_for_winner from the iteration parameters
	WinnerFound bool `json:"winning_version_found"`

	// ID of the current winner with the maximum probability of winning.
	// This is currently computed based on Bayesian estimation
	Winner *string `json:"current_best_version,omitempty"`

	//Posterior probability of the version declared as the current winner.
	// This is None if winner is None. This is currently computed based on Bayesian estimation
	Probability *float32 `json:"probability_of_winning_for_best_version,omitempty"`
}
