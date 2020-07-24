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
	"fmt"

	"github.com/iter8-tools/iter8-controller/pkg/analytics/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var experimentCondSet = []ExperimentConditionType{
	ExperimentConditionMetricsSynced,
	ExperimentConditionTargetsProvided,
	ExperimentConditionExperimentCompleted,
	ExperimentConditionAnalyticsServiceNormal,
	ExperimentConditionRoutingRulesReady,
}

func (s *ExperimentStatus) addCondition(conditionType ExperimentConditionType) *ExperimentCondition {
	condition := &ExperimentCondition{
		Type:   conditionType,
		Status: corev1.ConditionUnknown,
	}
	now := metav1.Now()
	condition.LastTransitionTime = &now
	s.Conditions = append(s.Conditions, condition)
	return condition
}

// GetCondition returns condition of given conditionType
func (s *ExperimentStatus) GetCondition(condition ExperimentConditionType) *ExperimentCondition {
	for _, c := range s.Conditions {
		if c.Type == condition {
			return c
		}
	}

	return s.addCondition(condition)
}

// IsTrue tells whether the experiment condition is true or not
func (c *ExperimentCondition) IsTrue() bool {
	return c.Status == corev1.ConditionTrue
}

// IsFalse tells whether the experiment condition is false or not
func (c *ExperimentCondition) IsFalse() bool {
	return c.Status == corev1.ConditionFalse
}

// InitStatus initialize status value of an experiment
func (e *Experiment) InitStatus() {
	e.Status.Assessment = &Assessment{
		Baseline: VersionAssessment{
			Name:   e.Spec.Baseline,
			Weight: int32(0),
			VersionAssessment: v1alpha2.VersionAssessment{
				CriterionAssessments: make([]v1alpha2.CriterionAssessment, 0),
			},
		},
		Candidates: make([]VersionAssessment, len(e.Spec.Candidates)),
	}
	for i, name := range e.Spec.Candidates {
		e.Status.Assessment.Candidates[i] = VersionAssessment{
			Name:   name,
			Weight: int32(0),
			VersionAssessment: v1alpha2.VersionAssessment{
				CriterionAssessments: make([]v1alpha2.CriterionAssessment, 0),
			},
		}
	}

	// sets relevant unset conditions to Unknown state.
	for _, c := range experimentCondSet {
		e.Status.addCondition(c)
	}

	currentTime := metav1.Now()
	e.Status.InitTimestamp = &currentTime //metav1.Now()

	if e.Status.AnalysisState == nil {
		e.Status.AnalysisState = &runtime.RawExtension{
			Raw: []byte("{}"),
		}
	}

	if e.Status.AnalysisState.Raw == nil {
		e.Status.AnalysisState.Raw = []byte("{}")
	}

	e.Status.Phase = PhaseProgressing
	currentIteration := int32(0)
	e.Status.CurrentIteration = &currentIteration
	e.Status.ExperimentType = e.Spec.experimentType()
	e.Status.EffectiveHosts = e.Spec.effectiveHosts()
}

const (
	ExperimentTypePerformance string = "Perfromance"
	ExperimentTypeCanary      string = "Canary"
	ExperimentTypeAB          string = "A/B"
	ExperimentTypeABN         string = "A/B/N"
)

func (spec *ExperimentSpec) experimentType() string {
	numCandidates := len(spec.Service.Candidates)
	if 0 == numCandidates {
		return ExperimentTypePerformance
	} else if 1 == numCandidates && spec.hasReward() {
		return ExperimentTypeAB
	} else if 1 == numCandidates {
		return ExperimentTypeCanary
	} else {
		return ExperimentTypeABN
	}
}

func (spec *ExperimentSpec) hasReward() bool {
	for _, criteria := range spec.Criteria {
		if criteria.HasRewardMetric() {
			return true
		}
	}
	return false
}

func (spec *ExperimentSpec) effectiveHosts() []string {
	hosts := make([]string, 0)
	if nil != spec.Service.ObjectReference {
		host := spec.Service.Name
		if host != "" {
			hosts = append(hosts, host)
		}
	}
	for _, host := range spec.Service.Hosts {
		hosts = append(hosts, host.Name)
	}
	return hosts
}

