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
	"time"
)

const (
	// DefaultRewardMetric indicate whether a metric is a reward by default, which is false
	DefaultRewardMetric bool = false

	// DefaultZeroToOne indicate whether the value range of metric is from 0 to 1  by default, which is false
	DefaultZeroToOne bool = false

	// DefaultCleanup indicate whether router and targets receiving no traffic should be deleted after expreriment
	DefaultCleanup bool = false

	// DefaultStrategy is the default value for strategy, which is progressive
	DefaultStrategy StrategyType = StrategyProgressive

	// DefaultOnTermination is the default value for onTermination, which is to_winner
	DefaultOnTermination OnTerminationType = OnTerminationToWinner

	// DefaultPercentage is the default traffic percentage used in experiment, which is 100
	DefaultPercentage int32 = 100

	// DefaultMaxIncrement is the default maxIncrement for traffic update, which is 2
	DefaultMaxIncrement int32 = 2

	// DefaultDuration is the default duration for an interval, which is 30 seconds
	DefaultDuration time.Duration = time.Second * 30

	// DefaultMaxIterations is the default number of iterations, which is 100
	DefaultMaxIterations int32 = 100

	// DefaultAnalyticsEndpoint is the default endpoint of analytics
	DefaultAnalyticsEndpoint string = "http://iter8-analytics.iter8:8080"
)

// ServiceNamespace gets the namespace for targets
func (e *Experiment) ServiceNamespace() string {
	serviceNamespace := e.Spec.Service.Namespace
	if serviceNamespace == "" {
		serviceNamespace = e.Namespace
	}
	return serviceNamespace
}

// Pause indicates whether an Experiment Pause request is issued or not
func (s *ExperimentSpec) Pause() bool {
	if s.ManualOverride != nil && s.ManualOverride.Action == ActionPause {
		return true
	}
	return false
}

// Resume indicates whether an Experiment Resume request is issued or not
func (s *ExperimentSpec) Resume() bool {
	if s.ManualOverride != nil && s.ManualOverride.Action == ActionResume {
		return true
	}
	return false
}

// Terminate indicates whether an Experiment Terminate request is issued or not
func (s *ExperimentSpec) Terminate() bool {
	if s.ManualOverride != nil && s.ManualOverride.Action == ActionTerminate {
		return true
	}
	return false
}

// GetAction retrieves the action specified in manual override if any
func (s *ExperimentSpec) GetAction() ActionType {
	if s.ManualOverride != nil {
		return s.ManualOverride.Action
	}
	return ActionType("")
}

// GetInterval returns specified(or default) interval for each duration
func (s *ExperimentSpec) GetInterval() (time.Duration, error) {
	if s.Duration == nil || s.Duration.Interval == nil {
		return DefaultDuration, nil
	}
	return time.ParseDuration(*s.Duration.Interval)
}

// GetMaxIterations returns specified(or default) max of iterations
func (s *ExperimentSpec) GetMaxIterations() int32 {
	if s.Duration == nil || s.Duration.MaxIterations == nil {
		return DefaultMaxIterations
	}
	return *s.Duration.MaxIterations
}

// HasRewardMetric indicates whether this criterion uses a reward metric or not
func (c *Criterion) HasRewardMetric() bool {
	if c.IsReward == nil {
		return DefaultRewardMetric
	}
	return *c.IsReward
}

// CutOffOnViolation indicates whether traffic should be cutoff to a target if threshold is violated
func (t *Threshold) CutOffOnViolation() bool {
	if t.CutoffTrafficOnViolation == nil {
		return false
	}
	return *t.CutoffTrafficOnViolation
}

// GetStrategy gets the specified(or default) strategy used for traffic control
func (s *ExperimentSpec) GetStrategy() string {
	if s.TrafficControl == nil || s.TrafficControl.Strategy == nil {
		return string(DefaultStrategy)
	}
	return string(*s.TrafficControl.Strategy)
}

// GetOnTermination returns specified(or default) onTermination strategy for traffic controller
func (s *ExperimentSpec) GetOnTermination() OnTerminationType {
	if s.TrafficControl == nil || s.TrafficControl.OnTermination == nil {
		return DefaultOnTermination
	}
	return *s.TrafficControl.OnTermination
}

// GetPercentage returns specified(or default) experiment traffic percentage
func (s *ExperimentSpec) GetPercentage() int32 {
	if s.TrafficControl == nil || s.TrafficControl.Percentage == nil {
		return DefaultPercentage
	}
	return *s.TrafficControl.Percentage
}

// GetMaxIncrements returns specified(or default) maxIncrements for each traffic update
func (s *ExperimentSpec) GetMaxIncrements() int32 {
	if s.TrafficControl == nil || s.TrafficControl.MaxIncrement == nil {
		return DefaultMaxIncrement
	}
	return *s.TrafficControl.MaxIncrement
}

// GetAnalyticsEndpoint returns specified(or default) analytics endpoint
func (s *ExperimentSpec) GetAnalyticsEndpoint() string {
	if s.AnalyticsEndpoint == nil {
		return DefaultAnalyticsEndpoint
	}
	return *s.AnalyticsEndpoint
}

// GetCleanup returns whether router and targets receiving no traffic should be deleted after expreriment
func (s *ExperimentSpec) GetCleanup() bool {
	if s.Cleanup == nil {
		return DefaultCleanup
	}
	return *s.Cleanup
}

// IsZeroToOne returns specified(or default) zeroToOne value
func (r *RatioMetric) IsZeroToOne() bool {
	if r.ZeroToOne == nil {
		return DefaultZeroToOne
	}
	return *r.ZeroToOne
}

// TerminateExperiment terminates experiment
func (s *ExperimentSpec) TerminateExperiment() {
	s.ManualOverride = &ManualOverride{
		Action: ActionTerminate,
	}
}

// Validate checks whether specification in Service can be supported by iter8 or not
// returns nil if ok; otherwise non-nil err with detailed explanation will be returned
func (s *Service) Validate() error {
	// check service/hosts specification
	if s.Name == "" && len(s.Hosts) == 0 {
		return fmt.Errorf("Either Name or Hosts should be specified in Service")
	}

	// check kind/apiVersion specification
	switch s.Kind {
	case "Deployment", "":
		if !(s.APIVersion == "" || s.APIVersion == "apps/v1" || s.APIVersion == "v1") {
			return fmt.Errorf("Invalid kind/apiVerison pair: %s, %s", s.Kind, s.APIVersion)
		}
	case "Service":
		if !(s.APIVersion == "" || s.APIVersion == "v1") {
			return fmt.Errorf("Invalid kind/apiVerison pair: %s, %s", s.Kind, s.APIVersion)
		}
	default:
		return fmt.Errorf("Invalid kind/apiVerison pair: %s, %s", s.Kind, s.APIVersion)
	}

	return nil
}
