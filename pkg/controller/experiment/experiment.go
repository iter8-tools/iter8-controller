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
	"k8s.io/apimachinery/pkg/labels"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/iter8-tools/iter8-controller/pkg/analytics"
	"github.com/iter8-tools/iter8-controller/pkg/analytics/checkandincrement"
	"github.com/iter8-tools/iter8-controller/pkg/analytics/epsilongreedy"
	"github.com/iter8-tools/iter8-controller/pkg/analytics/pbr"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"

	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
)

func (r *ReconcileExperiment) checkExperimentComplete(context context.Context, instance *iter8v1alpha1.Experiment) (bool, error) {
	// check experiment is finished
	if instance.Spec.TrafficControl.GetMaxIterations() < instance.Status.CurrentIteration ||
		instance.Spec.Assessment != iter8v1alpha1.AssessmentNull {
		if err := r.targets.Cleanup(context, instance, r.Client); err != nil {
			return true, err
		}
		if err := r.rules.Cleanup(instance, r.targets, r.istioClient); err != nil {
			return true, err
		}
		log.Info("Cleanup of experiment is done.")
		r.setExperimentEndStatus(context, instance)
		return true, nil
	}

	return false, nil
}

func (r *ReconcileExperiment) getRoutingLists(namespace string, m map[string]string) (*v1alpha3.DestinationRuleList, *v1alpha3.VirtualServiceList, error) {
	if drl, err := r.istioClient.NetworkingV1alpha3().DestinationRules(namespace).
		List(metav1.ListOptions{LabelSelector: labels.Set(m).String()}); err != nil {

		return nil, nil, err
	} else {
		if vsl, err := r.istioClient.NetworkingV1alpha3().VirtualServices(namespace).
			List(metav1.ListOptions{LabelSelector: labels.Set(m).String()}); err == nil {
			return drl, vsl, nil

		}
		return nil, nil, err
	}
}

func (r *ReconcileExperiment) checkOrInitRules(context context.Context, instance *iter8v1alpha1.Experiment) (bool, error) {
	serviceNs := getServiceNamespace(instance)
	drl, vsl, err := r.getRoutingLists(serviceNs,
		map[string]string{experimentHost: instance.Spec.TargetService.Name})
	updateStatus := true

	if err != nil || len(drl.Items) == 0 && len(vsl.Items) == 0 {
		Logger(context).Info("NoRulesDetected")
		// Initialize routing rules
		if err = r.initializeRoutingRules(instance); err != nil {
			r.MarkRoutingRulesError(context, instance, "Error in Initializing routing rules: %s, Experiment Ended.", err.Error())
			r.MarkExperimentFailed(context, instance, "Error in Initializing routing rules: %s, Experiment Ended.", err.Error())
		} else {
			r.MarkRoutingRulesReady(context, instance,
				"Init Routing Rules Suceeded, DR: %s, VS: %s",
				r.rules.DestinationRule.GetName(), r.rules.VirtualService.GetName())
		}
	} else {
		if r.rules, err = validateDetectedRules(drl, vsl, instance); err == nil {
			r.MarkRoutingRulesReady(context, instance,
				"RoutingRules detected, DR: %s, VS: %s",
				r.rules.DestinationRule.GetName(), r.rules.VirtualService.GetName())
			updateStatus = false
		} else {
			r.MarkRoutingRulesError(context, instance,
				"UnexpectedCondition: %s , Experiment Ended", err.Error())
			r.MarkExperimentFailed(context, instance, "UnexpectedCondition: %s , Experiment Ended", err.Error())
		}
	}

	return updateStatus, err
}

func (r *ReconcileExperiment) initializeRoutingRules(instance *iter8v1alpha1.Experiment) (err error) {
	serviceName := instance.Spec.TargetService.Name
	serviceNamespace := instance.Spec.TargetService.Namespace
	if serviceNamespace == "" {
		serviceNamespace = instance.Namespace
	}

	r.rules = &IstioRoutingRules{}

	if instance.Spec.RoutingReference != nil {
		if err = r.detectRoutingReferences(instance); err != nil {
			return err
		}
	} else {
		// Create Dummy Stable rules
		dr := NewDestinationRule(serviceName, instance.GetName(), serviceNamespace).
			WithStableLabel().
			WithInitLabel().
			Build()
		dr, err = r.istioClient.NetworkingV1alpha3().DestinationRules(serviceNamespace).Create(dr)
		if err != nil {
			return err
		}

		log.Info("NewRuleCreated", "dr", dr.GetName())

		vs := NewVirtualService(serviceName, instance.GetName(), serviceNamespace).
			WithNewStableSet(serviceName).
			WithInitLabel().
			Build()
		vs, err = r.istioClient.NetworkingV1alpha3().VirtualServices(serviceNamespace).Create(vs)
		if err != nil {
			return err
		}

		log.Info("NewRuleCreated", "vs", vs.GetName())

		r.rules.DestinationRule = dr
		r.rules.VirtualService = vs

	}

	return nil
}