func (c *ExperimentCondition) markCondition(status corev1.ConditionStatus, reason, messageFormat string, messageA ...interface{}) bool {
	message := fmt.Sprintf(messageFormat, messageA...)
	updated := status != c.Status || reason != *c.Reason || message != *c.Message
	c.Status = status
	c.Reason = &reason
	c.Message = &message
	now := metav1.Now()
	c.LastTransitionTime = &now
	return updated
}

// MetricsSynced returns whether status of ExperimentConditionMetricsSynced is true or not
func (s *ExperimentStatus) MetricsSynced() bool {
	return s.GetCondition(ExperimentConditionMetricsSynced).Status == corev1.ConditionTrue
}

// MarkMetricsSynced sets the condition that the metrics are synced with config map
// Return true if it's converted from false or unknown
func (s *ExperimentStatus) MarkMetricsSynced(messageFormat string, messageA ...interface{}) (bool, string) {
	reason := ReasonSyncMetricsSucceeded
	return s.GetCondition(ExperimentConditionMetricsSynced).
		markCondition(corev1.ConditionTrue, reason, messageFormat, messageA...), reason
}

// MarkMetricsSyncedError sets the condition that the error occurs when syncing with the config map
// Return true if it's converted from true or unknown
func (s *ExperimentStatus) MarkMetricsSyncedError(messageFormat string, messageA ...interface{}) (bool, string) {
	reason := ReasonSyncMetricsError
	s.Phase = PhasePause
	*s.Message = composeMessage(reason, messageFormat, messageA...)
	return s.GetCondition(ExperimentConditionMetricsSynced).
		markCondition(corev1.ConditionFalse, reason, messageFormat, messageA...), reason
}

// TargetsFound returns whether status of ExperimentConditionTargetsProvided is true or not
func (s *ExperimentStatus) TargetsFound() bool {
	return s.GetCondition(ExperimentConditionTargetsProvided).Status == corev1.ConditionTrue
}

// MarkTargetsFound sets the condition that the all target have been found
// Return true if it's converted from false or unknown
func (s *ExperimentStatus) MarkTargetsFound(messageFormat string, messageA ...interface{}) (bool, string) {
	reason := ReasonTargetsFound
	return s.GetCondition(ExperimentConditionTargetsProvided).
		markCondition(corev1.ConditionTrue, reason, messageFormat, messageA...), reason
}

// MarkTargetsError sets the condition that there is error in finding all targets
// Return true if it's converted from true or unknown
func (s *ExperimentStatus) MarkTargetsError(messageFormat string, messageA ...interface{}) (bool, string) {
	reason := ReasonTargetsError
	s.Phase = PhasePause
	*s.Message = composeMessage(reason, messageFormat, messageA...)
	return s.GetCondition(ExperimentConditionTargetsProvided).
		markCondition(corev1.ConditionFalse, reason, messageFormat, messageA...), reason
}

// MarkRoutingRulesReady sets the condition that the routing rules are ready
// Return true if it's converted from false or unknown
func (s *ExperimentStatus) MarkRoutingRulesReady(messageFormat string, messageA ...interface{}) (bool, string) {
	reason := ReasonRoutingRulesReady
	message := composeMessage(reason, messageFormat, messageA...)
	s.Message = &message
	return s.GetCondition(ExperimentConditionRoutingRulesReady).
		markCondition(corev1.ConditionTrue, reason, messageFormat, messageA...), reason
}

// MarkRoutingRulesError sets the condition that the routing rules are not ready
// Return true if it's converted from true or unknown
func (s *ExperimentStatus) MarkRoutingRulesError(messageFormat string, messageA ...interface{}) (bool, string) {
	reason := ReasonRoutingRulesError
	message := composeMessage(reason, messageFormat, messageA...)
	s.Phase = PhasePause
	s.Message = &message
	return s.GetCondition(ExperimentConditionRoutingRulesReady).
		markCondition(corev1.ConditionFalse, reason, messageFormat, messageA...), reason
}

