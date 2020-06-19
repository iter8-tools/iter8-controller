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
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"

	"github.com/iter8-tools/iter8-controller/pkg/analytics"
	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/routing"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/targets"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/util"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
)

func (r *ReconcileExperiment) completeExperiment(context context.Context, instance *iter8v1alpha1.Experiment) error {
	if err := r.cleanUp(context, instance); err != nil {
		return err
	}

	markExperimentCompleted(instance)

	msg := completeStatusMessage(context, instance)
	if instance.Succeeded() {
		r.MarkExperimentSucceeded(context, instance, "%s", msg)
	} else {
		r.MarkExperimentFailed(context, instance, "%s", msg)
	}
	return nil
}

// cleanUp cleans up the resources related to experiment to an end status
func (r *ReconcileExperiment) cleanUp(context context.Context, instance *iter8v1alpha1.Experiment) error {
	r.iter8Cache.RemoveExperiment(instance)
	targets.Cleanup(context, instance, r.Client)
	if err := r.rules.Cleanup(context, instance, r.istioClient); err != nil {
		return err
	}

	util.Logger(context).Info("Cleanup of experiment is done.")
	return nil
}

// returns hard-coded termination message
func completeStatusMessage(context context.Context, instance *iter8v1alpha1.Experiment) string {
	out := ""
	if instance.Action.TerminateExperiment() {
		out = "Abort"
	} else if len(instance.Status.AssessmentSummary.SuccessCriteriaStatus) > 0 {
		if instance.Status.AssessmentSummary.AllSuccessCriteriaMet {
			out = "All Success Criteria Were Met"
		} else {
			out = "Not All Success Criteria Were Met"
		}
	} else if instance.Spec.TrafficControl.GetMaxIterations() < instance.Status.CurrentIteration {
		out = "Last Iteration Was Completed"
	} else {
		out = "Unknown"
	}
	return out
}

func markExperimentCompleted(instance *iter8v1alpha1.Experiment) {
	// Clear analysis state
	instance.Status.AnalysisState.Raw = []byte("{}")

	// Update grafana url
	instance.Status.EndTimestamp = metav1.Now().UTC().UnixNano()
	updateGrafanaURL(instance)

	instance.Status.MarkExperimentCompleted()
}

func (r *ReconcileExperiment) detectRoutingReferences(instance *iter8v1alpha1.Experiment) error {
	if instance.Spec.RoutingReference == nil {
		return nil
	}
	// Only support single vs for edge service now
	// TODO: support DestinationRule as well
	expNamespace := instance.Namespace
	rule := instance.Spec.RoutingReference
	if rule.APIVersion == v1alpha3.SchemeGroupVersion.String() && rule.Kind == "VirtualService" {
		ruleNamespace := rule.Namespace
		if ruleNamespace == "" {
			ruleNamespace = expNamespace
		}
		vs, err := r.istioClient.NetworkingV1alpha3().VirtualServices(ruleNamespace).Get(rule.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Referenced rule does not exist: %s", err.Error())
		}

		if err := validateVirtualService(instance, vs); err != nil {
			return err
		}

		// set stable labels
		vs.SetLabels(map[string]string{
			routing.ExperimentLabel: instance.Name,
			routing.ExperimentHost:  util.ServiceToFullHostName(instance.Spec.TargetService.Name, expNamespace),
			routing.ExperimentRole:  routing.Stable,
		})

		dr := routing.NewDestinationRule(instance.Spec.TargetService.Name, instance.GetName(), ruleNamespace).
			WithStableLabel().
			Build()

		dr, err = r.istioClient.NetworkingV1alpha3().DestinationRules(ruleNamespace).Create(dr)
		if err != nil {
			return err
		}

		r.rules.DestinationRule = dr

		// update vs
		vs, err = r.istioClient.NetworkingV1alpha3().VirtualServices(ruleNamespace).Update(
			routing.NewVirtualServiceBuilder(vs).WithExternalLabel().Build())
		if err != nil {
			return err
		}

		r.rules.VirtualService = vs
		return nil
	}

	return fmt.Errorf("Referenced rule not supported")
}

