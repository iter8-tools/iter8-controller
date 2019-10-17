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
	serviceNamespace := getServiceNamespace(instance)

	// Get k8s service
	service := &corev1.Service{}
	err := r.Get(context, types.NamespacedName{Name: serviceName, Namespace: serviceNamespace}, service)
	if err != nil {
		r.MarkTargetsError(context, instance, "Missing Service %s", serviceName)
		if err := r.Status().Update(context, instance); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Set up vs and dr for experiment
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
		r.MarkRoutingRulesStatus(context, instance, false,
			"RoutingRules Found For Experiment, DR: %s, VS: %s", dr.GetName(), vs.GetName())
	} else if len(drl.Items) == 0 && len(vsl.Items) == 0 {
		// Initialize routing rules if not existed
		if ruleSet, err := initializeRoutingRules(context, r, instance); err != nil {
			r.MarkExperimentFailed(context, instance, "Error in Initializing routing rules: %s, Experiment failed.", err.Error())
			return reconcile.Result{}, r.Status().Update(context, instance)
		} else {
			dr = ruleSet.DestinationRules[0].DeepCopy()
			vs = ruleSet.VirtualServices[0].DeepCopy()
			r.MarkRoutingRulesStatus(context, instance, true,
				"Init Routing Rules Suceeded, DR: %s, VS: %s", dr.GetName(), vs.GetName())
		}
	} else {
		r.MarkRoutingRulesStatus(context, instance, true,
			"Unexpected Condition, Multiple Routing Rules Found, Delete All")
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
		//	log.Info("BaselineDeploymentFound", "Name", baselineName)
	}

	//	Convert state from stable to progressing
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
		//	log.Info("CandidateDeploymentFound", "Name", candidateName)
	}
	if !stable {
		if len(candidate.GetName()) > 0 {
			if updated := updateSubset(dr, candidate, Candidate); updated {
				if err := r.Update(context, dr); err != nil {
					log.Info("ProgressingRuleUpdateFailure", "dr", dr.GetName())
					return reconcile.Result{}, err
				}
				log.Info("ProgressingRuleUpdated", "dr", dr.GetName())
			}
		}
	}

	if baseline.GetName() == "" || candidate.GetName() == "" {
		if baseline.GetName() == "" && candidate.GetName() == "" {
			r.MarkTargetsError(context, instance, "%s", "Missing Baseline And Candidate")
		} else if candidate.GetName() == "" {
			r.MarkTargetsError(context, instance, "%s", "Missing Candidate")
		} else {
			r.MarkTargetsError(context, instance, "%s", "Missing Baseline")
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

	newlyFound := r.MarkTargetsFound(context, instance)
	if newlyFound {
		// Update GrafanaURL
		now := metav1.Now()
		ts := now.UTC().UnixNano() / int64(time.Millisecond)
		instance.Status.StartTimestamp = strconv.FormatInt(ts, 10)
		updateGrafanaURL(instance, getServiceNamespace(instance))
	}

	// check experiment is finished
	if instance.Spec.TrafficControl.GetMaxIterations() <= instance.Status.CurrentIteration ||
		instance.Spec.Assessment != iter8v1alpha1.AssessmentNull {
		log.Info("ExperimentCompleted")

		if instance.Spec.Assessment != iter8v1alpha1.AssessmentNull {
			log.Info("ExperimentStopWithAssessmentFlagSet", "Action", instance.Spec.Assessment)
		}

		if experimentSucceeded(instance) {
			// experiment is successful
			log.Info("ExperimentSucceeded")

			msg := ""
			switch instance.Spec.TrafficControl.GetOnSuccess() {
			case "baseline":
				dr = NewDestinationRuleBuilder(dr).WithProgressingToStable(baseline).Build()
				vs = NewVirtualServiceBuilder(vs).
					WithProgressingToStable(serviceName, serviceNamespace, Baseline).
					Build()
				msg = "AllToBaseline"
				instance.Status.TrafficSplit.Baseline = 100
				instance.Status.TrafficSplit.Candidate = 0
			case "candidate":
				dr = NewDestinationRuleBuilder(dr).WithProgressingToStable(candidate).Build()
				vs = NewVirtualServiceBuilder(vs).
					WithProgressingToStable(serviceName, serviceNamespace, Candidate).
					Build()
				msg = "AllToCandidate"
				instance.Status.TrafficSplit.Baseline = 0
				instance.Status.TrafficSplit.Candidate = 100
			case "both":
				// Change the role of current rules as stable
				vs.ObjectMeta.SetLabels(map[string]string{experimentRole: Stable})
				dr.ObjectMeta.SetLabels(map[string]string{experimentRole: Stable})
				msg = "KeepBothVersions"
			}

			r.MarkExperimentSucceeded(context, instance, "%s", SuccessMsg(instance, msg))
		} else {

			r.MarkExperimentFailed(context, instance, "%s", FailureMsg(instance, "AllToBaseline"))
			dr = NewDestinationRuleBuilder(dr).WithProgressingToStable(baseline).Build()
			vs = NewVirtualServiceBuilder(vs).
				WithProgressingToStable(serviceName, serviceNamespace, Baseline).
				Build()

			instance.Status.TrafficSplit.Baseline = 100
			instance.Status.TrafficSplit.Candidate = 0
		}

		removeExperimentLabel(dr, vs)
		if err := r.Update(context, vs); err != nil {
			log.Error(err, "Fail to update vs %s", vs.GetName())
			return reconcile.Result{}, err
		}
		if err := r.Update(context, dr); err != nil {
			log.Error(err, "Fail to update dr %s", dr.GetName())
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, r.Status().Update(context, instance)
	}

	// Progressing on Experiment
	traffic := instance.Spec.TrafficControl
	now := time.Now()
	// TODO: check err in getting the time value
	interval, _ := traffic.GetIntervalDuration()
	// Check experiment rollout status
	rolloutPercent := float64(getWeight(Candidate, vs))
	if now.After(instance.Status.LastIncrementTime.Add(interval)) {

		switch getStrategy(instance) {
		case "increment_without_check":
			rolloutPercent += traffic.GetStepSize()
		case "check_and_increment":
			// Get latest analysis
			payload, err := MakeRequest(instance, baseline, candidate)
			if err != nil {
				r.MarkAnalyticsServiceError(context, instance, "Can Not Compose Payload %v", err)
				if err := r.Status().Update(context, instance); err != nil {
					return reconcile.Result{}, err
				}

				return reconcile.Result{RequeueAfter: 5 * time.Second}, err
			}

			response, err := checkandincrement.Invoke(log, instance.Spec.Analysis.GetServiceEndpoint(), payload)
			if err != nil {
				r.MarkAnalyticsServiceError(context, instance, "Error From Analytics: %s", err.Error())
				if err := r.Status().Update(context, instance); err != nil {
					return reconcile.Result{}, err
				}
				log.Info("Error From Analytics, requeue after 5s")
				// TODO: not sure why getting the endpoint will cause requeue failing
				// So a manual sleep interval is added here
				time.Sleep(5 * time.Second)
				return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
			}

			if response.Assessment.Summary.AbortExperiment {
				log.Info("ExperimentAborted. Rollback to Baseline.")
				// rollback to baseline and mark experiment as complelete
				dr = NewDestinationRuleBuilder(dr).WithProgressingToStable(baseline).Build()
				vs = NewVirtualServiceBuilder(vs).
					WithProgressingToStable(serviceName, serviceNamespace, Baseline).
					Build()

				removeExperimentLabel(dr, vs)
				if err := r.Update(context, vs); err != nil {
					return reconcile.Result{}, err
				}
				if err := r.Update(context, dr); err != nil {
					return reconcile.Result{}, err
				}

				instance.Status.TrafficSplit.Baseline = 100
				instance.Status.TrafficSplit.Candidate = 0

				r.MarkExperimentFailed(context, instance, "%s", "Aborted, Traffic: AllToBaseline.")
				return reconcile.Result{}, r.Status().Update(context, instance)
			}

			instance.Status.AssessmentSummary = response.Assessment.Summary
			if response.LastState == nil {
				instance.Status.AnalysisState.Raw = []byte("{}")
			} else {
				lastState, err := json.Marshal(response.LastState)
				if err != nil {
					r.MarkAnalyticsServiceError(context, instance, "Error Analytics Response: %v", err)
					return reconcile.Result{}, r.Status().Update(context, instance)
				}
				instance.Status.AnalysisState = runtime.RawExtension{Raw: lastState}
			}

			rolloutPercent = response.Candidate.TrafficPercentage
			r.MarkAnalyticsServiceRunning(context, instance)
		}

		instance.Status.CurrentIteration++
		// Increase the traffic upto max traffic amount
		if rolloutPercent <= traffic.GetMaxTrafficPercentage() &&
			getWeight(Candidate, vs) != int(rolloutPercent) {
			// Update Traffic splitting rule
			vs = NewVirtualServiceBuilder(vs).
				WithRolloutPercent(serviceName, serviceNamespace, int(rolloutPercent)).
				Build()

			if err := r.Update(context, vs); err != nil {
				return reconcile.Result{}, err
			}
			instance.Status.TrafficSplit.Baseline = 100 - int(rolloutPercent)
			instance.Status.TrafficSplit.Candidate = int(rolloutPercent)

			r.MarkExperimentProgress(context, instance, true, "New Traffic, baseline: %d, candidate: %d",
				instance.Status.TrafficSplit.Baseline, instance.Status.TrafficSplit.Candidate)
		}
	}

	instance.Status.LastIncrementTime = metav1.NewTime(now)

	r.MarkExperimentProgress(context, instance, false, "Iteration %d Completed", instance.Status.CurrentIteration)
	return reconcile.Result{RequeueAfter: interval}, r.Status().Update(context, instance)
}

func removeExperimentLabel(objs ...runtime.Object) (err error) {
	for _, obj := range objs {
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return err
		}
		labels := accessor.GetLabels()
		delete(labels, experimentLabel)
		accessor.SetLabels(labels)
	}

	return nil
}

func setLabels(obj runtime.Object, newLabels map[string]string) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	labels := accessor.GetLabels()
	for key, val := range newLabels {
		labels[key] = val
	}
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
		serviceNamespace := getServiceNamespace(instance)

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

				removeExperimentLabel(dr, vs)
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
	for i, http := range vs.Spec.HTTP {
		matchIndex := -1
		for j, route := range http.Route {
			if equalHost(route.Destination.Host, vsNamespace, instance.Spec.TargetService.Name, svcNamespace) {
				// Only one entry of destination is allowed in an HTTP route
				if matchIndex < 0 {
					matchIndex = j
				} else {
					return false
				}
			}
		}
		// Set 100% weight to this host
		if matchIndex >= 0 {
			vs.Spec.HTTP[i].Route[matchIndex].Weight = 100
			return true
		}
	}
	return false
}

