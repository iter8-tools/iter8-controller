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

package routing

import (
	"context"
	"fmt"
	"strconv"

	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	iter8v1alpha2 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha2"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/targets"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/util"
)

const (
	SubsetBaseline  = "iter8-baseline" // use of a '.' is invalid
	SubsetCandidate = "iter8-candidate"
	SubsetStable    = "iter8-stable"

	RoleInitializing = "initializing"
	RoleStable       = "stable"
	RoleProgressing  = "progressing"

	ExperimentInit  = "iter8-tools/init"
	ExperimentRole  = "iter8-tools/role"
	ExperimentLabel = "iter8-tools/experiment"
	ExperimentHost  = "iter8-tools/host"

	ExternalReference = "iter8-tools/external"
)

type istioRoutingRules struct {
	destinationRule *v1alpha3.DestinationRule
	virtualService  *v1alpha3.VirtualService
}

type Router struct {
	client istioclient.Interface
	rules  istioRoutingRules
}

// Init returns an initialized router
func Init(client istioclient.Interface) *Router {
	return &Router{
		rules: istioRoutingRules{
			destinationRule: &v1alpha3.DestinationRule{},
			virtualService:  &v1alpha3.VirtualService{},
		},
		client: client,
	}
}

func (r *istioRoutingRules) isProgressing() bool {
	drRole, drok := r.destinationRule.GetLabels()[ExperimentRole]
	vsRole, vsok := r.virtualService.GetLabels()[ExperimentRole]

	return drok && vsok && drRole == RoleProgressing && vsRole == RoleProgressing
}

func (r *istioRoutingRules) isInit() bool {
	_, drok := r.destinationRule.GetLabels()[ExperimentInit]
	_, vsok := r.virtualService.GetLabels()[ExperimentInit]

	return drok && vsok
}

func (r *istioRoutingRules) isDestinationRuleDefined() bool {
	return "" != r.destinationRule.Name
}

func (r *istioRoutingRules) isVirtualServiceDefined() bool {
	return "" != r.virtualService.Name
}

func (r *istioRoutingRules) isExternalReference() bool {
	_, vsok := r.virtualService.GetLabels()[ExternalReference]

	return vsok
}

func (r *Router) UpdateBaseline(instance *iter8v1alpha2.Experiment, targets *targets.Targets) (err error) {
	if r.rules.isProgressing() {
		return nil
	}

	drb := (*DestinationRuleBuilder)(nil)
	if r.rules.isDestinationRuleDefined() {
		drb = NewDestinationRuleBuilder(r.rules.destinationRule)
	} else {
		drb = NewDestinationRule(instance.Spec.Service.Name, instance.GetName(), instance.ServiceNamespace()).
			WithInitLabel()
	}
	drb = drb.
		InitSubsets(1).
		WithSubset(targets.Baseline, SubsetBaseline, 0).
		WithExperimentRegistered(instance.Name)

	dr := (*v1alpha3.DestinationRule)(nil)
	if r.rules.isDestinationRuleDefined() {
		dr, err = r.client.NetworkingV1alpha3().
			DestinationRules(r.rules.destinationRule.GetNamespace()).
			Update(drb.Build())
	} else {
		dr, err = r.client.NetworkingV1alpha3().
			DestinationRules(instance.ServiceNamespace()).
			Create(drb.Build())
	}
	if err != nil {
		return
	}

	vsb := (*VirtualServiceBuilder)(nil)
	if r.rules.isVirtualServiceDefined() {
		vsb = NewVirtualServiceBuilder(r.rules.virtualService)
	} else {
		vsb = NewVirtualService(instance.Spec.Service.Name, instance.GetName(), instance.ServiceNamespace()).
			WithInitLabel()
	}
	vsb = vsb.
		WithExperimentRegistered(instance.Name)

	if r.rules.isExternalReference() {
		vsb = vsb.ExternalToProgressing(instance.Spec.Service.Name, instance.ServiceNamespace(), len(targets.Candidates))
	} else {
		vsb = vsb.ToProgressing(instance.Spec.Service.Name, len(targets.Candidates))
		trafficControl := instance.Spec.TrafficControl
		if trafficControl != nil && trafficControl.Match != nil && len(trafficControl.Match.HTTP) > 0 {
			vsb = vsb.WithHTTPMatch(trafficControl.Match.HTTP)
		}
	}

	vs := (*v1alpha3.VirtualService)(nil)
	if r.rules.isVirtualServiceDefined() {
		vs, err = r.client.NetworkingV1alpha3().
			VirtualServices(r.rules.virtualService.GetNamespace()).
			Update(vsb.Build())
	} else {
		vs, err = r.client.NetworkingV1alpha3().
			VirtualServices(instance.ServiceNamespace()).
			Create(vsb.Build())
	}
	if err != nil {
		return err
	}

	r.rules.virtualService = vs.DeepCopy()
	r.rules.destinationRule = dr.DeepCopy()

	instance.Status.Assessment.Baseline.Weight = 100
	return
}

