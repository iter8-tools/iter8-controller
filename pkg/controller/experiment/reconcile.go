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

package experiment

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"

	"github.com/iter8-tools/iter8/pkg/analytics"
	iter8v1alpha2 "github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
	"github.com/iter8-tools/iter8/pkg/controller/experiment/targets"
	"github.com/iter8-tools/iter8/pkg/controller/experiment/util"
)

func (r *ReconcileExperiment) completeExperiment(context context.Context, instance *iter8v1alpha2.Experiment) error {
	// remove experiment and targets from adapter
	r.iter8Adapter.RemoveExperiment(instance)

	overrideAssessment(instance)
	targets.Cleanup(context, instance, r.Client)
	err := r.router.UpdateRouteToStable(instance)
	if err != nil {
		return err
	}

	r.markExperimentCompleted(context, instance, "%s", completeStatusMessage(instance))
	return nil
}

// returns hard-coded termination message
func completeStatusMessage(instance *iter8v1alpha2.Experiment) string {
	out := ""

	if instance.Status.RoutingRulesReady() {
		switch instance.Spec.GetOnTermination() {
		case iter8v1alpha2.OnTerminationToWinner:
			if instance.Status.IsWinnerFound() {
				out += "Traffic To Winner"
				break
			}
			fallthrough
		case iter8v1alpha2.OnTerminationToBaseline:
			out += "Traffic To Baseline"
		case iter8v1alpha2.OnTerminationKeepLast:
			out += "Keep Last Traffic"
		}
	}

	if instance.Spec.Terminate() {
		out += "(Abort)"
	}

	return out
}

func (r *ReconcileExperiment) checkOrInitRules(context context.Context, instance *iter8v1alpha2.Experiment) error {
	err := r.router.Fetch(instance)
	if err != nil {
		r.markRoutingRulesError(context, instance, "Error in getting routing rules: %s, Experiment Ended.", err.Error())
		r.completeExperiment(context, instance)
	}

	return err
}

// return true if instance status should be updated
// returns non-nil error if current reconcile request should be terminated right after this function
func (r *ReconcileExperiment) detectTargets(context context.Context, instance *iter8v1alpha2.Experiment) (bool, error) {
	targetsHandler := targets.Init(instance, r.Client)

	if err := targetsHandler.GetService(context); err != nil {
		if instance.Status.TargetsFound() {
			r.markTargetsError(context, instance, "Service Deleted")
			return false, err
		} else {
			r.markTargetsError(context, instance, "Missing Service")
			return false, err
		}
	}

	if err := targetsHandler.GetBaseline(context); err != nil {
		if instance.Status.TargetsFound() {
			r.markTargetsError(context, instance, "Baseline Deleted")
			return false, err
		} else {
			r.markTargetsError(context, instance, "Missing Baseline")
			return false, err
		}
	} else {
		// UpdateBaseline will create DestinationRule and VirtualService if needed
		if err = r.router.UpdateRouteWithBaseline(instance, targetsHandler.Baseline); err != nil {
			r.markRoutingRulesError(context, instance, "Fail in updating routing rule: %v", err)
			return false, err
		}
	}

	if err := targetsHandler.GetCandidates(context); err != nil {
		if instance.Status.TargetsFound() {
			r.markTargetsError(context, instance, "Candidate Deleted")
			return false, err
		} else {
			r.markTargetsError(context, instance, "Missing Candidate")
			return false, err
		}
	} else {
		// Update DestinationRule for candidates
		// If baseline is also configured (see above), we move set rule to progressing
		if err = r.router.UpdateRouteWithCandidates(instance, targetsHandler.Candidates); err != nil {
			r.markRoutingRulesError(context, instance, "Fail in updating routing rule: %v", err)
			return false, err
		}
	}

	r.markTargetsFound(context, instance, "")
	r.markRoutingRulesReady(context, instance, "")
	return true, nil
}

