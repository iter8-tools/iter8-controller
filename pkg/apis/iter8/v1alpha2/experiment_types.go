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

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"

	analyticsv1alpha2 "github.com/iter8-tools/iter8-controller/pkg/analytics/api/v1alpha2"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Experiment contains the sections for --
// defining an experiment,
// showing experiment status,
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,iter8
type Experiment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ExperimentSpec `json:"spec"`
	// +optional
	Status ExperimentStatus `json:"status,omitempty"`
}

// ExperimentList contains a list of Experiment
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ExperimentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Experiment `json:"items"`
}

// ExperimentSpec defines the desired state of Experiment
type ExperimentSpec struct {
	// Service is a reference to the service that this experiment is targeting at
	Service `json:"service"`

	// Criteria contains a list of Criterion for assessing the target service
	// Noted that at most one reward metric is allowed
	// If more than one reward criterion is included, the first would be used while others would be omitted
	// +optional
	Criteria []Criterion `json:"criteria,omitempty"`

	// TrafficControl provides instructions on traffic management for an experiment
	// +optional
	TrafficControl *TrafficControl `json:"trafficControl,omitempty"`

	// Endpoint of reaching analytics service
	// default is http://iter8-analytics.iter8:8080
	// +optional
	AnalyticsEndpoint *string `json:"analyticsEndpoint,omitempty"`

	// Duration specifies how often/many times the expriment should re-evaluate the assessment
	// +optional
	Duration *Duration `json:"duration,omitempty"`

	// RoutingReference provides references to routing rules set by users
	// +optional
	RoutingReference *corev1.ObjectReference `json:"routingReference,omitempty"`

	// Cleanup indicates whether routing rules and deployment receiving no traffic should be deleted at the end of experiment
	// +optional
	Cleanup *bool `json:"cleanup"`

	// The metrics used in the experiment
	// +optional
	Metrics *Metrics `json:"metrics,omitempty"`

	// User actions to override the current status of the experiment
	// +optional
	ManualOverride *ManualOverride `json:"manualOverride,omitempty"`
}

// Service is a reference to the service that this experiment is targeting at
type Service struct {
	// defines the object reference to the service
	*corev1.ObjectReference `json:",inline"`

	// Name of the baseline deployment
	Baseline string `json:"baseline"`

	// List of names of candidate deployments
	Candidates []string `json:"candidates"`
}

// Criterion defines the criterion for assessing a target
type Criterion struct {
	// Name of metric used in the assessment
	Metric string `json:"metric"`

	// Threshold specifies the numerical value for a success criterion
	// Metric value above threhsold violates the criterion
	// +optional
	Threshold *Threshold `json:"threshold,omitempty"`

	// IsReward indicates whether the metric is a reward metric or not
	// +optional
	IsReward *bool `json:"isReward,omitempty"`
}

// Threshold defines the value and type of a criterion threshold
type Threshold struct {
	// Type of threshold
	// relative: value of threshold specifies the relative amount of changes
	// absolute: value of threshold indicates an absolute value
	//+kubebuilder:validation:Enum={relative,absolute}
	Type string `json:"type"`

	// Value of threshold
	Value float32 `json:"value"`

	// Once a target metric violates this threshold, traffic to the target should be cutoff or not
	// +optional
	CutoffTrafficOnViolation *bool `json:"cutoffTrafficOnViolation,omitempty"`
}

// Duration specifies how often/many times the expriment should re-evaluate the assessment
type Duration struct {
	// Interval specifies duration between iterations
	// default is 30s
	// +optional
	Interval *string `json:"interval,omitempty"`
	// MaxIterations indicates the amount of iteration
	// default is 100
	// +optional
	MaxIterations *int32 `json:"maxIterations,omitempty"`
}

// OnTerminationType is type of onTermination field
type OnTerminationType string

const (
	// OnTerminationToWinner indicates all traffic should go to winner candidate when experiment is terminated
	OnTerminationToWinner OnTerminationType = "to_winner"

	// OnTerminationToBaseline indicates all traffic should go to baseline when experiment is terminated
	OnTerminationToBaseline OnTerminationType = "to_baseline"

	// OnTerminationKeepLast keeps the last traffic status when experiment is terminated
	OnTerminationKeepLast OnTerminationType = "keep_last"
)

// TrafficControl specifies constrains on traffic and stratgy used to update the traffic
type TrafficControl struct {
	// Strategy used to shift traffic
	// default is progressive
	// +kubebuilder:validation:Enum={progressive, top_2, uniform}
	// +optional
	Strategy *string `json:"strategy,omitempty"`

	// OnTermination determines traffic split status at the end of experiment
	// +kubebuilder:validation:Enum={to_winner,to_baseline,keep_last}
	// +optional
	OnTermination *OnTerminationType `json:"onTermination,omitempty"`

	// Only requests fulfill the match section would be used in experiment
	// Istio matching rules are used
	// +optional
	Match *Match `json:"match,omitempty"`

	// Percentage specifies the amount of traffic to service that would be used in experiment
	// default is 100
	// +optional
	Percentage *int32 `json:"percentage,omitempty"`

	// MaxIncrement is the upperlimit of traffic increment for a target in one iteration
	// default is 2
	// +optional
	MaxIncrement *int32 `json:"maxIncrement,omitempty"`
}

// Match contains matching criteria for requests
type Match struct {
	// Matching criteria for HTTP requests
	// +optional
	HTTP []*HTTPMatchRequest `json:"http,omitempty"`
}

// ActionType is type of an Action
type ActionType string

const (
	// ActionPause is an action to pause the experiment
	ActionPause ActionType = "pause"

	// ActionResume is an action to resume the experiment
	ActionResume ActionType = "resume"

	// ActionTerminate is an action to terminate the experiment
	ActionTerminate ActionType = "terminate"
)

