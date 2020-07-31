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

// OnTerminationType provides options for onTermination
type OnTerminationType string

const (
	// OnTerminationToWinner indicates all traffic should go to winner candidate when experiment is terminated
	OnTerminationToWinner OnTerminationType = "to_winner"

	// OnTerminationToBaseline indicates all traffic should go to baseline when experiment is terminated
	OnTerminationToBaseline OnTerminationType = "to_baseline"

	// OnTerminationKeepLast keeps the last traffic status when experiment is terminated
	OnTerminationKeepLast OnTerminationType = "keep_last"
)

// StrategyType provides options for strategy used in experiment
type StrategyType string

const (
	// StrategyProgressive is the progressive strategy
	StrategyProgressive StrategyType = "progressive"

	// StrategyTop2 is the top_2 strategy
	StrategyTop2 StrategyType = "top_2"

	// StrategyUniform is the uniform strategy
	StrategyUniform StrategyType = "uniform"
)

// ActionType provides options for override actions
type ActionType string

const (
	// ActionPause is an action to pause the experiment
	ActionPause ActionType = "pause"

	// ActionResume is an action to resume the experiment
	ActionResume ActionType = "resume"

	// ActionTerminate is an action to terminate the experiment
	ActionTerminate ActionType = "terminate"
)

// ExperimentConditionType limits conditions can be set by controller
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

// PhaseType has options for phases that an experiment can be at
type PhaseType string

const (
	// PhasePause indicates experiment is paused
	PhasePause PhaseType = "Pause"

	// PhaseProgressing indicates experiment is progressing
	PhaseProgressing PhaseType = "Progressing"

	// PhaseCompleted indicates experiment has competed (successfully or not)
	PhaseCompleted PhaseType = "Completed"
)

// A set of reason setting the experiment condition status
const (
	ReasonTargetsFound            = "TargetsFound"
	ReasonTargetsError            = "TargetsError"
	ReasonAnalyticsServiceError   = "AnalyticsServiceError"
	ReasonAnalyticsServiceRunning = "AnalyticsServiceRunning"
	ReasonIterationUpdate         = "IterationUpdate"
	ReasonAssessmentUpdate        = "AssessmentUpdate"
	ReasonTrafficUpdate           = "TrafficUpdate"
	ReasonExperimentCompleted     = "ExperimentCompleted"
	ReasonSyncMetricsError        = "SyncMetricsError"
	ReasonSyncMetricsSucceeded    = "SyncMetricsSucceeded"
	ReasonRoutingRulesError       = "RoutingRulesError"
	ReasonRoutingRulesReady       = "RoutingRulesReady"
	ReasonActionPause             = "ActionPause"
	ReasonActionResume            = "ActionResume"
)
