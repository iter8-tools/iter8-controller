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
	// Action provides user an option to take action in the experiment
	// pause: pause the progress of experiment
	// resume: resume the experiment
	// override_failure: force the experiment to failure status
	// override_success: force the experiment to success status
	// +optional.
	//+kubebuilder:validation:Enum={pause,resume,override_failure,override_success}
	Action ExperimentAction `json:"action,omitempty"`
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
	// TargetService is a reference to an object to use as target service
	TargetService TargetService `json:"targetService"`

	// TrafficControl defines parameters for controlling the traffic
	// +optional
	TrafficControl TrafficControl `json:"trafficControl,omitempty"`

	// Analysis parameters
	// +optional
	Analysis Analysis `json:"analysis,omitempty"`

	// CleanUp is a flag to determine the action to take at the end of experiment
	// +optional.
	//+kubebuilder:validation:Enum=delete
	CleanUp CleanUpType `json:"cleanup,omitempty"`

	// RoutingReference provides references to routing rules set by users
	// +optional
	RoutingReference *corev1.ObjectReference `json:"routingReference,omitempty"`
}

// CleanUpType defines the possible input for cleanup
type CleanUpType string

const (
	// CleanUpDelete indicates unused deployment should be removed on experiment completion
	CleanUpDelete CleanUpType = "delete"
	// CleanUpNull indicates no action should ne taken on experiment completion
	CleanUpNull CleanUpType = ""
)

type Host struct {
	// Name of the Host
	Name string `json:"name"`

	// The gateway
	Gateway string `json:"gateway"`
}

// TargetService defines what to watch in the controller
type TargetService struct {
	// defines the characteristics of the service
	*corev1.ObjectReference `json:",inline"`

	// Baseline tells the name of baseline
	Baseline string `json:"baseline,omitempty"`

	// Candidate tells the name of candidate
	Candidate string `json:"candidate,omitempty"`

	// List of hosts related to this service
	Hosts []Host `json:"hosts,omitempty"`

	// Port number exposed by internal services
	Port *int32 `json:"port,omitempty"`
}

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

// ExperimentStatus defines the observed state of Experiment
type ExperimentStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// CreateTimestamp is the timestamp when the experiment is created
	CreateTimestamp int64 `json:"createTimestamp,omitempty"`

	// StartTimestamp is the timestamp when the experiment starts
	StartTimestamp int64 `json:"startTimestamp,omitempty"`

	// EndTimestamp is the timestamp when experiment completes
	EndTimestamp int64 `json:"endTimestamp,omitempty"`

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

// TrafficSplit specifies percentage of traffic to baseline vs candidate
type TrafficSplit struct {
	Baseline  int `json:"baseline"`
	Candidate int `json:"candidate"`
}

// TrafficControl specifies how/when traffic between versioins should be altered
type TrafficControl struct {
	// Strategy is the strategy used for experiment. Options:
	// "check_and_increment": get decision on traffic increament from analytics
	// "increment_without_check": increase traffic each interval without calling analytics
	// +optional. Default is "check_and_increment".
	//+kubebuilder:validation:Enum={check_and_increment,increment_without_check,epsilon_greedy,posterior_bayesian_routing,optimistic_bayesian_routing}
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
	//+kubebuilder:validation:Enum={baseline,candidate,both}
	OnSuccess *string `json:"onSuccess,omitempty"`

	// The required confidence in the recommeded traffic split. Defaults to 0.95
	// +optional
	Confidence *float64 `json:"confidence,omitempty"`
}

// Analysis specifies the parameters for posting/reading the assessment from analytics server
type Analysis struct {
	// AnalyticsService endpoint
	AnalyticsService string `json:"analyticsService,omitempty"`

	// Grafana Dashboard endpoint
	GrafanaEndpoint string `json:"grafanaEndpoint,omitempty"`

	// List of criteria for assessing the candidate version
	SuccessCriteria []SuccessCriterion `json:"successCriteria,omitempty"`

	// The reward used by analytics to assess candidate
	Reward *Reward `json:"reward,omitempty"`
}

type ToleranceType string

const (
	// ToleranceTypeDelta constant string for tolerances of type "delta"
	ToleranceTypeDelta ToleranceType = "delta"
	// ToleranceTypeThreshold constant string for tolerances of type "threshhold"
	ToleranceTypeThreshold ToleranceType = "threshold"
)

// MinMax captures minimum and maximum values of the metric
type MinMax struct {
	// Min minimum possible value of the metric
	Min float64 `json:"min"`

	//Max maximum possible value of the metric
	Max float64 `json:"max"`
}