func (r *ReconcileExperiment) detectRoutingReferences(instance *iter8v1alpha1.Experiment) error {
	if instance.Spec.RoutingReference == nil {
		log.Info("NoExternalReference")
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
			experimentLabel: instance.Name,
			experimentHost:  instance.Spec.TargetService.Name,
			experimentRole:  Stable,
		})

		dr := NewDestinationRule(instance.Spec.TargetService.Name, instance.GetName(), ruleNamespace).
			WithStableLabel().
			Build()

		dr, err = r.istioClient.NetworkingV1alpha3().DestinationRules(ruleNamespace).Create(dr)
		if err != nil {
			return err
		}

		r.rules.DestinationRule = dr

		// update vs
		vs, err = r.istioClient.NetworkingV1alpha3().VirtualServices(ruleNamespace).Update(vs)
		if err != nil {
			return err
		}

		r.rules.VirtualService = vs
		return nil
	}
	return fmt.Errorf("Referenced rule not supported")
}

func (r *ReconcileExperiment) setExperimentEndStatus(context context.Context, instance *iter8v1alpha1.Experiment) (err error) {
	if experimentSucceeded(instance) {
		// experiment is successful

		switch instance.Spec.TrafficControl.GetOnSuccess() {
		case "baseline":
			instance.Status.TrafficSplit.Baseline = 100
			instance.Status.TrafficSplit.Candidate = 0
		case "candidate":
			instance.Status.TrafficSplit.Baseline = 0
			instance.Status.TrafficSplit.Candidate = 100
		case "both":
		}

		r.MarkExperimentSucceeded(context, instance, "%s", successMsg(instance))
	} else {
		r.MarkExperimentFailed(context, instance, "%s", failureMsg(instance))
		instance.Status.TrafficSplit.Baseline = 100
		instance.Status.TrafficSplit.Candidate = 0
	}

	return
}

func (r *ReconcileExperiment) detectTargets(context context.Context, instance *iter8v1alpha1.Experiment) (bool, error) {
	serviceName := instance.Spec.TargetService.Name
	serviceNamespace := getServiceNamespace(instance)

	if r.targets == nil {
		r.targets = InitTargets()
	}
	// Get k8s service
	err := r.Get(context, types.NamespacedName{Name: serviceName, Namespace: serviceNamespace}, r.targets.Service)
	if err != nil {
		r.MarkTargetsError(context, instance, "MissingService")
		return true, fmt.Errorf("Missing service %s", serviceName)
	}

	baselineName, candidateName := instance.Spec.TargetService.Baseline, instance.Spec.TargetService.Candidate
	// Get baseline deployment and candidate deployment
	if err = r.Get(context, types.NamespacedName{Name: baselineName, Namespace: serviceNamespace}, r.targets.Baseline); err == nil {
		//	Convert state from stable to progressing
		if r.rules.IsStable() {
			// Need to pass baseline into the builder
			if err := r.rules.StableToProgressing(r.targets, instance.Name, serviceNamespace, r.istioClient); err != nil {
				r.MarkTargetsError(context, instance, "%s", err.Error())
				return true, err
			}

			instance.Status.TrafficSplit.Baseline = 100
		}
	} else {
		r.MarkTargetsError(context, instance, "Missing Baseline")
		return true, fmt.Errorf("Missing Baseline %s", baselineName)
	}

	if err = r.Get(context, types.NamespacedName{Name: candidateName, Namespace: serviceNamespace}, r.targets.Candidate); err == nil {
		if updateSubset(r.rules.DestinationRule, r.targets.Candidate, Candidate) {
			uddr, err := r.istioClient.NetworkingV1alpha3().
				DestinationRules(r.rules.DestinationRule.GetNamespace()).
				Update(r.rules.DestinationRule)
			if err != nil {
				return false, err
			}
			r.rules.DestinationRule = uddr
		}
	} else {
		r.MarkTargetsError(context, instance, "Missing Candidate")
		return true, fmt.Errorf("Missing Candidate %s", candidateName)
	}

	if r.MarkTargetsFound(context, instance) {
		// Update GrafanaURL
		now := metav1.Now()
		ts := now.UTC().UnixNano() / int64(time.Millisecond)
		instance.Status.StartTimestamp = strconv.FormatInt(ts, 10)
		updateGrafanaURL(instance, getServiceNamespace(instance))
		return true, nil
	}
	return false, nil
}

