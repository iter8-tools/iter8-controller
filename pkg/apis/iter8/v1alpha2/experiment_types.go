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

	analyticsv1alpha2 "github.com/iter8-tools/iter8/pkg/analytics/api/v1alpha2"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Experiment contains the sections for --
// defining an experiment,
// showing experiment status,
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,iter8
// +kubebuilder:printcolumn:name="type",type="string",JSONPath=".status.experimentType",description="Type of experiment",format="byte"
// +kubebuilder:printcolumn:name="hosts",type="string",JSONPath=".status.effectiveHosts",description="Names of candidates",format="byte"
// +kubebuilder:printcolumn:name="phase",type="string",JSONPath=".status.phase",description="Phase of the experiment",format="byte"
// +kubebuilder:printcolumn:name="winner found",type="boolean",JSONPath=".status.assessment.winner.winning_version_found",description="Winner identified",format="byte"
// +kubebuilder:printcolumn:name="current best",type="string",JSONPath=".status.assessment.winner.name",description="Current best version",format="byte"
// +kubebuilder:printcolumn:name="confidence",priority=1,type="string",JSONPath=".status.assessment.winner.probability_of_winning_for_best_version",description="Confidence current bets version will be the winner",format="float"
// +kubebuilder:printcolumn:name="status",type="string",JSONPath=".status.message",description="Detailed Status of the experiment",format="byte"
// +kubebuilder:printcolumn:name="baseline",priority=1,type="string",JSONPath=".spec.service.baseline",description="Name of baseline",format="byte"
// +kubebuilder:printcolumn:name="candidates",priority=1,type="string",JSONPath=".spec.service.candidates",description="Names of candidates",format="byte"
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
	// Service is a reference to the service componenets that this experiment is targeting at
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

	// Cleanup indicates whether routing rules and deployment receiving no traffic should be deleted at the end of experiment
	// +optional
	Cleanup *bool `json:"cleanup,omitempty"`

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

	// List of hosts related to this service
	Hosts []Host `json:"hosts,omitempty"`

	// Port number exposed by internal services
	Port *int32 `json:"port,omitempty"`
}

// Host holds the name of host and gateway associated with it
type Host struct {
	// Name of the Host
	Name string `json:"name"`

	// The gateway associated with the host
	Gateway string `json:"gateway"`
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

// TrafficControl specifies constrains on traffic and stratgy used to update the traffic
type TrafficControl struct {
	// Strategy used to shift traffic
	// default is progressive
	// +kubebuilder:validation:Enum={progressive, top_2, uniform}
	// +optional
	Strategy *StrategyType `json:"strategy,omitempty"`

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

	// RouterID refers to the id of router used to handle traffic for the experiment
	// If it's not specified, the first entry of effictive host will be used as the id
	// +optional
	RouterID *string `json:"routerID,omitempty"`
}

// Match contains matching criteria for requests
type Match struct {
	// Matching criteria for HTTP requests
	// +optional
	HTTP []*HTTPMatchRequest `json:"http,omitempty"`
}

// ManualOverride defines actions that the user can perform to an experiment
type ManualOverride struct {
	// Action to perform
	//+kubebuilder:validation:Enum={pause,resume,terminate}
	Action ActionType `json:"action"`
	// Traffic split status specification
	// Applied to action terminate only
	// example:
	//   reviews-v2:80
	//   reviews-v3:20
	// +optional
	TrafficSplit map[string]int32 `json:"trafficSplit,omitempty"`
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
	// Name of metric
	Name string `json:"name" yaml:"name"`

	// Query template of this metric
	QueryTemplate string `json:"query_template" yaml:"query_template"`

	// Preferred direction of the metric value
	// +optional
	PreferredDirection *string `json:"preferred_direction,omitempty" yaml:"preferred_direction,omitempty"`

	// Unit of the metric value
	// +optional
	Unit *string `json:"unit,omitempty" yaml:"unit,omitempty"`
}

// RatioMetric is the definiton of Ratio Metric
type RatioMetric struct {
	// name of metric
	Name string `json:"name" yaml:"name"`

	// Counter metric used in numerator
	Numerator string `json:"numerator" yaml:"numerator"`

	// Counter metric used in denominator
	Denominator string `json:"denominator" yaml:"denominator"`

	// Boolean flag indicating if the value of this metric is always in the range 0 to 1
	// +optional
	ZeroToOne *bool `json:"zero_to_one,omitempty" yaml:"zero_to_one,omitempty"`

	// Preferred direction of the metric value
	// +optional
	PreferredDirection *string `json:"preferred_direction,omitempty" yaml:"preferred_direction,omitempty"`
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

	// LastUpdateTime is the last time iteration has been updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// CurrentIteration is the current iteration number
	// +optional
	CurrentIteration *int32 `json:"currentIteration,omitempty"`

	// GrafanaURL is the url to the Grafana Dashboard
	// +optional
	GrafanaURL *string `json:"grafanaURL,omitempty"`

	// Assessment returned by the last analyis
	// +optional
	Assessment *Assessment `json:"assessment,omitempty"`

	// Phase marks the Phase the experiment is at
	// +optional
	Phase PhaseType `json:"phase,omitempty"`

	// Message specifies message to show in the kubectl printer
	// +optional
	Message *string `json:"message,omitempty"`

	// AnalysisState is the last recorded analysis state
	// +optional
	AnalysisState *runtime.RawExtension `json:"analysisState,omitempty"`

	// ExperimentType is type of experiment
	ExperimentType string `json:"experimentType,omitempty"`

	// EffectiveHosts is computed host for experiment.
	// List of spec.Service.Name and spec.Service.Hosts[0].name
	EffectiveHosts []string `json:"effectiveHosts,omitempty"`
}

// Conditions is a list of ExperimentConditions
type Conditions []*ExperimentCondition

// ExperimentCondition describes a condition of an experiment
type ExperimentCondition struct {
	// Type of the condition
	Type ExperimentConditionType `json:"type"`

	// Status of the condition
	Status corev1.ConditionStatus `json:"status"`

	// The time when this condition is last updated
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason for the last update
	// +optional
	Reason *string `json:"reason,omitempty"`

	// Detailed explanation on the update
	// +optional
	Message *string `json:"message,omitempty"`
}

// Assessment details for the each target
type Assessment struct {
	// Assessment details of baseline
	Baseline VersionAssessment `json:"baseline"`

	// Assessment details of each candidate
	Candidates []VersionAssessment `json:"candidates"`

	// Assessment for winner target if exists
	Winner *WinnerAssessment `json:"winner,omitempty"`
}

// WinnerAssessment shows assessment details for winner of an experiment
type WinnerAssessment struct {
	// name of winner version
	// +optional
	Name *string `json:"name,omitempty"`

	// Assessment details from analytics
	*analyticsv1alpha2.WinnerAssessment `json:",inline,omitempty"`
}

// VersionAssessment contains assessment details for each version
type VersionAssessment struct {
	// name of version
	Name string `json:"name"`

	// Weight of traffic
	Weight int32 `json:"weight"`

	// Assessment details from analytics
	analyticsv1alpha2.VersionAssessment `json:",inline"`

	// A flag indicates whether traffic to this target should be cutoff
	// +optional
	Rollback bool `json:"rollback,omitempty"`
}