func candiateSubsetName(idx int) string {
	return SubsetCandidate + "-" + strconv.Itoa(idx)
}

func (r *Router) UpdateCandidates(targets *targets.Targets) (err error) {
	if r.rules.isProgressing() {
		return nil
	}

	drb := NewDestinationRuleBuilder(r.rules.destinationRule)
	for i, candidate := range targets.Candidates {
		drb = drb.WithSubset(candidate, candiateSubsetName(i), i+1)
	}
	drb = drb.WithProgressingLabel()

	if dr, err := r.client.NetworkingV1alpha3().
		DestinationRules(r.rules.destinationRule.GetNamespace()).
		Update(drb.Build()); err != nil {
		return err
	} else {
		r.rules.destinationRule = dr.DeepCopy()
	}

	vsb := NewVirtualServiceBuilder(r.rules.virtualService).WithProgressingLabel()

	if vs, err := r.client.NetworkingV1alpha3().
		VirtualServices(r.rules.virtualService.GetNamespace()).
		Update(vsb.Build()); err != nil {
		return err
	} else {
		r.rules.virtualService = vs.DeepCopy()
	}

	return
}

// Cleanup configures routing rules to set up traffic to desired end state
func (r *Router) Cleanup(context context.Context, instance *iter8v1alpha2.Experiment) (err error) {
	if instance.Spec.Cleanup != nil && *instance.Spec.Cleanup && r.rules.isInit() {
		if err = r.client.NetworkingV1alpha3().DestinationRules(r.rules.destinationRule.Namespace).
			Delete(r.rules.destinationRule.Name, &metav1.DeleteOptions{}); err != nil {
			return
		}

		if err = r.client.NetworkingV1alpha3().VirtualServices(r.rules.virtualService.Namespace).
			Delete(r.rules.virtualService.Name, &metav1.DeleteOptions{}); err != nil {
			return
		}
	} else {
		toStableSubset := make(map[string]string)
		subsetWeight := make(map[string]int32)
		assessment := instance.Status.Assessment
		switch instance.Spec.GetOnTermination() {
		case iter8v1alpha2.OnTerminationToWinner:
			if assessment != nil && assessment.Winner != nil && assessment.Winner.WinnerFound {
				// change winner version to stable
				for i, candidate := range instance.Spec.Candidates {
					if candidate == *assessment.Winner.Winner {
						toStableSubset[candiateSubsetName(i)] = SubsetStable
						subsetWeight[SubsetStable] = 100
						break
					}
				}
			} else {
				// change baseline to stable
				toStableSubset[SubsetBaseline] = SubsetStable
				subsetWeight[SubsetStable] = 100
			}
		case iter8v1alpha2.OnTerminationToBaseline:
			// change baseline to stable
			toStableSubset[SubsetBaseline] = SubsetStable
			subsetWeight[SubsetStable] = 100
		case iter8v1alpha2.OnTerminationKeepLast:
			// change all subset to stable-0, stable-1...
			if assessment != nil {
				stableCnt := 0
				if assessment.Baseline.Weight > 0 {
					stableSubset := SubsetStable + "-" + strconv.Itoa(stableCnt)
					toStableSubset[SubsetBaseline] = stableSubset
					subsetWeight[stableSubset] = assessment.Baseline.Weight
					stableCnt++
				}
				for i, candidate := range assessment.Candidates {
					if candidate.Weight > 0 {
						stableSubset := SubsetStable + "-" + strconv.Itoa(stableCnt)
						toStableSubset[candiateSubsetName(i)] = stableSubset
						subsetWeight[stableSubset] = candidate.Weight
						stableCnt++
					}
				}
			}
		}

		if _, err = r.client.NetworkingV1alpha3().
			DestinationRules(r.rules.destinationRule.Namespace).
			Update(NewDestinationRuleBuilder(r.rules.destinationRule).
				ProgressingToStable(toStableSubset).
				WithStableLabel().
				RemoveExperimentLabel().
				Build()); err != nil {
			return
		}

		if _, err = r.client.NetworkingV1alpha3().
			VirtualServices(r.rules.virtualService.Namespace).
			Update(NewVirtualServiceBuilder(r.rules.virtualService).
				ProgressingToStable(subsetWeight, instance.Spec.Service.Name, instance.ServiceNamespace()).
				WithStableLabel().
				RemoveExperimentLabel().
				Build()); err != nil {
			return
		}
	}
	return
}

