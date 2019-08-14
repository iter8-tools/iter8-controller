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

package v1alpha1

import (
	"fmt"
	"time"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Experiment is the Schema for the experiments API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,iter8
// +kubebuilder:printcolumn:name="phase",type="string",JSONPath=".status.phase",description="Phase of the experiment",format="byte"
// +kubebuilder:printcolumn:name="status",type="string",JSONPath=".status.message",description="Detailed Status of the experiment",format="byte"
// +kubebuilder:printcolumn:name="baseline",type="string",JSONPath=".spec.targetService.baseline",description="Name of baseline",format="byte"
// +kubebuilder:printcolumn:name="percentage",type="integer",JSONPath=".status.trafficSplitPercentage.baseline",description="Traffic percentage for baseline",format="int32"
// +kubebuilder:printcolumn:name="candidate",type="string",JSONPath=".spec.targetService.candidate",description="Name of candidate",format="byte"
// +kubebuilder:printcolumn:name="percentage",type="integer",JSONPath=".status.trafficSplitPercentage.candidate",description="Traffic percentage for candidate",format="int32"
type Experiment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec    ExperimentSpec    `json:"spec,omitempty"`
	Status  ExperimentStatus  `json:"status,omitempty"`
	Metrics ExperimentMetrics `json:"metrics,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ExperimentList contains a list of Experiment
type ExperimentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Experiment `json:"items"`
}

type AssessmentType string

const (
	AssessmentOverrideSuccess AssessmentType = "override_success"
	AssessmentOverrideFailure AssessmentType = "override_failure"
	AssessmentNull            AssessmentType = ""
)

// ExperimentSpec defines the desired state of Experiment
type ExperimentSpec struct {
	// TargetService is a reference to an object to use as target service
	TargetService TargetService `json:"targetService"`

	// TrafficControl defines parameters for controlling the traffic
	// +optional
	TrafficControl TrafficControl `json:"trafficControl,omitempty"`

	// Analysis parameters
	// +optional
	Analysis Analysis `json:"analysis,omitempty"`

	// Assessment is a flag to terminate experiment with action
	// +optional.
	//+kubebuilder:validation:Enum=override_success,override_failure
	Assessment AssessmentType `json:"assessment,omitempty"`
}

// TargetService defines what to watch in the controller
type TargetService struct {
	// defines the characteristics of the service
	*corev1.ObjectReference `json:",inline"`

	// Baseline tells the name of baseline
	Baseline string `json:"baseline,omitempty"`

	// Candidate tells the name of candidate
	Candidate string `json:"candidate,omitempty"`
}

type Phase string

const (
	PhaseInitializing Phase = "Initializing"
	PhasePause        Phase = "Pause"
	PhaseProgressing  Phase = "Progressing"
	PhaseSucceeded    Phase = "Succeeded"
	PhaseFailed       Phase = "Failed"
)

// ExperimentStatus defines the observed state of Experiment
type ExperimentStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// StartTimestamp is the timestamp when the experiment starts
	StartTimestamp string `json:"startTimestamp,omitempty"`

	// EndTimestamp is the timestamp when experiment completes
	EndTimestamp string `json:"endTimestamp,omitempty"`

	// LastIncrementTime is the last time the traffic has been incremented
	LastIncrementTime metav1.Time `json:"lastIncrementTime,omitempty"`

	// CurrentIteration is the current iteration number
	CurrentIteration int `json:"currentIteration,omitempty"`

	// AnalysisState is the last analysis state
	AnalysisState runtime.RawExtension `json:"analysisState,omitempty"`

	// GrafanaURL is the url to the Grafana Dashboard
	GrafanaURL string `json:"grafanaURL,omitempty"`

	// AssessmentSummary returned by the last analyis
	AssessmentSummary Summary `json:"assessment,omitempty"`

	// TrafficSplit tells the current traffic spliting between baseline and candidate
	TrafficSplit TrafficSplit `json:"trafficSplitPercentage,omitempty"`

	// Phase marks the Phase the experiment is at
	Phase Phase `json:"phase,omitempty"`

	// Message specifies message to show in the kubectl printer
	Message string `json:"message,omitempty"`
}

type TrafficSplit struct {
	Baseline  int `json:"baseline"`
	Candidate int `json:"candidate"`
}

type TrafficControl struct {
	// Strategy is the strategy used for experiment. Options:
	// "check_and_increment": get decision on traffic increament from analytics
	// "increment_without_check": increase traffic each interval without calling analytics
	// +optional. Default is "check_and_increment".
	//+kubebuilder:validation:Enum=check_and_increment,increment_without_check
	Strategy *string `json:"strategy,omitempty"`

	// MaxTrafficPercentage is the maximum traffic ratio to send to the candidate. Default is 50
	// +optional
	MaxTrafficPercentage *float64 `json:"maxTrafficPercentage,omitempty"`

	// TrafficStepSize is the traffic increment per interval. Default is 2.0
	// +optional
	TrafficStepSize *float64 `json:"trafficStepSize,omitempty"`

	// Interval is the time in second before the next increment. Default is 1mn
	// +optional
	Interval *string `json:"interval,omitempty"`

	// Maximum number of iterations for this experiment. Default to 100.
	// +optional
	MaxIterations *int `json:"maxIterations,omitempty"`

	// Determines how the traffic must be split at the end of the experiment; options:
	// "baseline": all traffic goes to the baseline version;
	// "candidate": all traffic goes to the candidate version;
	// "both": traffic is split across baseline and candidate.
	// Defaults to “candidate”
	// +optional
	//+kubebuilder:validation:Enum=baseline,candidate,both
	OnSuccess *string `json:"onSuccess,omitempty"`
}

type Analysis struct {
	// AnalyticsService endpoint
	AnalyticsService string `json:"analyticsService,omitempty"`

	// Grafana Dashboard endpoint
	GrafanaEndpoint string `json:"grafanaEndpoint,omitempty"`

	// List of criteria for assessing the candidate version
	SuccessCriteria []SuccessCriterion `json:"successCriteria,omitempty"`
}

type Summary struct {
	// Overall summary based on all success criteria
	Conclusions []string `json:"conclusions,omitempty"`

	// Indicates whether or not all success criteria for assessing the candidate version
	// have been met
	AllSuccessCriteriaMet bool `json:"all_success_criteria_met,omitempty"`

	// Indicates whether or not the experiment must be aborted based on the success criteria
	AbortExperiment bool `json:"abort_experiment,omitempty"`
}

type ToleranceType string

const (
	ToleranceTypeDelta     ToleranceType = "delta"
	ToleranceTypeThreshold ToleranceType = "threshold"
)

// SuccessCriterion specifies the criteria for an experiment to succeed
type SuccessCriterion struct {
	// Name of the metric to which the criterion applies. Options:
	// "iter8_latency": mean latency of the service
	// "iter8_error_rate": mean error rate (~5** HTTP Status codes) of the service
	// "iter8_error_count": total error count (~5** HTTP Status codes) of the service
	//+kubebuilder:validation:Enum=iter8_latency,iter8_error_rate,iter8_error_count
	MetricName string `json:"metricName"`

	// 	Tolerance type. Options:
	// "delta": compares the candidate against the baseline version with respect to the metric;
	// "threshold": checks the candidate with respect to the metric
	//+kubebuilder:validation:Enum=threshold,delta
	ToleranceType ToleranceType `json:"toleranceType"`

	// Value to check
	Tolerance float64 `json:"tolerance"`

	// Minimum number of data points required to make a decision based on this criterion;
	// If not specified, the default value is 10
	// +optional
	SampleSize *int `json:"sampleSize,omitempty"`

	// Indicates whether or not the experiment must finish if this criterion is not satisfied;
	// defaults to false
	// +optional
	StopOnFailure *bool `json:"stopOnFailure,omitempty"`
}

// GetStrategy gets the strategy used for traffic control. Default is "check_and_increment".
func (t *TrafficControl) GetStrategy() string {
	strategy := t.Strategy
	if strategy == nil {
		defaultValue := "check_and_increment"
		strategy = &defaultValue
	}
	return *strategy
}

// GetMaxTrafficPercentage gets the specified max traffic percent or the default value (50)
func (t *TrafficControl) GetMaxTrafficPercentage() float64 {
	maxPercent := t.MaxTrafficPercentage
	if maxPercent == nil {
		fifty := float64(50)
		maxPercent = &fifty
	}
	return *maxPercent
}

// GetStepSize gets the specified step size or the default value (2%)
func (t *TrafficControl) GetStepSize() float64 {
	stepSize := t.TrafficStepSize
	if stepSize == nil {
		two := float64(2)
		stepSize = &two
	}
	return *stepSize
}

// GetMaxIterations gets the number of iterations or the default value (100)
func (t *TrafficControl) GetMaxIterations() int {
	count := t.MaxIterations
	if count == nil {
		hundred := int(100)
		count = &hundred
	}
	return *count
}

// GetInterval gets the specified interval or the default value (1m)
func (t *TrafficControl) GetInterval() string {
	interval := t.Interval
	if interval == nil {
		onemn := "1m"
		interval = &onemn
	}
	return *interval
}

// GetIntervalDuration gets the specified interval or the default value (1mn)
func (t *TrafficControl) GetIntervalDuration() (time.Duration, error) {
	interval := t.GetInterval()

	return time.ParseDuration(interval)
}

// GetOnSuccess describes how the traffic must be split at the end of the experiment; Default is "candidate"
func (t *TrafficControl) GetOnSuccess() string {
	onsuccess := t.OnSuccess
	if onsuccess == nil {
		return "candidate"
	}
	return *onsuccess
}

// GetServiceEndpoint returns the analytcis endpoint; Default is "http://iter8analytics:5555".
func (a *Analysis) GetServiceEndpoint() string {
	endpoint := a.AnalyticsService
	if len(endpoint) == 0 {
		endpoint = "http://iter8analytics:5555"
	}

	return endpoint
}

// GetGrafanaEndpoint returns the grafana endpoint; Default is "http://localhost:3000".
func (a *Analysis) GetGrafanaEndpoint() string {
	endpoint := a.GrafanaEndpoint
	if len(endpoint) == 0 {
		endpoint = "http://localhost:3000"
	}

	return endpoint
}

// GetSampleSize returns the sample size for analytics in each iteration; Default is 10.
func (s *SuccessCriterion) GetSampleSize() int {
	size := s.SampleSize
	if size == nil {
		defaultValue := 10
		size = &defaultValue
	}
	return *size
}

// GetStopOnFailure returns the sample size for analytics in each iteration; Default is false.
func (s *SuccessCriterion) GetStopOnFailure() bool {
	out := s.StopOnFailure
	if out == nil {
		defaultValue := false
		out = &defaultValue
	}
	return *out
}

const (
	// ExperimentConditionReady has status True when the Experiment has finished controlling traffic
	ExperimentConditionReady = duckv1alpha1.ConditionReady

	// ExperimentConditionTargetsProvided has status True when the Experiment detects all elements specified in targetService
	ExperimentConditionTargetsProvided duckv1alpha1.ConditionType = "TargetsProvided"

	// ExperimentConditionAnalyticsServiceNormal has status True when the analytics service is operating normally
	ExperimentConditionAnalyticsServiceNormal duckv1alpha1.ConditionType = "AnalyticsServiceNormal"

	// ExperimentConditionMetricsSynced has status True when metrics are successfully synced with config map
	ExperimentConditionMetricsSynced duckv1alpha1.ConditionType = "MetricsSynced"

	// ExperimentConditionExperimentCompleted has status True when the experiment is completed
	ExperimentConditionExperimentCompleted duckv1alpha1.ConditionType = "ExperimentCompleted"

	// ExperimentConditionExperimentSucceeded has status True when the experiment is succeeded
	ExperimentConditionExperimentSucceeded duckv1alpha1.ConditionType = "ExperimentSucceeded"
)

var experimentCondSet = duckv1alpha1.NewLivingConditionSet(
	ExperimentConditionMetricsSynced,
	ExperimentConditionTargetsProvided,
	ExperimentConditionExperimentCompleted,
	ExperimentConditionExperimentSucceeded,
	ExperimentConditionAnalyticsServiceNormal,
)

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *ExperimentStatus) InitializeConditions() {
	experimentCondSet.Manage(s).InitializeConditions()
	if s.Phase == "" {
		s.Phase = PhaseInitializing
	}
}

// MarkMetricsSynced sets the condition that the metrics are synced with config map
func (s *ExperimentStatus) MarkMetricsSynced() {
	experimentCondSet.Manage(s).MarkTrue(ExperimentConditionMetricsSynced)
}

// MarkMetricsSyncedError sets the condition that the error occurs when syncing with the config map
func (s *ExperimentStatus) MarkMetricsSyncedError(reason, messageFormat string, messageA ...interface{}) {
	experimentCondSet.Manage(s).MarkFalse(ExperimentConditionMetricsSynced, reason, messageFormat, messageA...)
	s.Phase = PhasePause
	s.Message = composeMessage(reason, messageFormat, messageA...)
}

// MarkTargetsFound sets the condition that the all target have been found
func (s *ExperimentStatus) MarkTargetsFound() {
	experimentCondSet.Manage(s).MarkTrue(ExperimentConditionTargetsProvided)
}

// MarkTargetsError sets the condition that the target service hasn't been found.
func (s *ExperimentStatus) MarkTargetsError(reason, messageFormat string, messageA ...interface{}) {
	experimentCondSet.Manage(s).MarkFalse(ExperimentConditionTargetsProvided, reason, messageFormat, messageA...)
	s.Phase = PhasePause
	s.Message = composeMessage(reason, messageFormat, messageA...)
}

// MarkAnalyticsServiceRunning sets the condition that the analytics service is operating normally
func (s *ExperimentStatus) MarkAnalyticsServiceRunning() {
	experimentCondSet.Manage(s).MarkTrue(ExperimentConditionAnalyticsServiceNormal)
}

// MarkAnalyticsServiceError sets the condition that the analytics service has breakdown
func (s *ExperimentStatus) MarkAnalyticsServiceError(reason, messageFormat string, messageA ...interface{}) {
	experimentCondSet.Manage(s).MarkFalse(ExperimentConditionTargetsProvided, reason, messageFormat, messageA...)
	s.Message = composeMessage(reason, messageFormat, messageA...)
	s.Phase = PhasePause
}

// MarkExperimentCompleted sets the condition that the experiemnt is completed
func (s *ExperimentStatus) MarkExperimentCompleted() {
	experimentCondSet.Manage(s).MarkTrue(ExperimentConditionExperimentCompleted)
}

// MarkExperimentNotCompleted sets the condition that the experiemnt is ongoing
func (s *ExperimentStatus) MarkExperimentNotCompleted(reason, messageFormat string, messageA ...interface{}) {
	experimentCondSet.Manage(s).MarkFalse(ExperimentConditionExperimentCompleted, reason, messageFormat, messageA...)
	s.Phase = PhaseProgressing
	s.Message = composeMessage(reason, messageFormat, messageA...)
}

// MarkExperimentSucceeded sets the condition that the experiemnt is completed
func (s *ExperimentStatus) MarkExperimentSucceeded(reason, messageFormat string, messageA ...interface{}) {
	experimentCondSet.Manage(s).MarkTrue(ExperimentConditionExperimentSucceeded)
	s.Phase = PhaseSucceeded
	s.Message = composeMessage(reason, messageFormat, messageA...)
}

// MarkExperimentFailed sets the condition that the experiemnt is ongoing
func (s *ExperimentStatus) MarkExperimentFailed(reason, messageFormat string, messageA ...interface{}) {
	experimentCondSet.Manage(s).MarkFalse(ExperimentConditionExperimentSucceeded, reason, messageFormat, messageA...)
	s.Phase = PhaseFailed
	s.Message = composeMessage(reason, messageFormat, messageA...)
}

func composeMessage(reason, messageFormat string, messageA ...interface{}) string {
	out := reason
	if len(fmt.Sprintf(messageFormat, messageA...)) > 0 {
		out += ", " + fmt.Sprintf(messageFormat, messageA...)
	}
	return out
}

func init() {
	SchemeBuilder.Register(&Experiment{}, &ExperimentList{})
}

// ExperimentMetrics is a map from metric name to metric definition
type ExperimentMetrics map[string]ExperimentMetric

// ExperimentMetric stores details of a metric query template to
type ExperimentMetric struct {
	// QueryTemplate is the query template for metric
	QueryTemplate string `json:"query_template"`

	// SampleSizeTemplate is the query template for sample size
	SampleSizeTemplate string `json:"sample_size_template"`

	// Type is the type of this metric
	Type string `json:"type"`
}