// SuccessCriterion specifies the criteria for an experiment to succeed
type SuccessCriterion struct {
	// Name of the metric to which the criterion applies. Options:
	MetricName string `json:"metricName"`

	// 	Tolerance type. Options:
	// "delta": compares the candidate against the baseline version with respect to the metric;
	// "threshold": checks the candidate with respect to the metric
	//+kubebuilder:validation:Enum={threshold,delta}
	ToleranceType ToleranceType `json:"toleranceType"`

	// Value to check
	Tolerance float64 `json:"tolerance"`

	// Minimum number of data points required to make a decision based on this criterion;
	// If not specified, the default value is 10
	// +optional
	SampleSize *int `json:"sampleSize,omitempty"`

	// Minimum and maximum values of the metric
	MinMax *MinMax `json:"min_max,omitempty"`

	// Indicates whether or not the experiment must finish if this criterion is not satisfied;
	// defaults to false
	// +optional
	StopOnFailure *bool `json:"stopOnFailure,omitempty"`
}

// Reward specifies the criteria for an experiment to succeed
type Reward struct {
	// Name of the metric to which the criterion applies. Options:
	MetricName string `json:"metricName"`

	// Minimum and maximum values of the metric
	MinMax *MinMax `json:"min_max,omitempty"`
}

// SuccessCriterionStatus contains assessment for a specific success criteria
type SuccessCriterionStatus struct {
	// Name of the metric to which the criterion applies
	// example: iter8_latency
	MetricName string `json:"metric_name"`

	// Assessment of this success criteria in plain English
	Conclusions []string `json:"conclusions"`

	// Indicates whether or not the success criterion for the corresponding metric has been met
	SuccessCriterionMet bool `json:"success_criterion_met"`

	// Indicates whether or not the experiment must be aborted on the basis of the criterion for this metric
	AbortExperiment bool `json:"abort_experiment"`
}

// Summary contains assessment summary from the analytics service
type Summary struct {
	// Overall summary based on all success criteria
	Conclusions []string `json:"conclusions,omitempty"`

	// Indicates whether or not all success criteria for assessing the candidate version
	// have been met
	AllSuccessCriteriaMet bool `json:"all_success_criteria_met,omitempty"`

	// Indicates whether or not the experiment must be aborted based on the success criteria
	AbortExperiment bool `json:"abort_experiment,omitempty"`

	// The list of status for all success criteria applied
	SuccessCriteriaStatus []SuccessCriterionStatus `json:"success_criteria,omitempty"`
}

// ServiceNamespace gets the namespace for targets
func (e *Experiment) ServiceNamespace() string {
	serviceNamespace := e.Spec.TargetService.Namespace
	if serviceNamespace == "" {
		serviceNamespace = e.Namespace
	}
	return serviceNamespace
}