// UpdateTrafficSplit updates virtualservice with latest traffic split
func (r *Router) UpdateTrafficSplit(instance *iter8v1alpha2.Experiment) error {
	subset2Weight := make(map[string]int32)
	subset2Weight[SubsetBaseline] = instance.Status.Assessment.Baseline.Weight
	for i := range instance.Spec.Candidates {
		subset2Weight[SubsetCandidate+"-"+strconv.Itoa(i)] = instance.Status.Assessment.Candidates[i].Weight
	}

	vs := NewVirtualServiceBuilder(r.rules.virtualService).
		WithTrafficSplit(instance.Spec.Service.Name, subset2Weight).
		Build()

	if vs, err := r.client.NetworkingV1alpha3().VirtualServices(vs.Namespace).Update(vs); err != nil {
		return err
	} else {
		r.rules.virtualService = vs.DeepCopy()
	}

	return nil
}

// GetRoutingRules will inject routing rules into router or return error if there is any
func (r *Router) GetRoutingRules(instance *iter8v1alpha2.Experiment) error {
	selector := map[string]string{ExperimentHost: instance.Spec.Service.Name}
	drl, err := r.client.NetworkingV1alpha3().DestinationRules(instance.ServiceNamespace()).
		List(metav1.ListOptions{LabelSelector: labels.Set(selector).String()})
	if err != nil {
		return err
	}

	vsl, err := r.client.NetworkingV1alpha3().VirtualServices(instance.ServiceNamespace()).
		List(metav1.ListOptions{LabelSelector: labels.Set(selector).String()})
	if err != nil {
		return err
	}

	if len(drl.Items) == 0 && len(vsl.Items) == 0 {
		// Defer initialization of routing rules until targets identified
		// Initialize routing rules
		// if err = r.InitRoutingRules(instance); err != nil {
		// 	return err
		// }
	} else {
		if err = r.validateDetectedRules(drl, vsl, instance); err != nil {
			return err
		}
	}
	return nil
}

// GetRoutingRuleName returns namespaced name of routing rulesin the router
func (r *Router) GetRoutingRuleName() string {
	out := ""
	if r.rules.destinationRule != nil {
		out += "DestinationRule: " + r.rules.destinationRule.Name + "." + r.rules.destinationRule.Namespace
	}
	if r.rules.virtualService != nil {
		out += ", VirtualService: " + r.rules.virtualService.Name + "." + r.rules.virtualService.Namespace
	}

	return out
}