func getStableSet(context context.Context, c client.Client, instance *iter8v1alpha1.Experiment) *IstioRoutingSet {
	serviceName := instance.Spec.TargetService.Name
	vsl, drl := &v1alpha3.VirtualServiceList{}, &v1alpha3.DestinationRuleList{}
	listOptions := (&client.ListOptions{}).
		MatchingLabels(map[string]string{experimentRole: Stable, experimentHost: serviceName}).
		InNamespace(instance.GetNamespace())
	// No need to retry if non-empty error returned(empty results are expected)
	c.List(context, listOptions, drl)
	c.List(context, listOptions, vsl)

	out := &IstioRoutingSet{}

	for _, vs := range vsl.Items {
		out.VirtualServices = append(out.VirtualServices, &vs)
	}

	for _, dr := range drl.Items {
		out.DestinationRules = append(out.DestinationRules, &dr)
	}

	return out
}

func detectRoutingReferences(context context.Context, c client.Client, instance *iter8v1alpha1.Experiment) (*IstioRoutingSet, error) {
	log := Logger(context)
	if instance.Spec.RoutingReference == nil {
		log.Info("No RoutingReference Found in experiment")
		return nil, nil
	}
	// Only supports single vs for edge service now
	// TODO: supports DestinationRule as well
	expNamespace := instance.Namespace
	rule := instance.Spec.RoutingReference
	if rule.APIVersion == v1alpha3.SchemeGroupVersion.String() && rule.Kind == "VirtualService" {
		vs := &v1alpha3.VirtualService{}
		ruleNamespace := rule.Namespace
		if ruleNamespace == "" {
			ruleNamespace = expNamespace
		}
		if err := c.Get(context, types.NamespacedName{Name: rule.Name, Namespace: ruleNamespace}, vs); err != nil {
			log.Error(err, "ReferencedRuleNotExisted", "rule", rule)
			return nil, err
		}

		if !validateVirtualService(instance, vs) {
			err := fmt.Errorf("NoMatchedDestinationHostFoundInReferencedRule")
			log.Error(err, "NoMatchedDestinationHostFoundInReferencedRule", "rule", rule)
			return nil, err
		}

		// Detect previous stable rules, if exist, delete them
		// This is based on the assumption that reference rule is of higher priority than iter8 stable rules
		stableSet := getStableSet(context, c, instance)
		if len(stableSet.DestinationRules) > 0 {
			for _, dr := range stableSet.DestinationRules {
				if err := c.Delete(context, dr); err != nil {
					return nil, err
				}
			}
		}
		if len(stableSet.VirtualServices) > 0 {
			for _, vs := range stableSet.VirtualServices {
				if err := c.Delete(context, vs); err != nil {
					return nil, err
				}
			}
		}

		vs.SetLabels(map[string]string{
			experimentLabel: instance.Name,
			experimentHost:  instance.Spec.TargetService.Name,
			experimentRole:  Stable,
		})

		dr := NewDestinationRule(instance.Spec.TargetService.Name, instance.GetName(), instance.GetNamespace()).
			WithStableLabel().
			Build()

		if err := c.Create(context, dr); err != nil {
			log.Error(err, "ReferencedDRCanNotBeCreated", "dr", dr)
			return nil, err
		}

		// update vs
		if err := c.Update(context, vs); err != nil {
			log.Error(err, "ReferencedRuleCanNotBeUpdated", "vs", vs)
			return nil, err
		}

		return &IstioRoutingSet{
			VirtualServices:  []*v1alpha3.VirtualService{vs},
			DestinationRules: []*v1alpha3.DestinationRule{dr},
		}, nil
	}
	return nil, fmt.Errorf("Reference Rule not supported")
}