// returns non-nil error if reconcile process should be terminated right after this function
func (r *ReconcileExperiment) processIteration(context context.Context, instance *iter8v1alpha2.Experiment) error {
	log := util.Logger(context)
	trafficUpdated := false
	// mark experiment begin
	if instance.Status.StartTimestamp == nil {
		startTime := metav1.Now()
		instance.Status.StartTimestamp = &startTime
		r.grafanaConfig.UpdateGrafanaURL(instance)
		r.markStatusUpdate()
	}

	if len(instance.Spec.Criteria) == 0 {
		// each candidate gets maxincrement traffic at each interval
		// until no more traffic can be deducted from baseline
		basetraffic := instance.Status.Assessment.Baseline.Weight
		diff := instance.Spec.GetMaxIncrements() * int32(len(instance.Spec.Candidates))
		if basetraffic-diff >= 0 {
			instance.Status.Assessment.Baseline.Weight = basetraffic - diff
			for i := range instance.Status.Assessment.Candidates {
				instance.Status.Assessment.Candidates[i].Weight += instance.Spec.GetMaxIncrements()
			}
			trafficUpdated = true
		}
	} else {
		// Get latest analysis
		payload, err := analytics.MakeRequest(instance)
		if err != nil {
			r.markAnalyticsServiceError(context, instance, "%s", err.Error())
			return err
		}

		response, err := analytics.Invoke(log, instance.Spec.GetAnalyticsEndpoint(), payload)
		if err != nil {
			r.markAnalyticsServiceError(context, instance, "%s", err.Error())
			return err
		}

		if response.LastState == nil {
			instance.Status.AnalysisState.Raw = []byte("{}")
		} else {
			lastState, err := json.Marshal(response.LastState)
			if err != nil {
				r.markAnalyticsServiceError(context, instance, "%s", err.Error())
				return err
			}
			instance.Status.AnalysisState = &runtime.RawExtension{Raw: lastState}
		}

		abort := true
		instance.Status.Assessment.Baseline.VersionAssessment = response.BaselineAssessment
		for i, ca := range response.CandidateAssessments {
			instance.Status.Assessment.Candidates[i].VersionAssessment = ca.VersionAssessment
			instance.Status.Assessment.Candidates[i].Rollback = ca.Rollback
			if !ca.Rollback {
				abort = false
			}
		}

		if abort {
			instance.Spec.TerminateExperiment()
			log.Info("AbortExperiment", "All candidates fail assessment", "")
			return nil
		}

		// set winner assessment
		instance.Status.Assessment.Winner = &iter8v1alpha2.WinnerAssessment{
			WinnerAssessment: &response.WinnerAssessment,
		}
		if response.WinnerAssessment.WinnerFound {
			for _, candidate := range instance.Status.Assessment.Candidates {
				if candidate.VersionAssessment.ID == response.WinnerAssessment.Winner {
					instance.Status.Assessment.Winner.Name = &candidate.Name
					break
				}
			}
		}
		r.markAssessmentUpdate(context, instance, "Winner assessment: %s", instance.Status.WinnerToString())

		strategy := instance.Spec.GetStrategy()
		_, ok := response.TrafficSplitRecommendation[strategy]
		if !ok {
			err := fmt.Errorf("Missing traffic split recommendation for strategy %s", strategy)
			r.markAnalyticsServiceError(context, instance, "%v", err)
			return err
		}
		trafficSplit := response.TrafficSplitRecommendation[strategy]

		if baselineWeight, ok := trafficSplit[analytics.GetBaselineID()]; ok {
			if instance.Status.Assessment.Baseline.Weight != baselineWeight {
				trafficUpdated = true
			}
			instance.Status.Assessment.Baseline.Weight = baselineWeight
		} else {
			err := fmt.Errorf("traffic split recommendation for baseline not found")
			r.markAnalyticsServiceError(context, instance, "%v", err)
			return err
		}

		for i, candidate := range instance.Status.Assessment.Candidates {
			if candidate.Rollback {
				trafficUpdated = true
				instance.Status.Assessment.Candidates[i].Weight = int32(0)
			} else if weight, ok := trafficSplit[analytics.GetCandidateID(i)]; ok {
				if candidate.Weight != weight {
					trafficUpdated = true
				}
				instance.Status.Assessment.Candidates[i].Weight = weight
			} else {
				err := fmt.Errorf("traffic split recommendation for candidate %s not found", candidate.Name)
				r.markAnalyticsServiceError(context, instance, "%v", err)
				return err
			}
		}

		r.markAnalyticsServiceRunning(context, instance, "")
	}

	if trafficUpdated {
		if err := r.router.UpdateRouteWithTrafficUpdate(instance); err != nil {
			r.markRoutingRulesError(context, instance, "%v", err)
			return err
		}
		r.markTrafficUpdate(context, instance, "Traffic: %s", instance.Status.TrafficToString())
	}

	r.markIterationUpdate(context, instance, "Iteration %d/%d completed", *instance.Status.CurrentIteration, instance.Spec.GetMaxIterations())
	now := metav1.Now()
	instance.Status.LastUpdateTime = &now
	return nil
}

func (r *ReconcileExperiment) updateIteration(instance *iter8v1alpha2.Experiment) {
	*instance.Status.CurrentIteration++
	r.markStatusUpdate()
}