// MarkAnalyticsServiceRunning sets the condition that the analytics service is operating normally
// Return true if it's converted from false or unknown
func (s *ExperimentStatus) MarkAnalyticsServiceRunning(messageFormat string, messageA ...interface{}) (bool, string) {
	reason := ReasonAnalyticsServiceRunning
	return s.GetCondition(ExperimentConditionAnalyticsServiceNormal).
		markCondition(corev1.ConditionTrue, reason, messageFormat, messageA...), reason
}

// MarkAnalyticsServiceError sets the condition that the analytics service breaks down
// Return true if it's converted from true or unknown
func (s *ExperimentStatus) MarkAnalyticsServiceError(messageFormat string, messageA ...interface{}) (bool, string) {
	reason := ReasonAnalyticsServiceError
	message := composeMessage(reason, messageFormat, messageA...)
	s.Message = &message
	s.Phase = PhasePause
	return s.GetCondition(ExperimentConditionAnalyticsServiceNormal).
		markCondition(corev1.ConditionFalse, reason, messageFormat, messageA...), reason
}

// ExperimentCompleted returns whether experiment is completed or not
func (s *ExperimentStatus) ExperimentCompleted() bool {
	return s.GetCondition(ExperimentConditionExperimentCompleted).Status == corev1.ConditionTrue
}

// MarkExperimentCompleted sets the condition that the experiemnt is completed
func (s *ExperimentStatus) MarkExperimentCompleted(messageFormat string, messageA ...interface{}) (bool, string) {
	reason := ReasonExperimentCompleted
	message := composeMessage(reason, messageFormat, messageA...)
	s.Phase = PhaseCompleted
	s.Message = &message
	return s.GetCondition(ExperimentConditionExperimentCompleted).
		markCondition(corev1.ConditionTrue, reason, messageFormat, messageA...), reason
}

// MarkIterationUpdate sets the condition that the iteration updated
func (s *ExperimentStatus) MarkIterationUpdate(messageFormat string, messageA ...interface{}) (bool, string) {
	reason := ReasonIterationUpdate
	message := composeMessage(reason, messageFormat, messageA...)
	s.Phase = PhaseProgressing
	s.Message = &message
	s.GetCondition(ExperimentConditionExperimentCompleted).
		markCondition(corev1.ConditionFalse, reason, messageFormat, messageA...)
	return true, reason
}

// MarkTrafficUpdate sets the condition that traffic for target service updated
func (s *ExperimentStatus) MarkTrafficUpdate(messageFormat string, messageA ...interface{}) (bool, string) {
	reason := ReasonTrafficUpdate
	message := composeMessage(reason, messageFormat, messageA...)
	s.Phase = PhaseProgressing
	s.Message = &message
	return s.GetCondition(ExperimentConditionExperimentCompleted).
		markCondition(corev1.ConditionFalse, reason, messageFormat, messageA...), reason
}

// MarkExperimentPause sets the phase and status that experiment is paused by manualOverrides
// returns true if this is a newly-set operation
func (s *ExperimentStatus) MarkExperimentPause(messageFormat string, messageA ...interface{}) (bool, string) {
	reason := ReasonActionPause
	message := composeMessage(reason, messageFormat, messageA...)
	s.Phase = PhasePause
	s.Message = &message
	s.GetCondition(ExperimentConditionExperimentCompleted).
		markCondition(corev1.ConditionFalse, reason, messageFormat, messageA...)
	return true, reason
}

// MarkExperimentResume sets the phase and status that experiment is resmued by manualOverrides
// returns true if this is a newly-set operation
func (s *ExperimentStatus) MarkExperimentResume(messageFormat string, messageA ...interface{}) (bool, string) {
	reason := ReasonActionResume
	message := composeMessage(reason, messageFormat, messageA...)
	s.Phase = PhaseProgressing
	s.Message = &message
	s.GetCondition(ExperimentConditionExperimentCompleted).
		markCondition(corev1.ConditionFalse, reason, messageFormat, messageA...)
	return true, reason
}

func composeMessage(reason, messageFormat string, messageA ...interface{}) string {
	out := reason
	msg := fmt.Sprintf(messageFormat, messageA...)
	if len(msg) > 0 {
		out += ": " + msg
	}
	return out
}