// To validate whether the detected rules can be handled by the experiment
func (r *Router) validateDetectedRules(drl *v1alpha3.DestinationRuleList, vsl *v1alpha3.VirtualServiceList, instance *iter8v1alpha2.Experiment) error {
	// should only be one set of rules for stable or progressing
	if len(drl.Items) == 1 && len(vsl.Items) == 1 {
		drrole, drok := drl.Items[0].GetLabels()[ExperimentRole]
		vsrole, vsok := vsl.Items[0].GetLabels()[ExperimentRole]
		if drok && vsok {
			if drrole == RoleInitializing || vsrole == RoleInitializing {
				// Valid initializing rules detected
				r.rules.destinationRule = drl.Items[0].DeepCopy()
				r.rules.virtualService = vsl.Items[0].DeepCopy()
			} else if drrole == RoleStable && vsrole == RoleStable {
				// Valid stable rules detected
				r.rules.destinationRule = drl.Items[0].DeepCopy()
				r.rules.virtualService = vsl.Items[0].DeepCopy()
			} else if drrole == RoleProgressing && vsrole == RoleProgressing {
				drLabel, drok := drl.Items[0].GetLabels()[ExperimentLabel]
				vsLabel, vsok := vsl.Items[0].GetLabels()[ExperimentLabel]
				if drok && vsok {
					expName := instance.GetName()
					if drLabel == expName && vsLabel == expName {
						// valid progressing rules found
						r.rules.destinationRule = drl.Items[0].DeepCopy()
						r.rules.virtualService = vsl.Items[0].DeepCopy()
					} else {
						return fmt.Errorf("Progressing rules of other experiment are detected")
					}
				} else {
					return fmt.Errorf("Host label missing in dr or vs")
				}
			} else {
				return fmt.Errorf("Invalid role specified in dr or vs")
			}
		} else {
			return fmt.Errorf("experiment role label missing in dr or vs")
		}
	} else {
		return fmt.Errorf("%d dr and %d vs detected", len(drl.Items), len(vsl.Items))
	}

	return nil
}

// InitRoutingRules creates routing rules for experiment
func (r *Router) InitRoutingRules(instance *iter8v1alpha2.Experiment) error {
	serviceName := instance.Spec.Service.Name
	serviceNamespace := instance.ServiceNamespace()

	if instance.Spec.RoutingReference != nil {
		if err := r.detectRoutingReferences(instance); err != nil {
			return err
		}
	} else {
		dr, err := r.client.NetworkingV1alpha3().DestinationRules(serviceNamespace).Create(
			NewDestinationRule(serviceName, instance.GetName(), serviceNamespace).
				WithInitLabel().
				Build())
		if err != nil {
			return err
		}

		vs, err := r.client.NetworkingV1alpha3().VirtualServices(serviceNamespace).Create(
			NewVirtualService(serviceName, instance.GetName(), serviceNamespace).
				WithInitLabel().
				Build())
		if err != nil {
			return err
		}

		r.rules.destinationRule = dr.DeepCopy()
		r.rules.virtualService = vs.DeepCopy()
	}

	return nil
}

func (r *Router) detectRoutingReferences(instance *iter8v1alpha2.Experiment) error {
	reference := instance.Spec.RoutingReference
	if reference.APIVersion == v1alpha3.SchemeGroupVersion.String() && reference.Kind == "VirtualService" {
		ruleNamespace := reference.Namespace
		if ruleNamespace == "" {
			ruleNamespace = instance.Namespace
		}

		vs, err := r.client.NetworkingV1alpha3().VirtualServices(ruleNamespace).Get(reference.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Fail to read referenced rule: %s", err.Error())
		}

		if err := validateVirtualService(instance, vs); err != nil {
			return err
		}

		vs, err = r.client.NetworkingV1alpha3().VirtualServices(ruleNamespace).Update(
			NewVirtualServiceBuilder(vs).
				WithExperimentRegistered(instance.Name).
				WithHostRegistered(instance.Spec.Service.Name).
				WithExternalLabel().
				Build())
		if err != nil {
			return err
		}

		dr, err := r.client.NetworkingV1alpha3().DestinationRules(ruleNamespace).Create(
			NewDestinationRule(instance.Spec.Service.Name, instance.GetName(), ruleNamespace).
				WithStableLabel().
				Build())
		if err != nil {
			return err
		}

		r.rules.destinationRule = dr.DeepCopy()
		r.rules.virtualService = vs.DeepCopy()
		return nil
	}
	return fmt.Errorf("Referenced rule not supported")
}

func validateVirtualService(instance *iter8v1alpha2.Experiment, vs *v1alpha3.VirtualService) error {
	// Look for an entry with destination host the same as target service
	if vs.Spec.Http == nil || len(vs.Spec.Http) == 0 {
		return fmt.Errorf("Empty HttpRoute")
	}

	vsNamespace, svcNamespace := vs.Namespace, instance.ServiceNamespace()
	if vsNamespace == "" {
		vsNamespace = instance.Namespace
	}

	// The first valid entry in http route is used as stable version
	for i, http := range vs.Spec.Http {
		matchIndex := -1
		for j, route := range http.Route {
			if util.EqualHost(route.Destination.Host, vsNamespace, instance.Spec.Service.Name, svcNamespace) {
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