// ManualOverride defines actions that the user can perform to an experiment
type ManualOverride struct {
	// Action to perform
	//+kubebuilder:validation:Enum={pause,resume,terminate}
	Action ActionType `json:"action"`
	// Traffic split status specification
	// Applied to action terminate only
	// example:
	// - reviews-v2:80
	// - reviews-v3:20
	// +optional
	TrafficSplit []string `json:"trafficSplit,omitempty"`
}

// Metrics contains definitions for metrics used in the experiment
type Metrics struct {
	// List of counter metrics definiton
	// +optional
	CounterMetrics []CounterMetric `json:"counter_metrics,omitempty"`

	// List of ratio metrics definiton
	// +optional
	RatioMetrics []RatioMetric `json:"ratio_metrics,omitempty"`
}

// CounterMetric is the definition of Counter Metric
type CounterMetric struct {
	QueryTemplate string `json:"query_template"`
}

// RatioMetric is the definiton of Ratio Metric
type RatioMetric struct {
	// Counter metric used in numerator
	Numerator string `json:"numerator"`

	// Counter metric used in denominator
	Denominator string `json:"denominator"`

	// Boolean flag indicating if the value of this metric is always in the range 0 to 1
	// +optional
	ZeroToOne *bool `json:"zero_to_one,omitempty"`
}

// ExperimentStatus defines the observed state of Experiment
type ExperimentStatus struct {
	// List of conditions
	// +optional
	Conditions Conditions `json:"conditions,omitempty"`

	// InitTimestamp is the timestamp when the experiment is initialized
	// +optional
	InitTimestamp *metav1.Time `json:"initTimestamp,omitempty"`

	// StartTimestamp is the timestamp when the experiment starts
	// +optional
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// EndTimestamp is the timestamp when experiment completes
	// +optional
	EndTimestamp *metav1.Time `json:"endTimestamp,omitempty"`

	// LastIncrementTime is the last time the traffic has been incremented
	// +optional
	LastIncrementTime *metav1.Time `json:"lastIncrementTime,omitempty"`

	// CurrentIteration is the current iteration number
	// +optional
	CurrentIteration *int32 `json:"currentIteration,omitempty"`

	// GrafanaURL is the url to the Grafana Dashboard
	// +optional
	GrafanaURL *string `json:"grafanaURL,omitempty"`

	// Assessment returned by the last analyis
	// +optional
	Assessment *Assessment `json:"assessment,omitempty"`

	// TrafficSplit tells the current traffic spliting among targets
	// +optional
	TrafficSplit []TrafficSplit `json:"trafficSplit,omitempty"`

	// Phase marks the Phase the experiment is at
	// +optional
	Phase *Phase `json:"phase,omitempty"`

	// Message specifies message to show in the kubectl printer
	// +optional
	Message *string `json:"message,omitempty"`

	// AnalysisState is the last recorded analysis state
	// +optional
	AnalysisState *runtime.RawExtension `json:"analysisState,omitempty"`
}

// Conditions is a list of ExperimentConditions
type Conditions []ExperimentCondition

// ExperimentCondition describes a condition of an experiment
type ExperimentCondition struct {
	// Type of the condition
	Type ExperimentConditionType `json:"name"`

	// Status of the condition
	Status corev1.ConditionStatus `json:"status"`

	// The time when this condition is last updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// Reason for the last update
	// +optional
	Reason *string `json:"reason,omitempty"`

	// Detailed explanation on the update
	// +optional
	Message *string `json:"message,omitempty"`
}

// ExperimentConditionType is type of ExperimentCondition
type ExperimentConditionType string

const (
	// ExperimentConditionTargetsProvided has status True when the Experiment detects all elements specified in targetService
	ExperimentConditionTargetsProvided ExperimentConditionType = "TargetsProvided"

	// ExperimentConditionAnalyticsServiceNormal has status True when the analytics service is operating normally
	ExperimentConditionAnalyticsServiceNormal ExperimentConditionType = "AnalyticsServiceNormal"

	// ExperimentConditionMetricsSynced has status True when metrics are successfully synced with config map
	ExperimentConditionMetricsSynced ExperimentConditionType = "MetricsSynced"

	// ExperimentConditionExperimentCompleted has status True when the experiment is completed
	ExperimentConditionExperimentCompleted ExperimentConditionType = "ExperimentCompleted"

	// ExperimentConditionRoutingRulesReady has status True when routing rules are ready
	ExperimentConditionRoutingRulesReady ExperimentConditionType = "RoutingRulesReady"
)

// Phase the experiment is in
type Phase string

const (
	// PhasePause indicates experiment is paused
	PhasePause Phase = "Pause"
	// PhaseProgressing indicates experiment is progressing
	PhaseProgressing Phase = "Progressing"
	// PhaseCompleted indicates experiment has competed (successfully or not)
	PhaseCompleted Phase = "Completed"
)

// Assessment details for the each target
type Assessment struct {
	// Assessment for baseline
	Baseline analyticsv1alpha2.VersionAssessment `json:"baseline"`

	// Assessment for candidates
	Candidates []CandidateAssessment `json:"candidates"`

	// Assessment for winner target if exists
	// +optional
	Winner *analyticsv1alpha2.WinnerAssessment `json:"winner,omitempty"`
}

// CandidateAssessment contains name of candidate and assessment details from analytics
type CandidateAssessment struct {
	// name of candidate
	Name string `json:"name"`

	// Assessment details from analytics
	analyticsv1alpha2.CandidateAssessment `json:",inline"`
}

// TrafficSplit shows traffic for a target
type TrafficSplit struct {
	// Name of deployment
	Name string `json:"name"`

	// Weight of traffic
	Weight int32 `json:"weight"`
}
