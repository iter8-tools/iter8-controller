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
	"strings"
	"time"

	"github.com/iter8-tools/iter8-controller/pkg/analytics/checkandincrement"
	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/knative/pkg/apis/istio/v1alpha3"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ReconcileExperiment) syncKubernetes(context context.Context, instance *iter8v1alpha1.Experiment) (reconcile.Result, error) {
	log := Logger(context)
	serviceName := instance.Spec.TargetService.Name
	serviceNamespace := instance.Spec.TargetService.Namespace
	if serviceNamespace == "" {
		serviceNamespace = instance.Namespace
	}

	// Get k8s service
	service := &corev1.Service{}
	err := r.Get(context, types.NamespacedName{Name: serviceName, Namespace: serviceNamespace}, service)
	if err != nil {
		log.Info("TargetServiceNotFound", "service", serviceName)
		instance.Status.MarkHasNotService("Service Not Found", "")
		err = r.Status().Update(context, instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Set up vs and dr for experiment
	rName := getIstioRuleName(instance)
	dr := &v1alpha3.DestinationRule{}
	vs := &v1alpha3.VirtualService{}

	drl := &v1alpha3.DestinationRuleList{}
	vsl := &v1alpha3.VirtualServiceList{}
	listOptions := (&client.ListOptions{}).
		MatchingLabels(map[string]string{experimentLabel: instance.Name, experimentHost: serviceName}).
		InNamespace(instance.GetNamespace())
	// No need to retry if non-empty error returned(empty results are expected)
	r.List(context, listOptions, drl)
	r.List(context, listOptions, vsl)

	if len(drl.Items) == 1 && len(vsl.Items) == 1 {
		dr = drl.Items[0].DeepCopy()
		vs = vsl.Items[0].DeepCopy()
		log.Info("RoutingRules Found For Experiment", "dr", dr.GetName(), "vs", vs.GetName())
	} else if len(drl.Items) == 0 && len(vsl.Items) == 0 {
		// Initialize routing rules if not existed
		if ruleSet, err := initializeRoutingRules(r, context, instance); err != nil {
			log.Error(err, "Fail To Init Routing Rules")
			return reconcile.Result{}, err
		} else {
			dr = ruleSet.dr.DeepCopy()
			vs = ruleSet.vs.DeepCopy()
			log.Info("Init Routing Rules Suceeded", "dr", dr.GetName(), "vs", vs.GetName())
		}
	} else {
		log.Info("UnexpectedCondition, MultipleRoutingRulesFound, DeleteAll")
		if len(drl.Items) > 0 {
			for _, dr := range drl.Items {
				if err := r.Delete(context, &dr); err != nil {
					return reconcile.Result{}, err
				}
			}
		}
		if len(vsl.Items) > 0 {
			for _, vs := range vsl.Items {
				if err := r.Delete(context, &vs); err != nil {
					return reconcile.Result{}, err
				}
			}
		}
		return reconcile.Result{}, fmt.Errorf("UnexpectedContidtion, retrying")
	}

	stable := false
	if stable, err = isStable(dr); err != nil {
		log.Info("LabelMissingInIstioRule", err)
	}

	baselineName, candidateName := instance.Spec.TargetService.Baseline, instance.Spec.TargetService.Candidate
	baseline, candidate := &appsv1.Deployment{}, &appsv1.Deployment{}
	// Get current deployment and candidate deployment
	if err = r.Get(context, types.NamespacedName{Name: baselineName, Namespace: serviceNamespace}, baseline); err == nil {
		log.Info("BaselineDeploymentFound", "Name", baselineName)
	}

	// Convert state from stable to progressing
	if stable && len(baseline.GetName()) > 0 {
		// Need to pass baseline into the builder
		dr = NewDestinationRuleBuilder(dr).
			WithStableToProgressing(baseline).
			Build()

		if err := r.Update(context, dr); err != nil {
			return reconcile.Result{}, err
		}
		log.Info("ChangedToProgressing", "DR", dr.GetName())

		// Need to change subset stable to baseline
		// Add subset candidate to route
		vs = NewVirtualServiceBuilder(vs).
			WithStableToProgressing(serviceName, serviceNamespace).
			Build()
		if err := r.Update(context, vs); err != nil {
			return reconcile.Result{}, err
		}
		log.Info("ChangedToProgressing", "VS", vs.GetName())
		stable = false
	}

	if err = r.Get(context, types.NamespacedName{Name: candidateName, Namespace: serviceNamespace}, candidate); err == nil {
		log.Info("CandidateDeploymentFound", "Name", candidateName)
	}
	if !stable {
		if len(candidate.GetName()) > 0 {
			if updated := updateSubset(dr, candidate, Candidate); updated {
				if err := r.Update(context, dr); err != nil {
					log.Info("ProgressingRuleUpdateFailure", "dr", rName)
					return reconcile.Result{}, err
				}
				log.Info("ProgressingRuleUpdated", "dr", dr.GetName())
			}
		}
	}

	if baseline.GetName() == "" || candidate.GetName() == "" {
		if baseline.GetName() == "" && candidate.GetName() == "" {
			log.Info("Missing Baseline and Candidate Deployments")
			instance.Status.MarkHasNotService("Baseline and candidate deployments are missing", "")
		} else if candidate.GetName() == "" {
			log.Info("Missing Candidate Deployment")
			instance.Status.MarkHasNotService("Candidate deployment is missing", "")
		} else {
			log.Info("Missing Baseline Deployment")
			instance.Status.MarkHasNotService("Baseline deployment is missing", "")
		}

		if len(baseline.GetName()) > 0 {
			rolloutPercent := getWeight(Candidate, vs)
			instance.Status.TrafficSplit.Baseline = 100 - rolloutPercent
			instance.Status.TrafficSplit.Candidate = rolloutPercent
		}

		err = r.Status().Update(context, instance)
		if err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// All elements in the targetService are found
	instance.Status.MarkHasService()

	// Start Experiment Process
	// Get info on Experiment
	traffic := instance.Spec.TrafficControl
	now := time.Now()
	// TODO: check err in getting the time value
	interval, _ := traffic.GetIntervalDuration()

	if instance.Status.StartTimestamp == "" {
		ts := metav1.NewTime(now).UTC().UnixNano() / int64(time.Millisecond)
		instance.Status.StartTimestamp = strconv.FormatInt(ts, 10)
		updateGrafanaURL(instance, serviceNamespace)
	}

	// check experiment is finished
	if instance.Spec.TrafficControl.GetMaxIterations() <= instance.Status.CurrentIteration ||
		instance.Spec.Assessment != iter8v1alpha1.AssessmentNull {
		log.Info("ExperimentCompleted")
		// remove experiment labels in Routing Rules and Deployments
		if err := removeExperimentLabel(context, r, baseline); err != nil {
			return reconcile.Result{}, err
		}
		if err := removeExperimentLabel(context, r, candidate); err != nil {
			return reconcile.Result{}, err
		}
		// remove experiment labels in Routing Rules and Deployments
		if err := removeExperimentLabel(context, r, vs); err != nil {
			return reconcile.Result{}, err
		}
		if err := removeExperimentLabel(context, r, dr); err != nil {
			return reconcile.Result{}, err
		}

		// Clear analysis state
		instance.Status.AnalysisState.Raw = []byte("{}")

		ts := metav1.NewTime(now).UTC().UnixNano() / int64(time.Millisecond)
		instance.Status.EndTimestamp = strconv.FormatInt(ts, 10)
		updateGrafanaURL(instance, serviceNamespace)

		if instance.Spec.Assessment != iter8v1alpha1.AssessmentNull {
			log.Info("ExperimentStopWithAssessmentFlagSet", "Action", instance.Spec.Assessment)
		}

		if (getStrategy(instance) == "check_and_increment" &&
			(instance.Spec.Assessment == iter8v1alpha1.AssessmentOverrideSuccess || instance.Status.AssessmentSummary.AllSuccessCriteriaMet)) ||
			(getStrategy(instance) == "increment_without_check" &&
				(instance.Spec.Assessment == iter8v1alpha1.AssessmentOverrideSuccess || instance.Spec.Assessment == iter8v1alpha1.AssessmentNull)) {

			// experiment is successful
			log.Info("ExperimentSucceeded: AllSuccessCriteriaMet")
			switch instance.Spec.TrafficControl.GetOnSuccess() {
			case "baseline":
				dr = NewDestinationRuleBuilder(dr).WithProgressingToStable(baseline).Build()
				vs = NewVirtualServiceBuilder(vs).
					WithProgressingToStable(serviceName, serviceNamespace, Baseline).
					Build()

				instance.Status.MarkNotRollForward("Roll Back to Baseline", "")
				instance.Status.TrafficSplit.Baseline = 100
				instance.Status.TrafficSplit.Candidate = 0
			case "candidate":
				dr = NewDestinationRuleBuilder(dr).WithProgressingToStable(candidate).Build()
				vs = NewVirtualServiceBuilder(vs).
					WithProgressingToStable(serviceName, serviceNamespace, Candidate).
					Build()

				instance.Status.MarkRollForward()
				instance.Status.TrafficSplit.Baseline = 0
				instance.Status.TrafficSplit.Candidate = 100
			case "both":
				// Change the role of current rules as stable
				vs.ObjectMeta.SetLabels(map[string]string{experimentRole: Stable})
				dr.ObjectMeta.SetLabels(map[string]string{experimentRole: Stable})
				instance.Status.MarkNotRollForward("Traffic is maintained as end of experiment", "")
			}
		} else {
			log.Info("ExperimentFailure: NotAllSuccessCriteriaMet")

			dr = NewDestinationRuleBuilder(dr).WithProgressingToStable(baseline).Build()
			vs = NewVirtualServiceBuilder(vs).
				WithProgressingToStable(serviceName, serviceNamespace, Baseline).
				Build()

			instance.Status.MarkNotRollForward("ExperimentFailure: Roll Back to Baseline", "")
			instance.Status.TrafficSplit.Baseline = 100
			instance.Status.TrafficSplit.Candidate = 0
		}

		if err := r.Update(context, vs); err != nil {
			log.Error(err, "Fail to update vs %s", vs.GetName())
			return reconcile.Result{}, err
		}
		if err := r.Update(context, dr); err != nil {
			log.Error(err, "Fail to update dr %s", dr.GetName())
			return reconcile.Result{}, err
		}

		instance.Status.MarkExperimentCompleted()
		// End experiment
		err = r.Status().Update(context, instance)
		return reconcile.Result{}, err
	}

	// Check experiment rollout status
	rolloutPercent := float64(getWeight(Candidate, vs))
	if now.After(instance.Status.LastIncrementTime.Add(interval)) {

		switch getStrategy(instance) {
		case "increment_without_check":
			rolloutPercent += traffic.GetStepSize()
		case "check_and_increment":
			// Get latest analysis
			payload := MakeRequest(instance, baseline, candidate)
			response, err := checkandincrement.Invoke(log, instance.Spec.Analysis.GetServiceEndpoint(), payload)
			if err != nil {
				instance.Status.MarkExperimentNotCompleted("Istio Analytics Service is not reachable", "%v", err)
				log.Info("Istio Analytics Service is not reachable", "err", err)
				err = r.Status().Update(context, instance)
				return reconcile.Result{RequeueAfter: 5 * time.Second}, err
			}

			if response.Assessment.Summary.AbortExperiment {
				log.Info("ExperimentAborted. Rollback to Baseline.")
				// rollback to baseline and mark experiment as complelete
				dr = NewDestinationRuleBuilder(dr).WithProgressingToStable(baseline).Build()
				vs = NewVirtualServiceBuilder(vs).
					WithProgressingToStable(serviceName, serviceNamespace, Baseline).
					Build()

				if err := r.Update(context, vs); err != nil {
					log.Error(err, "Fail to update vs %s", vs.GetName())
					return reconcile.Result{}, err
				}
				if err := r.Update(context, dr); err != nil {
					log.Error(err, "Fail to update dr %s", dr.GetName())
					return reconcile.Result{}, err
				}

				instance.Status.MarkNotRollForward("AbortExperiment: Roll Back to Baseline", "")
				instance.Status.TrafficSplit.Baseline = 100
				instance.Status.TrafficSplit.Candidate = 0
				instance.Status.MarkExperimentCompleted()

				ts := metav1.NewTime(now).UTC().UnixNano() / int64(time.Millisecond)
				instance.Status.EndTimestamp = strconv.FormatInt(ts, 10)
				updateGrafanaURL(instance, serviceNamespace)
				// End experiment
				err = r.Status().Update(context, instance)
				return reconcile.Result{}, err
			}

			baselineTraffic := response.Baseline.TrafficPercentage
			candidateTraffic := response.Canary.TrafficPercentage
			log.Info("NewTraffic", "Baseline", baselineTraffic, "Candidate", candidateTraffic)
			rolloutPercent = candidateTraffic

			instance.Status.AssessmentSummary = response.Assessment.Summary
			if response.LastState == nil {
				instance.Status.AnalysisState.Raw = []byte("{}")
			} else {
				lastState, err := json.Marshal(response.LastState)
				if err != nil {
					instance.Status.MarkExperimentNotCompleted("ErrorAnalyticsResponse", "%v", err)
					err = r.Status().Update(context, instance)
					return reconcile.Result{}, err
				}
				instance.Status.AnalysisState = runtime.RawExtension{Raw: lastState}
			}
		}

		instance.Status.CurrentIteration++
		log.Info("IterationUpdated", "count", instance.Status.CurrentIteration)
		// Increase the traffic upto max traffic amount
		if rolloutPercent <= traffic.GetMaxTrafficPercentage() && getWeight(Candidate, vs) != int(rolloutPercent) {
			// Update Traffic splitting rule
			log.Info("RolloutPercentUpdated", "NewWeight", rolloutPercent)
			vs = NewVirtualServiceBuilder(vs).
				WithRolloutPercent(serviceName, serviceNamespace, int(rolloutPercent)).
				Build()

			err := r.Update(context, vs)
			if err != nil {
				log.Info("RuleUpdateError", "vs", vs)
				return reconcile.Result{}, err
			}
			instance.Status.TrafficSplit.Baseline = 100 - int(rolloutPercent)
			instance.Status.TrafficSplit.Candidate = int(rolloutPercent)
		}
	}

	instance.Status.LastIncrementTime = metav1.NewTime(now)

	instance.Status.MarkExperimentNotCompleted("Progressing", "")
	err = r.Status().Update(context, instance)
	return reconcile.Result{RequeueAfter: interval}, err
}

func updateGrafanaURL(instance *iter8v1alpha1.Experiment, namespace string) {
	endTs := instance.Status.EndTimestamp
	if endTs == "" {
		endTs = "now"
	}
	instance.Status.GrafanaURL = instance.Spec.Analysis.GetGrafanaEndpoint() +
		"/d/eXPEaNnZz/iter8-application-metrics?" +
		"var-namespace=" + namespace +
		"&var-service=" + instance.Spec.TargetService.Name +
		"&var-baseline=" + instance.Spec.TargetService.Baseline +
		"&var-candidate=" + instance.Spec.TargetService.Candidate +
		"&from=" + instance.Status.StartTimestamp +
		"&to=" + endTs
}

func removeExperimentLabel(context context.Context, r *ReconcileExperiment, obj runtime.Object) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	labels := accessor.GetLabels()
	delete(labels, experimentLabel)
	delete(labels, experimentRole)
	accessor.SetLabels(labels)
	if err = r.Update(context, obj); err != nil {
		return err
	}
	Logger(context).Info("ExperimentLabelRemoved", "obj", accessor.GetName())
	return nil
}

func updateSubset(dr *v1alpha3.DestinationRule, d *appsv1.Deployment, name string) bool {
	update, found := true, false
	for idx, subset := range dr.Spec.Subsets {
		if subset.Name == Stable && name == Baseline {
			dr.Spec.Subsets[idx].Name = name
			dr.Spec.Subsets[idx].Labels = d.Spec.Template.Labels
			found = true
			break
		}
		if subset.Name == name {
			found = true
			update = false
			break
		}
	}

	if !found {
		dr.Spec.Subsets = append(dr.Spec.Subsets, v1alpha3.Subset{
			Name:   name,
			Labels: d.Spec.Template.Labels,
		})
	}
	return update
}

func getWeight(subset string, vs *v1alpha3.VirtualService) int {
	for _, route := range vs.Spec.HTTP[0].Route {
		if route.Destination.Subset == subset {
			return route.Weight
		}
	}
	return 0
}

func (r *ReconcileExperiment) finalizeIstio(context context.Context, instance *iter8v1alpha1.Experiment) (reconcile.Result, error) {
	completed := instance.Status.GetCondition(iter8v1alpha1.ExperimentConditionExperimentCompleted)
	if completed != nil && completed.Status != corev1.ConditionTrue {
		// Get baseline deployment
		baselineName := instance.Spec.TargetService.Baseline
		baseline := &appsv1.Deployment{}
		serviceNamespace := instance.Spec.TargetService.Namespace
		if serviceNamespace == "" {
			serviceNamespace = instance.Namespace
		}

		if err := r.Get(context, types.NamespacedName{Name: baselineName, Namespace: serviceNamespace}, baseline); err != nil {
			Logger(context).Info("BaselineNotFoundWhenDeleted", "name", baselineName)
		} else {
			// Do a rollback
			// Find routing rules
			drl := &v1alpha3.DestinationRuleList{}
			vsl := &v1alpha3.VirtualServiceList{}
			dr := &v1alpha3.DestinationRule{}
			vs := &v1alpha3.VirtualService{}
			listOptions := (&client.ListOptions{}).
				MatchingLabels(map[string]string{experimentLabel: instance.Name, experimentHost: instance.Spec.TargetService.Name}).
				InNamespace(instance.GetNamespace())
			// No need to retry if non-empty error returned(empty results are expected)
			r.List(context, listOptions, drl)
			r.List(context, listOptions, vsl)

			if len(drl.Items) > 0 && len(vsl.Items) > 0 {
				dr = NewDestinationRuleBuilder(&drl.Items[0]).WithProgressingToStable(baseline).Build()
				vs = NewVirtualServiceBuilder(&vsl.Items[0]).WithProgressingToStable(instance.Spec.TargetService.Name, serviceNamespace, Baseline).Build()

				Logger(context).Info("StableRoutingRulesAfterFinalizing", "dr", dr, "vs", vs)

				if err := r.Update(context, vs); err != nil {
					log.Error(err, "Fail to update vs %s", vs.GetName())
					return reconcile.Result{}, err
				}
				if err := r.Update(context, dr); err != nil {
					log.Error(err, "Fail to update dr %s", dr.GetName())
					return reconcile.Result{}, err
				}
			}
		}
	}

	return reconcile.Result{}, removeFinalizer(context, r, instance, Finalizer)
}

func getIstioRuleName(instance *iter8v1alpha1.Experiment) string {
	return instance.GetName() + IstioRuleSuffix
}

func getStrategy(instance *iter8v1alpha1.Experiment) string {
	strategy := instance.Spec.TrafficControl.GetStrategy()
	if strategy == "check_and_increment" &&
		(instance.Spec.Analysis.SuccessCriteria == nil || len(instance.Spec.Analysis.SuccessCriteria) == 0) {
		strategy = "increment_without_check"
	}
	return strategy
}

func validateVirtualService(instance *iter8v1alpha1.Experiment, vs *v1alpha3.VirtualService) bool {
	// Look for an entry with destination host the same as target service
	if vs.Spec.HTTP == nil || len(vs.Spec.HTTP) == 0 {
		return false
	}

	vsNamespace, svcNamespace := vs.Namespace, instance.Spec.TargetService.Namespace
	if vsNamespace == "" {
		vsNamespace = instance.Namespace
	}
	if svcNamespace == "" {
		svcNamespace = instance.Namespace
	}

	// The first valid entry in http route is used as stable version
	for _, http := range vs.Spec.HTTP {
		found := false
		for _, route := range http.Route {
			if equalHost(route.Destination.Host, vsNamespace, instance.Spec.TargetService.Name, svcNamespace) {
				// Only one entry of destination is allowed in an HTTP route
				if !found {
					found = true
				} else {
					return false
				}
			}
		}
		if found {
			return true
		}
	}
	return false
}

func detectRoutingReferences(r *ReconcileExperiment, context context.Context, instance *iter8v1alpha1.Experiment) (*IstioRoutingSet, error) {
	log := Logger(context)
	if instance.Spec.RoutingReference == nil {
		return nil, nil
	}
	// Only supports single vs for edge service now
	// TODO: supports DestinationRule as well
	expNamespace := instance.Namespace
	rule := instance.Spec.RoutingReference
	if rule.APIVersion == "networking.istio.io/v1alpha3" && rule.Kind == "VirtualService" {
		vs := &v1alpha3.VirtualService{}
		ruleNamespace := rule.Namespace
		if ruleNamespace == "" {
			ruleNamespace = expNamespace
		}
		if err := r.Get(context, types.NamespacedName{Name: rule.Name, Namespace: ruleNamespace}, vs); err != nil {
			log.Error(err, "ReferencedRuleNotExisted", "rule", rule)
			return nil, err
		}

		if !validateVirtualService(instance, vs) {
			err := fmt.Errorf("NoMatchedDestinationHostFoundInReferencedRule")
			log.Error(err, "NoMatchedDestinationHostFoundInReferencedRule", "rule", rule)
			return nil, err
		}

		vs.SetLabels(map[string]string{
			experimentLabel: instance.Name,
			experimentHost:  instance.Spec.TargetService.Name,
			experimentRole:  Stable,
		})

		// Get backend deployment based on host service selector labels
		// Get the first deployment pop up
		destinationHost := vs.Spec.HTTP[0].Route[0].Destination.Host
		s := strings.Split(destinationHost, ".")
		svcName := s[0]
		svcNamespace := ruleNamespace
		if len(s) > 1 {
			svcNamespace = s[1]
		}

		service := &corev1.Service{}
		if err := r.Get(context, types.NamespacedName{Name: svcName, Namespace: svcNamespace}, service); err != nil {
			log.Error(err, "CanNotGetServiceFromDestinationHost", "dh", destinationHost)
			return nil, err
		}

		listOptions := (&client.ListOptions{}).
			MatchingLabels(service.Spec.Selector).
			InNamespace(svcNamespace)

		dl := &appsv1.DeploymentList{}
		if err := r.List(context, listOptions, dl); err != nil || len(dl.Items) == 0 {
			log.Error(err, "CanNotGetDeploymentBehindHostService", "dh", destinationHost)
			return nil, err
		}

		dr := NewDestinationRule(destinationHost, instance.GetName(), instance.GetNamespace()).
			WithStableDeployment(&dl.Items[0]).
			Build()

		if err := r.Create(context, dr); err != nil {
			log.Error(err, "ReferencedDRCanNotBeCreated", "dr", dr)
			return nil, err
		}

		vs = NewVirtualServiceBuilder(vs).
			AppendStableSubset(instance.Spec.TargetService.Name, expNamespace).
			Build()

		// update vs
		if err := r.Update(context, vs); err != nil {
			log.Error(err, "ReferencedRuleCanNotBeUpdated", "vs", vs)
			return nil, err
		}

		return &IstioRoutingSet{
			vs: vs,
			dr: dr,
		}, nil
	}
	return nil, fmt.Errorf("Reference Rule not supported")
}

func initializeRoutingRules(r *ReconcileExperiment, context context.Context, instance *iter8v1alpha1.Experiment) (*IstioRoutingSet, error) {
	log := Logger(context)
	serviceName := instance.Spec.TargetService.Name
	serviceNamespace := instance.Spec.TargetService.Namespace
	if serviceNamespace == "" {
		serviceNamespace = instance.Namespace
	}
	if refset, err := detectRoutingReferences(r, context, instance); err != nil {
		log.Error(err, "")
		return nil, fmt.Errorf("%s", err)
	} else if refset != nil {
		// Set reference rule as stable rules to this experiment
		log.Info("GetStableRulesFromReferences", "vs", refset.vs.GetName(), "dr", refset.dr.GetName())
		return refset, nil
	}

	// Detect stable rules with the same host
	vs, dr := &v1alpha3.VirtualService{}, &v1alpha3.DestinationRule{}
	vsl, drl := &v1alpha3.VirtualServiceList{}, &v1alpha3.DestinationRuleList{}
	listOptions := (&client.ListOptions{}).
		MatchingLabels(map[string]string{experimentRole: Stable, experimentHost: serviceName}).
		InNamespace(instance.GetNamespace())
	// No need to retry if non-empty error returned(empty results are expected)
	r.List(context, listOptions, drl)
	r.List(context, listOptions, vsl)

	if len(drl.Items) == 1 && len(vsl.Items) == 1 {
		dr = drl.Items[0].DeepCopy()
		vs = vsl.Items[0].DeepCopy()
		log.Info("StableRulesFound", "dr", dr.GetName(), "vs", vs.GetName())

		// Validate Stable rules
		if !validateVirtualService(instance, vs) {
			return nil, fmt.Errorf("Existing Stable Virtualservice can not serve current experiment")
		}

		// Set Experiment Label to the Routing Rules
		vs.SetLabels(map[string]string{experimentLabel: instance.Name})
		dr.SetLabels(map[string]string{experimentLabel: instance.Name})
		if err := r.Update(context, vs); err != nil {
			return nil, err
		}
		if err := r.Update(context, dr); err != nil {
			return nil, err
		}
	} else if len(drl.Items) == 0 && len(vsl.Items) == 0 {
		// Create Dummy Stable rules
		dr = NewDestinationRule(serviceName, instance.GetName(), instance.GetNamespace()).
			WithStableLabel().
			Build()
		err := r.Create(context, dr)
		if err != nil {
			log.Error(err, "FailToCreateStableDR", "dr", dr.GetName())
			return nil, err
		}
		log.Info("StableRuleCreated", "dr", dr.GetName())

		vs = NewVirtualService(serviceName, instance.GetName(), instance.GetNamespace()).
			WithNewStableSet(serviceName).
			Build()
		err = r.Create(context, vs)
		if err != nil {
			log.Info("FailToCreateStableVS", "vs", vs)
			return nil, err
		}
		log.Info("StableRuleCreated", "vs", vs)
	} else {
		//Unexpected condition, delete all before progressing rules are created
		log.Info("UnexpectedCondition")
		if len(drl.Items) > 0 {
			for _, dr := range drl.Items {
				if err := r.Delete(context, &dr); err != nil {
					return nil, err
				}
			}
		}
		if len(vsl.Items) > 0 {
			for _, vs := range vsl.Items {
				if err := r.Delete(context, &vs); err != nil {
					return nil, err
				}
			}
		}
		return nil, fmt.Errorf("UnexpectedContidtion, retrying")
	}

	return &IstioRoutingSet{
		vs: vs,
		dr: dr,
	}, nil
}
