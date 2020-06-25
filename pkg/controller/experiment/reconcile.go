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

	"github.com/iter8-tools/iter8-controller/pkg/analytics"
	iter8v1alpha2 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha2"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/targets"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/util"
)

func (r *ReconcileExperiment) completeExperiment(context context.Context, instance *iter8v1alpha2.Experiment) error {
	r.iter8Cache.RemoveExperiment(instance)
	r.targets.Cleanup(context, instance)
	err := r.router.Cleanup(context, instance)
	if err != nil {
		return err
	}

	msg := completeStatusMessage(context, instance)
	r.markExperimentCompleted(context, instance, "%s", msg)
	return nil
}

// returns hard-coded termination message
func completeStatusMessage(context context.Context, instance *iter8v1alpha2.Experiment) string {
	// TODO: might need more detailed situations
	if experimentAbstract(context) != nil && experimentAbstract(context).Terminate() {
		return experimentAbstract(context).GetTerminateStatus()
	} else if instance.Spec.Terminate() {
		return "Abort"
	} else if instance.Spec.GetMaxIterations() < *instance.Status.CurrentIteration {
		return "Last Iteration Was Completed"
	}

	return "Error"
}

func (r *ReconcileExperiment) checkOrInitRules(context context.Context, instance *iter8v1alpha2.Experiment) error {
	// log := util.Logger(context)

	err := r.router.GetRoutingRules(instance)
	if err != nil {
		r.markRoutingRulesError(context, instance, "Error in getting routing rules: %s, Experiment Ended.", err.Error())
		r.completeExperiment(context, instance)
	} else {
		r.markRoutingRulesReady(context, instance, "")
	}

	return err
}

// return true if instance status should be updated
// returns non-nil error if current reconcile request should be terminated right after this function
func (r *ReconcileExperiment) detectTargets(context context.Context, instance *iter8v1alpha2.Experiment) (err error) {
	// serviceName := instance.Spec.Service.Name
	// serviceNamespace := instance.ServiceNamespace()

	if err = r.targets.GetService(context, instance); err != nil {
		if instance.Status.TargetsFound() {
			r.markTargetsError(context, instance, "Service Deleted")
			onDeletedTarget(instance, targets.RoleService)
			return nil
		} else {
			r.markTargetsError(context, instance, "Missing Service")
			return err
		}
	}

	if err = r.targets.GetBaseline(context, instance); err != nil {
		if instance.Status.TargetsFound() {
			r.markTargetsError(context, instance, "Baseline Deleted")
			onDeletedTarget(instance, targets.RoleBaseline)
			return nil
		} else {
			r.markTargetsError(context, instance, "Missing Baseline")
			return err
		}
	} else {
		// ToProgressing will create DestinationRule and VirtualService if needed
		if err = r.router.ToProgressing(instance, r.targets); err != nil {
			r.markRoutingRulesError(context, instance, "Fail in updating routing rule: %v", err)
			return err
		}
	}

	if err = r.targets.GetCandidates(context, instance); err != nil {
		if instance.Status.TargetsFound() {
			r.markTargetsError(context, instance, "Candidate Deleted")
			onDeletedTarget(instance, targets.RoleCandidate)
			return nil
		} else {
			r.markTargetsError(context, instance, "Err in getting candidates: %v", err)
			return err
		}
	} else {
		if err = r.router.UpdateCandidates(r.targets); err != nil {
			r.markRoutingRulesError(context, instance, "Fail in updating routing rule: %v", err)
			return err
		}
	}

	r.markTargetsFound(context, instance, "")

	return nil
}

// returns non-nil error if reconcile process should be terminated right after this function
func (r *ReconcileExperiment) updateIteration(context context.Context, instance *iter8v1alpha2.Experiment) error {
	log := util.Logger(context)
	// mark experiment begin
	if instance.Status.StartTimestamp == nil {
		startTime := metav1.Now()
		instance.Status.StartTimestamp = &startTime
		r.grafanaConfig.UpdateGrafanaURL(instance)
		r.markStatusUpdate()
		// TODO rethink the else wrapper. Added to prevent immediate call to
		// analytics engine who can't deal with an interval of 0s
	} else {
		if len(instance.Spec.Criteria) == 0 {
			// each candidate gets maxincrement traffic at each interval
			// until no more traffic can be deducted from baseline
			basetraffic := instance.Status.Assessment.Baseline.Weight
			diff := instance.Spec.GetMaxIncrements() * int32(len(instance.Spec.Candidates))
			if basetraffic-diff >= 0 {
				instance.Status.Assessment.Baseline.Weight = basetraffic - diff
				for _, candidate := range instance.Status.Assessment.Candidates {
					candidate.Weight += instance.Spec.GetMaxIncrements()
				}
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

			instance.Status.Assessment.Baseline.VersionAssessment = response.BaselineAssessment
			for i, ca := range response.CandidateAssessments {
				instance.Status.Assessment.Candidates[i].VersionAssessment = ca.VersionAssessment
				instance.Status.Assessment.Candidates[i].Rollback = ca.Rollback
			}

			instance.Status.Assessment.Winner = &response.WinnerAssessment

			strategy := instance.Spec.GetStrategy()
			trafficSplit, ok := response.TrafficSplitRecommendation[strategy]
			if !ok {
				err := fmt.Errorf("Missing trafficSplitRecommendation for strategy %s", strategy)
				r.markAnalyticsServiceError(context, instance, "%v", err)
				return err
			}

			trafficUpdated := false
			if baselineWeight, ok := trafficSplit[instance.Spec.Baseline]; ok {
				if instance.Status.Assessment.Baseline.Weight != baselineWeight {
					trafficUpdated = true
				}
				instance.Status.Assessment.Baseline.Weight = baselineWeight
			} else {
				err := fmt.Errorf("trafficSplitRecommendation for baseline not found")
				r.markAnalyticsServiceError(context, instance, "%v", err)
				return err
			}

			for _, candidate := range instance.Status.Assessment.Candidates {
				if candidate.Rollback {
					trafficUpdated = true
					candidate.Weight = int32(0)
				} else if weight, ok := trafficSplit[candidate.Name]; ok {
					if candidate.Weight != weight {
						trafficUpdated = true
					}
					candidate.Weight = weight
				} else {
					err := fmt.Errorf("trafficSplitRecommendation for candidate %s not found", candidate.Name)
					r.markAnalyticsServiceError(context, instance, "%v", err)
					return err
				}
			}

			if trafficUpdated {
				if err := r.router.UpdateTrafficSplit(instance); err != nil {
					r.markRoutingRulesError(context, instance, "%v", err)
					return err
				}
			}

			r.markAnalyticsServiceRunning(context, instance, "")
		}
	}

	now := metav1.Now()
	instance.Status.LastUpdateTime = &now
	*instance.Status.CurrentIteration++
	r.markIterationUpdate(context, instance, "")
	return nil
}

func onDeletedTarget(instance *iter8v1alpha2.Experiment, role targets.Role) {
	instance.Spec.ManualOverride = &iter8v1alpha2.ManualOverride{
		Action: iter8v1alpha2.ActionTerminate,
	}
	switch role {
	case targets.RoleBaseline:
		// Keep traffic status
	case targets.RoleCandidate, targets.RoleService:
		// Send all traffic to baseline
		instance.Spec.ManualOverride.TrafficSplit = map[string]int32{
			instance.Spec.Service.Baseline: 100,
		}
	}
}