// To validate whether the detected rules can be handled by the experiment
func validateDetectedRules(drl *v1alpha3.DestinationRuleList, vsl *v1alpha3.VirtualServiceList, instance *iter8v1alpha1.Experiment) (*IstioRoutingRules, error) {
	out := &IstioRoutingRules{}
	// should only be one set of rules for stable or progressing
	if len(drl.Items) == 1 && len(vsl.Items) == 1 {
		drrole, drok := drl.Items[0].GetLabels()[experimentRole]
		vsrole, vsok := vsl.Items[0].GetLabels()[experimentRole]
		if drok && vsok {
			if drrole == Stable && vsrole == Stable {
				// Valid stable rules detected
				out.DestinationRule = drl.Items[0].DeepCopy()
				out.VirtualService = vsl.Items[0].DeepCopy()
			} else if drrole == Progressing && vsrole == Progressing {
				drLabel, drok := drl.Items[0].GetLabels()[experimentLabel]
				vsLabel, vsok := vsl.Items[0].GetLabels()[experimentLabel]
				if drok && vsok {
					expName := instance.GetName()
					if drLabel == expName && vsLabel == expName {
						// valid progressing rules found
						out.DestinationRule = drl.Items[0].DeepCopy()
						out.VirtualService = vsl.Items[0].DeepCopy()
					} else {
						return nil, fmt.Errorf("Progressing rules of other experiment are detected")
					}
				} else {
					return nil, fmt.Errorf("Host label missing in dr or vs")
				}
			} else {
				return nil, fmt.Errorf("Invalid role specified in dr or vs")
			}
		} else {
			return nil, fmt.Errorf("experiment role label missing in dr or vs")
		}
	} else {
		return nil, fmt.Errorf("%d dr and %d vs detected", len(drl.Items), len(vsl.Items))
	}

	return out, nil
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
			if equalHost(route.Destination.Host, vsNamespace, instance.Spec.TargetService.Name, svcNamespace) {
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

func (r *ReconcileExperiment) progressExperiment(context context.Context, instance *iter8v1alpha1.Experiment) error {
	// Progressing on Experiment
	traffic := instance.Spec.TrafficControl
	rolloutPercent := r.rules.GetWeight(Candidate)
	strategy := getStrategy(instance)
	if iter8v1alpha1.StrategyIncrementWithoutCheck == strategy {
		rolloutPercent += int32(traffic.GetStepSize())
	} else {
		var analyticsService analytics.AnalyticsService
		switch getStrategy(instance) {
		case checkandincrement.Strategy:
			analyticsService = checkandincrement.GetService()
		case epsilongreedy.Strategy:
			analyticsService = epsilongreedy.GetService()
		case pbr.Strategy:
			analyticsService = pbr.GetService()
		}

		// Get latest analysis
		payload, err := analyticsService.MakeRequest(instance, r.targets.Baseline, r.targets.Candidate)
		if err != nil {
			r.MarkAnalyticsServiceError(context, instance, "%s", err.Error())
			return err
		}

		response, err := analyticsService.Invoke(log, instance.Spec.Analysis.GetServiceEndpoint(), payload, analyticsService.GetPath())
		if err != nil {
			r.MarkAnalyticsServiceError(context, instance, "%s", err.Error())
			return err
		}

		// update summary in instance object
		instance.Status.AssessmentSummary = response.Assessment.Summary
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
			return r.abortExperiment(context, instance)
		}

		// read latest rollout percent
		rolloutPercent = int32(response.Candidate.TrafficPercentage)
		r.MarkAnalyticsServiceRunning(context, instance)
	}

	// Increase the traffic upto max traffic amount
	if rolloutPercent <= int32(traffic.GetMaxTrafficPercentage()) &&
		r.rules.GetWeight(Candidate) != rolloutPercent {
		// Update Traffic splitting rule
		if err := r.rules.UpdateRolloutPercent(instance.Spec.TargetService.Name, getServiceNamespace(instance), rolloutPercent, r.istioClient); err != nil {
			r.MarkRoutingRulesError(context, instance, "%s", err.Error())
			return err
		}
		instance.Status.TrafficSplit.Baseline = 100 - int(rolloutPercent)
		instance.Status.TrafficSplit.Candidate = int(rolloutPercent)

		r.MarkExperimentProgress(context, instance, true, "New Traffic, baseline: %d, candidate: %d",
			instance.Status.TrafficSplit.Baseline, instance.Status.TrafficSplit.Candidate)
	}

	instance.Status.LastIncrementTime = metav1.NewTime(time.Now())
	instance.Status.CurrentIteration++
	r.MarkExperimentProgress(context, instance, false, "Iteration %d Started", instance.Status.CurrentIteration)
	return nil

}

func (r *ReconcileExperiment) abortExperiment(context context.Context, instance *iter8v1alpha1.Experiment) (err error) {
	// rollback to baseline and mark experiment as complete
	if err = r.targets.Cleanup(context, instance, r.Client); err != nil {
		return
	}
	if err = r.rules.Cleanup(instance, r.targets, r.istioClient); err != nil {
		return
	}

	instance.Status.TrafficSplit.Baseline = 100
	instance.Status.TrafficSplit.Candidate = 0

	r.MarkExperimentFailed(context, instance, "%s", "Aborted")
	return
}