func (r *ReconcileExperiment) checkOrInitRules(context context.Context, instance *iter8v1alpha1.Experiment) error {
	r.rules = &routing.IstioRoutingRules{}

	err := r.rules.GetRoutingRules(instance, r.istioClient)
	if err != nil {
		r.MarkRoutingRulesError(context, instance,
			"UnexpectedCondition: %s , Experiment Pause", err.Error())
	} else {
		r.MarkRoutingRulesReady(context, instance, "%s", r.rules.ToString())
	}

	return err
}

func validateVirtualService(instance *iter8v1alpha1.Experiment, vs *v1alpha3.VirtualService) error {
	// Look for an entry with destination host the same as target service
	if vs.Spec.Http == nil || len(vs.Spec.Http) == 0 {
		return fmt.Errorf("Empty HttpRoute")
	}

	vsNamespace, svcNamespace := vs.Namespace, instance.Spec.TargetService.Namespace
	if vsNamespace == "" {
		vsNamespace = instance.Namespace
	}
	if svcNamespace == "" {
		svcNamespace = instance.Namespace
	}

	// The first valid entry in http route is used as stable version
	for i, http := range vs.Spec.Http {
		matchIndex := -1
		for j, route := range http.Route {
			if util.EqualHost(route.Destination.Host, vsNamespace, instance.Spec.TargetService.Name, svcNamespace) {
				// Only one entry of destination is allowed in an HTTP route
				if matchIndex < 0 {
					matchIndex = j
				} else {
					return fmt.Errorf("Multiple host-matching routes found")
				}
			}
		}
		// Set 100% weight to this host
		if matchIndex >= 0 {
			vs.Spec.Http[i].Route[matchIndex].Weight = 100
			return nil
		}
	}
	return nil
}

// return true if instance status should be updated
// returns non-nil error if current reconcile request should be terminated right after this function
func (r *ReconcileExperiment) detectTargets(context context.Context, instance *iter8v1alpha1.Experiment) error {
	r.targets = targets.InitTargets(instance, r.Client)
	// Get k8s service
	err := r.targets.GetService(context)
	if err != nil {
		if instance.Status.TargetsFound() {
			r.MarkTargetsError(context, instance, "Service Deleted")
			onDeletedTarget(instance, targets.RoleService)
			return nil
		} else {
			r.MarkTargetsError(context, instance, "Missing Service")
			return err
		}
	}

	// Get baseline deployment and candidate deployment
	if err = r.targets.GetBaseline(context); err == nil {
		if err := r.rules.Initialize(context, instance, r.targets, r.istioClient); err != nil {
			r.MarkTargetsError(context, instance, "%s", err.Error())
			return err
		}
	} else {
		if instance.Status.TargetsFound() {
			r.MarkTargetsError(context, instance, "Baseline Deleted")
			onDeletedTarget(instance, targets.RoleBaseline)
			return nil
		} else {
			r.MarkTargetsError(context, instance, "Missing Baseline")
			return err
		}
	}

	if err = r.targets.GetCandidate(context); err == nil {
		if err = r.rules.UpdateCandidate(context, r.targets, r.istioClient); err != nil {
			r.MarkRoutingRulesError(context, instance, "Fail in updating routing rule: %v", err)
			return err
		}
	} else {
		if instance.Status.TargetsFound() {
			r.MarkTargetsError(context, instance, "Candidate Deleted")
			onDeletedTarget(instance, targets.RoleCandidate)
			return nil
		} else {
			r.MarkTargetsError(context, instance, "Missing Candidate")
			return err
		}
	}

	r.MarkTargetsFound(context, instance)

	return nil
}