// Assessment2String prints formatted output of assessment summary
func (s *Summary) Assessment2String() string {
	if len(s.Conclusions) == 0 {
		return "Not Available"
	}

	out := "Conclusions:\n"
	for _, s := range s.Conclusions {
		out += "- " + s + "\n"
	}

	out += "Success Criteria Status:\n"
	for _, ss := range s.SuccessCriteriaStatus {
		out += "- metric name: " + ss.MetricName + "\n"
		out += "   conclusions:\n"
		for _, c := range ss.Conclusions {
			out += "   - " + c + "\n"
		}
	}

	return out
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

// GetConfidence retrieves the desired probability that all the success criteria are met
func (t *TrafficControl) GetConfidence() float64 {
	confidence := t.Confidence
	if confidence == nil {
		return 0.95
	}
	return *confidence
}

// GetServiceEndpoint returns the analytcis endpoint; Default is "http://iter8-analytics.iter8:8080".
func (a *Analysis) GetServiceEndpoint() string {
	endpoint := a.AnalyticsService
	if len(endpoint) == 0 {
		return "http://iter8-analytics.iter8:8080"
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

	// ExperimentConditionRoutingRulesReady has status True when routing rules are ready
	ExperimentConditionRoutingRulesReady duckv1alpha1.ConditionType = "RoutingRulesReady"
)

var experimentCondSet = duckv1alpha1.NewLivingConditionSet(
	ExperimentConditionMetricsSynced,
	ExperimentConditionTargetsProvided,
	ExperimentConditionExperimentCompleted,
	ExperimentConditionExperimentSucceeded,
	ExperimentConditionAnalyticsServiceNormal,
	ExperimentConditionRoutingRulesReady,
)

// A set of reason setting the experiment condition status
const (
	ReasonTargetsNotFound         = "TargetsNotFound"
	ReasonTargetsFound            = "TargetsFound"
	ReasonAnalyticsServiceError   = "AnalyticsServiceError"
	ReasonAnalyticsServiceRunning = "AnalyticsServiceRunning"
	ReasonIterationUpdate         = "IterationUpdate"
	ReasonIterationSucceeded      = "IterationSucceeded"
	ReasonIterationFailed         = "IterationFailed"
	ReasonExperimentSucceeded     = "ExperimentSucceeded"
	ReasonExperimentFailed        = "ExperimentFailed"
	ReasonSyncMetricsError        = "SyncMetricsError"
	ReasonSyncMetricsSucceeded    = "SyncMetricsSucceeded"
	ReasonRoutingRulesError       = "RoutingRulesError"
	ReasonRoutingRulesReady       = "RoutingRulesReady"
	ReasonActionPause             = "Pause"
	ReasonActionResume            = "Resume"
)

// Init initialize status values
func (s *ExperimentStatus) Init() {
	// sets relevant unset conditions to Unknown state.
	experimentCondSet.Manage(s).InitializeConditions()

	s.CreateTimestamp = metav1.Now().UTC().UnixNano()

	// TODO: not sure why this is needed
	if s.LastIncrementTime.IsZero() {
		s.LastIncrementTime = metav1.NewTime(time.Unix(0, 0))
	}

	if s.AnalysisState.Raw == nil {
		s.AnalysisState.Raw = []byte("{}")
	}

	s.Phase = PhaseProgressing
}

// MarkMetricsSynced sets the condition that the metrics are synced with config map
// Return true if it's converted from false or unknown
func (s *ExperimentStatus) MarkMetricsSynced() bool {
	prevStat := s.GetCondition(ExperimentConditionMetricsSynced).Status
	experimentCondSet.Manage(s).MarkTrue(ExperimentConditionMetricsSynced)
	return prevStat != corev1.ConditionTrue
}

// MarkMetricsSyncedError sets the condition that the error occurs when syncing with the config map
// Return true if it's converted from true or unknown
func (s *ExperimentStatus) MarkMetricsSyncedError(reason, messageFormat string, messageA ...interface{}) bool {
	prevStat := s.GetCondition(ExperimentConditionMetricsSynced).Status
	experimentCondSet.Manage(s).MarkFalse(ExperimentConditionMetricsSynced, reason, messageFormat, messageA...)
	s.Phase = PhasePause
	s.Message = composeMessage(reason, messageFormat, messageA...)
	return prevStat != corev1.ConditionFalse
}

func (s *ExperimentStatus) TargetsFound() bool {
	return s.GetCondition(ExperimentConditionTargetsProvided).Status == corev1.ConditionTrue
}

// MarkTargetsFound sets the condition that the all target have been found
// Return true if it's converted from false or unknown
func (s *ExperimentStatus) MarkTargetsFound() bool {
	prevStat := s.GetCondition(ExperimentConditionTargetsProvided).Status
	experimentCondSet.Manage(s).MarkTrue(ExperimentConditionTargetsProvided)
	return prevStat != corev1.ConditionTrue
}

// MarkTargetsError sets the condition that the target service hasn't been found.
// Return true if it's converted from true or unknown
func (s *ExperimentStatus) MarkTargetsError(reason, messageFormat string, messageA ...interface{}) bool {
	prevStat := s.GetCondition(ExperimentConditionTargetsProvided).Status
	prevMsg := s.GetCondition(ExperimentConditionTargetsProvided).Message
	experimentCondSet.Manage(s).MarkFalse(ExperimentConditionTargetsProvided, reason, messageFormat, messageA...)
	s.Phase = PhasePause
	s.Message = composeMessage(reason, messageFormat, messageA...)
	return prevStat != corev1.ConditionFalse || prevMsg != s.GetCondition(ExperimentConditionTargetsProvided).Message
}

// MarkRoutingRulesReady sets the condition that the routing rules are ready
// Return true if it's converted from false or unknown
func (s *ExperimentStatus) MarkRoutingRulesReady() bool {
	prevStat := s.GetCondition(ExperimentConditionRoutingRulesReady).Status
	experimentCondSet.Manage(s).MarkTrue(ExperimentConditionRoutingRulesReady)
	return prevStat != corev1.ConditionTrue
}

// MarkRoutingRulesError sets the condition that the routing rules are not ready
// Return true if it's converted from true or unknown
func (s *ExperimentStatus) MarkRoutingRulesError(reason, messageFormat string, messageA ...interface{}) bool {
	prevStat := s.GetCondition(ExperimentConditionRoutingRulesReady).Status
	experimentCondSet.Manage(s).MarkFalse(ExperimentConditionRoutingRulesReady, reason, messageFormat, messageA...)
	s.Phase = PhasePause
	s.Message = composeMessage(reason, messageFormat, messageA...)
	return prevStat != corev1.ConditionFalse
}

// MarkAnalyticsServiceRunning sets the condition that the analytics service is operating normally
// Return true if it's converted from false or unknown
func (s *ExperimentStatus) MarkAnalyticsServiceRunning() bool {
	prevStat := s.GetCondition(ExperimentConditionAnalyticsServiceNormal).Status
	experimentCondSet.Manage(s).MarkTrue(ExperimentConditionAnalyticsServiceNormal)
	return prevStat != corev1.ConditionTrue
}

// MarkAnalyticsServiceError sets the condition that the analytics service breaks down
// Return true if it's converted from true or unknown
func (s *ExperimentStatus) MarkAnalyticsServiceError(reason, messageFormat string, messageA ...interface{}) bool {
	prevStat := s.GetCondition(ExperimentConditionAnalyticsServiceNormal).Status
	experimentCondSet.Manage(s).MarkFalse(ExperimentConditionAnalyticsServiceNormal, reason, messageFormat, messageA...)
	s.Message = composeMessage(reason, messageFormat, messageA...)
	s.Phase = PhasePause
	return prevStat != corev1.ConditionFalse
}

// MarkExperimentCompleted sets the condition that the experiemnt is completed
func (s *ExperimentStatus) MarkExperimentCompleted() {
	experimentCondSet.Manage(s).MarkTrue(ExperimentConditionExperimentCompleted)
	s.Phase = PhaseCompleted
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
	s.Phase = PhaseCompleted
	s.Message = composeMessage(reason, messageFormat, messageA...)
}

// MarkExperimentFailed sets the condition that the experiemnt is ongoing
func (s *ExperimentStatus) MarkExperimentFailed(reason, messageFormat string, messageA ...interface{}) {
	experimentCondSet.Manage(s).MarkFalse(ExperimentConditionExperimentSucceeded, reason, messageFormat, messageA...)
	s.Phase = PhaseCompleted
	s.Message = composeMessage(reason, messageFormat, messageA...)
}

// MarkActionPause sets the phase and status that experiment is paused by action
// returns true if this is a newly-set operation
func (s *ExperimentStatus) MarkActionPause() bool {
	prevPhase, prevMsg := s.Phase, s.Message
	s.Phase = PhasePause
	s.Message = ReasonActionPause
	return prevPhase != s.Phase || s.Message != prevMsg
}

// MarkActionResume sets the phase and status that experiment is resmued by action
// returns true if this is a newly-set operation
func (s *ExperimentStatus) MarkActionResume() bool {
	prevPhase, prevMsg := s.Phase, s.Message
	s.Phase = PhaseProgressing
	s.Message = ReasonActionResume
	return prevPhase != s.Phase || s.Message != prevMsg
}

func composeMessage(reason, messageFormat string, messageA ...interface{}) string {
	out := reason
	if len(fmt.Sprintf(messageFormat, messageA...)) > 0 {
		out += ": " + fmt.Sprintf(messageFormat, messageA...)
	}
	return out
}

// ExperimentMetrics is a map from metric name to metric definition
type ExperimentMetrics map[string]ExperimentMetric

// ExperimentMetric stores details of a metric query template to
type ExperimentMetric struct {
	// QueryTemplate is the query template for metric
	QueryTemplate string `json:"query_template"`

	// SampleSizeTemplate is the query template for sample size
	SampleSizeTemplate string `json:"sample_size_template"`

	// IsCounter indicates metric is a monotonically increasing counter
	IsCounter bool `json:"is_counter"`

	// AbsentValue  is default value when data source does not provide a value
	AbsentValue string `json:"absent_value"`
}

// ExperimentAction defines the external action that can be performed to the experiment
type ExperimentAction string

const (
	// ActionOverrideSuccess indicates that the experiment should be forced to a successful termination
	ActionOverrideSuccess ExperimentAction = "override_success"
	// ActionOverrideFailure indicates that the experiment should be forced to a failed termination
	ActionOverrideFailure ExperimentAction = "override_failure"
	// ActionPause indicates a request for pausing experiment
	ActionPause ExperimentAction = "pause"
	// ActionResume indicates a request for resuming experiment
	ActionResume ExperimentAction = "resume"
)

func (a ExperimentAction) TerminateExperiment() bool {
	return a == ActionOverrideFailure || a == ActionOverrideSuccess
}

// GetStrategy returns the actual strategy of the experiment
func (e *Experiment) GetStrategy() string {
	strategy := e.Spec.TrafficControl.GetStrategy()
	if strategy != "increment_without_check" &&
		(e.Spec.Analysis.SuccessCriteria == nil || len(e.Spec.Analysis.SuccessCriteria) == 0) {
		strategy = "increment_without_check"
	}
	return strategy
}

// Succeeded determines whether experiment is a success or not
func (e *Experiment) Succeeded() bool {
	if e.Action.TerminateExperiment() {
		return e.Action == ActionOverrideSuccess
	}

	if e.GetStrategy() == "increment_without_check" {
		return true
	} else {
		return e.Status.AssessmentSummary.AllSuccessCriteriaMet
	}
}

func init() {
	SchemeBuilder.Register(&Experiment{}, &ExperimentList{})
}