func initializeRoutingRules(context context.Context, c client.Client, instance *iter8v1alpha1.Experiment) (*IstioRoutingSet, error) {
	log := Logger(context)
	serviceName := instance.Spec.TargetService.Name
	serviceNamespace := instance.Spec.TargetService.Namespace
	if serviceNamespace == "" {
		serviceNamespace = instance.Namespace
	}
	vs, dr := &v1alpha3.VirtualService{}, &v1alpha3.DestinationRule{}

	if refset, err := detectRoutingReferences(context, c, instance); err != nil {
		log.Error(err, "")
		return nil, fmt.Errorf("%s", err)
	} else if refset != nil {
		// Set reference rule as stable rules to this experiment
		log.Info("GetStableRulesFromReferences", "vs", refset.VirtualServices[0].GetName(),
			"dr", refset.DestinationRules[0].GetName())
		return refset, nil
	}

	// Detect stable rules with the same host
	stableSet := getStableSet(context, c, instance)
	drs, vss := stableSet.DestinationRules, stableSet.VirtualServices
	if len(drs) == 1 && len(vss) == 1 {
		dr = drs[0].DeepCopy()
		vs = vss[0].DeepCopy()
		log.Info("StableRulesFound", "dr", dr.GetName(), "vs", vs.GetName())

		// Validate Stable rules
		if !validateVirtualService(instance, vs) {
			return nil, fmt.Errorf("Existing Stable Virtualservice can not serve current experiment")
		}

		// Set Experiment Label to the Routing Rules
		setLabels(dr, map[string]string{experimentLabel: instance.Name})
		setLabels(vs, map[string]string{experimentLabel: instance.Name})
		if err := c.Update(context, vs); err != nil {
			return nil, err
		}
		if err := c.Update(context, dr); err != nil {
			return nil, err
		}
	} else if len(drs) == 0 && len(vss) == 0 {
		// Create Dummy Stable rules
		dr = NewDestinationRule(serviceName, instance.GetName(), instance.GetNamespace()).
			WithStableLabel().
			Build()
		err := c.Create(context, dr)
		if err != nil {
			log.Error(err, "FailToCreateStableDR", "dr", dr.GetName())
			return nil, err
		}
		log.Info("StableRuleCreated", "dr", dr.GetName())

		vs = NewVirtualService(serviceName, instance.GetName(), instance.GetNamespace()).
			WithNewStableSet(serviceName).
			Build()
		err = c.Create(context, vs)
		if err != nil {
			log.Info("FailToCreateStableVS", "vs", vs)
			return nil, err
		}
		log.Info("StableRuleCreated", "vs", vs)
	} else {
		//Unexpected condition, delete all before progressing rules are created
		log.Info("UnexpectedCondition")
		if len(drs) > 0 {
			for _, dr := range drs {
				if err := c.Delete(context, dr); err != nil {
					return nil, err
				}
			}
		}
		if len(vss) > 0 {
			for _, vs := range vss {
				if err := c.Delete(context, vs); err != nil {
					return nil, err
				}
			}
		}
		return nil, fmt.Errorf("UnexpectedContidtion, retrying")
	}

	return &IstioRoutingSet{
		VirtualServices:  []*v1alpha3.VirtualService{vs},
		DestinationRules: []*v1alpha3.DestinationRule{dr},
	}, nil
}