// returns non-nil error if reconcile process should be terminated right after this function
func (r *ReconcileExperiment) progressExperiment(context context.Context, instance *iter8v1alpha1.Experiment) error {
	log := util.Logger(context)
	// mark experiment begin
	if instance.Status.StartTimestamp == 0 {
		instance.Status.StartTimestamp = metav1.Now().UTC().UnixNano()
		updateGrafanaURL(instance)
		r.markStatusUpdate()
	}

	traffic := instance.Spec.TrafficControl
	rolloutPercent := instance.Status.TrafficSplit.Candidate
	strategy := instance.GetStrategy()
	algorithm := analytics.GetAlgorithm(strategy)
	if algorithm == nil {
		err := fmt.Errorf("Unsupported Strategy %s", strategy)
		r.MarkAnalyticsServiceError(context, instance, "%s", err.Error())
		return err
	}

	if algorithm.GetPath() == "" {
		rolloutPercent += int(traffic.GetStepSize())
	} else {
		// Get latest analysis
		payload, err := analytics.MakeRequest(instance, algorithm)
		if err != nil {
			r.MarkAnalyticsServiceError(context, instance, "%s", err.Error())
			return err
		}

		response, err := analytics.Invoke(log, instance.Spec.Analysis.GetServiceEndpoint(), payload, algorithm)
		if err != nil {
			r.MarkAnalyticsServiceError(context, instance, "%s", err.Error())
			return err
		}

		// update assessment in instance object
		instance.Status.AssessmentSummary = response.Assessment.Summary
		instance.Status.AssessmentSummary.SuccessCriteriaStatus = response.Assessment.SuccessCriteria

		if response.LastState == nil {
			instance.Status.AnalysisState.Raw = []byte("{}")
		} else {
			lastState, err := json.Marshal(response.LastState)
			if err != nil {
				r.MarkAnalyticsServiceError(context, instance, "%s", err.Error())
				return err
			}
			instance.Status.AnalysisState = runtime.RawExtension{Raw: lastState}
		}

		// abort experiment
		if response.Assessment.Summary.AbortExperiment {
			instance.Action = iter8v1alpha1.ActionOverrideFailure
			return nil
		}

		// read latest rollout percent
		rolloutPercent = int(response.Candidate.TrafficPercentage)
		r.MarkAnalyticsServiceRunning(context, instance)
	}

	// fail in this iteration
	// skip check for the first iteration
	if instance.Status.CurrentIteration > 0 && !instance.Succeeded() {
		r.MarkExperimentProgress(context, instance, true, iter8v1alpha1.ReasonIterationFailed,
			"")
	} else if rolloutPercent <= int(traffic.GetMaxTrafficPercentage()) &&
		instance.Status.TrafficSplit.Candidate != rolloutPercent {
		log.Info("NewTrafficUpdate")
		// Increase the traffic upto max traffic amount
		// Update Traffic splitting rule
		if err := r.rules.UpdateRolloutPercent(instance, rolloutPercent, r.istioClient); err != nil {
			r.MarkRoutingRulesError(context, instance, "%s", err.Error())
			return err
		}

		r.MarkExperimentProgress(context, instance, true, iter8v1alpha1.ReasonIterationSucceeded,
			"New Traffic, baseline: %d, candidate: %d",
			instance.Status.TrafficSplit.Baseline, instance.Status.TrafficSplit.Candidate)
	}

	instance.Status.LastIncrementTime = metav1.NewTime(time.Now())
	instance.Status.CurrentIteration++

	r.MarkExperimentProgress(context, instance, false, iter8v1alpha1.ReasonIterationUpdate,
		"Iteration %d Started", instance.Status.CurrentIteration)
	return nil
}

func onDeletedTarget(instance *iter8v1alpha1.Experiment, role targets.Role) {
	switch role {
	case targets.RoleBaseline:
		instance.Action = iter8v1alpha1.ActionOverrideSuccess
		onsuccess := "candidate"
		instance.Spec.TrafficControl.OnSuccess = &onsuccess
	case targets.RoleCandidate, targets.RoleService:
		instance.Action = iter8v1alpha1.ActionOverrideFailure
	}
}

func updateGrafanaURL(instance *iter8v1alpha1.Experiment) {
	endTsStr := "now"
	if instance.Status.EndTimestamp > 0 {
		endTsStr = strconv.FormatInt(instance.Status.EndTimestamp/int64(time.Millisecond), 10)
	}
	instance.Status.GrafanaURL = instance.Spec.Analysis.GetGrafanaEndpoint() +
		"/d/eXPEaNnZz/iter8-application-metrics?" +
		"var-namespace=" + instance.ServiceNamespace() +
		"&var-service=" + instance.Spec.TargetService.Name +
		"&var-baseline=" + instance.Spec.TargetService.Baseline +
		"&var-candidate=" + instance.Spec.TargetService.Candidate +
		"&from=" + strconv.FormatInt(instance.Status.StartTimestamp/int64(time.Millisecond), 10) +
		"&to=" + endTsStr
}
