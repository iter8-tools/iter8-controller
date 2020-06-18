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

	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/targets"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/util"
)

type IstioRoutingRules struct {
	DestinationRule *v1alpha3.DestinationRule
	VirtualService  *v1alpha3.VirtualService
}

func (r *IstioRoutingRules) ToString() string {
	out := ""
	if r.DestinationRule != nil {
		out += "DestinationRule: " + r.DestinationRule.Name
	}

	if r.VirtualService != nil {
		out += ", VirtualSerivce: " + r.VirtualService.Name
	}

	return out
}

func (r *IstioRoutingRules) GetRoutingRules(instance *iter8v1alpha1.Experiment, ic istioclient.Interface) error {
	selector := map[string]string{
		ExperimentHost: util.GetHost(instance)}

	targetKind := instance.Spec.TargetService.Kind

	drl, err := ic.NetworkingV1alpha3().DestinationRules(instance.ServiceNamespace()).
		List(metav1.ListOptions{LabelSelector: labels.Set(selector).String()})
	if err != nil {
		return err
	}

	vsl, err := ic.NetworkingV1alpha3().VirtualServices(instance.ServiceNamespace()).
		List(metav1.ListOptions{LabelSelector: labels.Set(selector).String()})
	if err != nil {
		return err
	}

	if targetKind == "Deployment" || targetKind == "" {
		return r.validateDetectedRulesForDeployments(drl, vsl, instance)
	} else if targetKind == "Service" {
		return r.validateDetectedRulesForServices(drl, vsl, instance)
	}
	return fmt.Errorf("Invalid targetKind %s", targetKind)
}

func (r *IstioRoutingRules) validateDetectedRulesForServices(drl *v1alpha3.DestinationRuleList, vsl *v1alpha3.VirtualServiceList, instance *iter8v1alpha1.Experiment) error {
	if len(vsl.Items) == 0 {
		// init rules
		r.VirtualService = NewVirtualService(util.GetHost(instance), util.FullExperimentName(instance), instance.ServiceNamespace()).
			WithInitLabel().
			Build()

	} else if len(vsl.Items) == 1 {
		vsrole, vsok := vsl.Items[0].GetLabels()[ExperimentRole]
		if vsok {
			if vsrole == Stable {
				// Valid stable rules detected
				r.VirtualService = &v1alpha3.VirtualService{}
				r.VirtualService = vsl.Items[0].DeepCopy()
			} else {
				vsLabel, vsok := vsl.Items[0].GetLabels()[ExperimentLabel]
				if vsok {
					expName := util.FullExperimentName(instance)
					if vsLabel == expName {
						// valid progressing rules found
						r.VirtualService = &v1alpha3.VirtualService{}
						r.VirtualService = vsl.Items[0].DeepCopy()
					} else {
						return fmt.Errorf("Progressing rules of other experiment are detected")
					}
				} else {
					return fmt.Errorf("Experiment label missing in dr or vs")
				}
			}
		} else {
			return fmt.Errorf("experiment role label missing in vs")
		}
	} else {
		return fmt.Errorf("%d vs detected", len(vsl.Items))
	}
	return nil
}

// To validate whether the detected rules can be handled by the experiment
func (r *IstioRoutingRules) validateDetectedRulesForDeployments(drl *v1alpha3.DestinationRuleList, vsl *v1alpha3.VirtualServiceList, instance *iter8v1alpha1.Experiment) error {
	svcNamespace := instance.ServiceNamespace()
	host := util.GetHost(instance)
	expFullName := util.FullExperimentName(instance)
	if len(drl.Items) == 0 && len(vsl.Items) == 0 {
		// init rule
		r.DestinationRule = NewDestinationRule(host, expFullName, svcNamespace).
			WithInitLabel().
			Build()
		r.VirtualService = NewVirtualService(host, expFullName, svcNamespace).
			WithInitLabel().
			Build()
	} else if len(drl.Items) == 1 && len(vsl.Items) == 1 {
		drrole, drok := drl.Items[0].GetLabels()[ExperimentRole]
		vsrole, vsok := vsl.Items[0].GetLabels()[ExperimentRole]
		if drok && vsok {
			if drrole == Stable && vsrole == Stable {
				// Valid stable rules detected
				r.DestinationRule = drl.Items[0].DeepCopy()
				r.VirtualService = vsl.Items[0].DeepCopy()
			} else {
				drLabel, drok := drl.Items[0].GetLabels()[ExperimentLabel]
				vsLabel, vsok := vsl.Items[0].GetLabels()[ExperimentLabel]
				if drok && vsok {
					if drLabel == expFullName && vsLabel == expFullName {
						// valid progressing rules found
						r.DestinationRule = drl.Items[0].DeepCopy()
						r.VirtualService = vsl.Items[0].DeepCopy()
					} else {
						return fmt.Errorf("Progressing rules being involved in other experiments")
					}
				} else {
					return fmt.Errorf("Experiment label missing in dr or vs")
				}
			}
		} else {
			return fmt.Errorf("experiment role label missing in dr or vs")
		}
	} else if len(drl.Items) == 0 && len(vsl.Items) == 1 {
		vsrole, vsok := vsl.Items[0].GetLabels()[ExperimentRole]
		if vsok && vsrole == Stable {
			// Valid stable rules detected
			r.VirtualService = vsl.Items[0].DeepCopy()
			r.DestinationRule = NewDestinationRule(host, expFullName, svcNamespace).
				WithInitLabel().
				Build()
		} else {
			return fmt.Errorf("0 dr and 1 unstable vs found")
		}
	} else {
		return fmt.Errorf("%d dr and %d vs detected", len(drl.Items), len(vsl.Items))
	}

	return nil
}

func (r *IstioRoutingRules) withLabels(labels map[string]string) bool {
	for key, val := range labels {
		if r.DestinationRule != nil {
			drval, drok := r.DestinationRule.GetLabels()[key]
			if !drok || drval != val {
				return false
			}
		}

		if r.VirtualService != nil {
			vsval, vsok := r.VirtualService.GetLabels()[key]
			if !vsok || vsval != val {
				return false
			}
		}
	}
	return true
}

func (r *IstioRoutingRules) isStable() bool {
	return r.withLabels(map[string]string{
		ExperimentRole: Stable,
	})
}

func (r *IstioRoutingRules) isProgressing() bool {
	return r.withLabels(map[string]string{
		ExperimentRole: Progressing,
	})
}

func (r *IstioRoutingRules) isInitializing() bool {
	return r.withLabels(map[string]string{
		ExperimentRole: Initializing,
	})
}

func (r *IstioRoutingRules) IsInit() bool {
	return r.withLabels(map[string]string{
		ExperimentInit: "True",
	})
}

func (r *IstioRoutingRules) isExternal() bool {
	return r.withLabels(map[string]string{
		External: "True",
	})
}

func (r *IstioRoutingRules) Initialize(context context.Context, instance *iter8v1alpha1.Experiment, targets *targets.Targets, ic istioclient.Interface) error {
	if r.isProgressing() || r.isInitializing() {
		return nil
	}
	vsb := NewVirtualServiceBuilder(r.VirtualService).
		WithExperimentRegistered(util.FullExperimentName(instance)).
		WithInitializingLabel().
		InitHosts().
		InitHTTPRoutes()

	if targets.Service != nil {
		vsb = vsb.
			WithHosts([]string{util.ServiceToFullHostName(targets.Service.Name, targets.Service.Namespace)}).
			InitMeshGateway()
	}

	if len(targets.Hosts) > 0 {
		vsb = vsb.WithHosts(targets.Hosts)
	}

	if len(targets.Gateways) > 0 {
		vsb = vsb.WithGateways(targets.Gateways)
	}

	baselineDestination := NewHTTPRouteDestination().WithWeight(100)
	if targets.Baseline.GetObjectKind().GroupVersionKind().Kind == "Service" {
		baselineAccessor, err := meta.Accessor(targets.Baseline)
		if err != nil {
			return err
		}
		baselineDestination = baselineDestination.
			WithHost(util.ServiceToFullHostName(baselineAccessor.GetName(), baselineAccessor.GetNamespace()))
	} else if targets.Baseline.GetObjectKind().GroupVersionKind().Kind == "Deployment" {
		baselineDestination = baselineDestination.
			WithHost(util.ServiceToFullHostName(targets.Service.Name, targets.Service.Namespace)).
			WithSubset(Baseline)
	}
	if targets.Port != nil {
		baselineDestination = baselineDestination.WithPort(uint32(*targets.Port))
	}

	vsb = vsb.WithHTTPRoute(NewEmptyHTTPRoute().WithDestination(baselineDestination.Build()).Build())
	util.Logger(context).Info("ToProgress", "vs", vsb)
	if _, ok := vsb.GetLabels()[ExperimentInit]; ok {
		vs, err := ic.NetworkingV1alpha3().
			VirtualServices(r.VirtualService.GetNamespace()).
			Create(vsb.Build())
		if err != nil {
			return err
		}
		r.VirtualService = vs.DeepCopy()
	} else {
		vs, err := ic.NetworkingV1alpha3().
			VirtualServices(r.VirtualService.GetNamespace()).
			Update(vsb.Build())
		if err != nil {
			return err
		}
		r.VirtualService = vs.DeepCopy()
	}

	// Update destination rule
	if targets.Baseline.GetObjectKind().GroupVersionKind().Kind == "Deployment" {
		drb := NewDestinationRuleBuilder(r.DestinationRule).
			InitSubsets().
			WithSubset(Baseline, (targets.Baseline).(*appsv1.Deployment)).
			WithInitializingLabel().
			WithExperimentRegistered(util.FullExperimentName(instance))

		util.Logger(context).Info("ToProgress", "dr", drb)
		if _, ok := drb.GetLabels()[ExperimentInit]; ok {
			dr, err := ic.NetworkingV1alpha3().
				DestinationRules(r.DestinationRule.GetNamespace()).
				Create(drb.Build())
			if err != nil {
				return err
			}
			r.DestinationRule = dr.DeepCopy()
		} else {
			dr, err := ic.NetworkingV1alpha3().
				DestinationRules(r.DestinationRule.GetNamespace()).
				Update(drb.Build())
			if err != nil {
				return err
			}
			r.DestinationRule = dr.DeepCopy()
		}

	}

	instance.Status.TrafficSplit.Baseline = 100
	instance.Status.TrafficSplit.Candidate = 0
	return nil
}

func (r *IstioRoutingRules) UpdateCandidate(context context.Context, targets *targets.Targets, ic istioclient.Interface) error {
	if r.isProgressing() {
		return nil
	}

	candidateKind := targets.Candidate.GetObjectKind().GroupVersionKind().Kind
	candidateDestination := NewHTTPRouteDestination().WithWeight(0)
	if candidateKind == "Service" {
		candidateAccessor, err := meta.Accessor(targets.Candidate)
		if err != nil {
			return err
		}
		candidateDestination = candidateDestination.
			WithHost(util.ServiceToFullHostName(candidateAccessor.GetName(), candidateAccessor.GetNamespace()))
	} else if candidateKind == "Deployment" {
		candidateDestination = candidateDestination.
			WithHost(util.ServiceToFullHostName(targets.Service.Name, targets.Service.Namespace)).
			WithSubset(Candidate)
	}
	if targets.Port != nil {
		candidateDestination = candidateDestination.WithPort(uint32(*targets.Port))
	}

	// The first route is used by iter8
	NewHTTPRoute(r.VirtualService.Spec.Http[0]).
		WithDestination(candidateDestination.Build())

	vs := NewVirtualServiceBuilder(r.VirtualService).
		WithProgressingLabel().
		Build()

	if vs, err := ic.NetworkingV1alpha3().
		VirtualServices(r.VirtualService.GetNamespace()).
		Update(vs); err != nil {
		return err
	} else {
		r.VirtualService = vs
	}

	// Update destination rule
	if candidateKind == "Deployment" {
		r.DestinationRule = NewDestinationRuleBuilder(r.DestinationRule).
			WithSubset(Candidate, (targets.Candidate).(*appsv1.Deployment)).
			WithProgressingLabel().
			Build()

		if dr, err := ic.NetworkingV1alpha3().
			DestinationRules(r.DestinationRule.GetNamespace()).
			Update(r.DestinationRule); err != nil {
			return err
		} else {
			r.DestinationRule = dr
		}

	}
	return nil
}

func (r *IstioRoutingRules) DeleteAll(ic istioclient.Interface) (err error) {
	if r.DestinationRule != nil {
		if err = ic.NetworkingV1alpha3().DestinationRules(r.DestinationRule.Namespace).
			Delete(r.DestinationRule.Name, &metav1.DeleteOptions{}); err != nil {
			return
		}
	}

	if err = ic.NetworkingV1alpha3().VirtualServices(r.VirtualService.Namespace).
		Delete(r.VirtualService.Name, &metav1.DeleteOptions{}); err != nil {
		return
	}
	return
}

func (r *IstioRoutingRules) Cleanup(context context.Context, instance *iter8v1alpha1.Experiment, ic istioclient.Interface) (err error) {
	if instance.Spec.CleanUp == iter8v1alpha1.CleanUpDelete && r.IsInit() {
		err = r.DeleteAll(ic)
	} else {
		stableRouteDestinationFilter := NewHTTPRouteDestination()

		success := instance.Succeeded()
		onSuccess := instance.Spec.TrafficControl.GetOnSuccess()
		namespace := instance.ServiceNamespace()
		if !success || onSuccess == "baseline" {
			// keep baseline
			switch instance.Spec.TargetService.Kind {
			case "Service":
				stableRouteDestinationFilter = stableRouteDestinationFilter.
					WithHost(util.ServiceToFullHostName(instance.Spec.TargetService.Baseline, namespace))
			case "Deployment", "":
				stableRouteDestinationFilter = stableRouteDestinationFilter.
					WithHost(util.ServiceToFullHostName(instance.Spec.TargetService.Name, namespace)).
					WithSubset(Baseline)
			}

			if r.DestinationRule != nil {
				r.DestinationRule = NewDestinationRuleBuilder(r.DestinationRule).
					ToStable(Baseline).
					WithStableLabel().
					RemoveExperimentLabel().
					Build()
			}

			util.Logger(context).Info("Before", "vs", r.VirtualService)
			r.VirtualService = NewVirtualServiceBuilder(r.VirtualService).
				ToStable(stableRouteDestinationFilter.Build()).
				RemoveExperimentLabel().
				WithStableLabel().
				Build()

			util.Logger(context).Info("After", "vs", r.VirtualService)

			instance.Status.TrafficSplit.Baseline = 100
			instance.Status.TrafficSplit.Candidate = 0
		} else if onSuccess == "candidate" {
			// keep candidate
			switch instance.Spec.TargetService.Kind {
			case "Service":
				stableRouteDestinationFilter = stableRouteDestinationFilter.
					WithHost(util.ServiceToFullHostName(instance.Spec.TargetService.Candidate, namespace))
			case "Deployment", "":
				stableRouteDestinationFilter = stableRouteDestinationFilter.
					WithHost(util.ServiceToFullHostName(instance.Spec.TargetService.Name, namespace)).
					WithSubset(Candidate)
			}

			if r.DestinationRule != nil {
				r.DestinationRule = NewDestinationRuleBuilder(r.DestinationRule).
					ToStable(Candidate).
					WithStableLabel().
					RemoveExperimentLabel().
					Build()
			}

			r.VirtualService = NewVirtualServiceBuilder(r.VirtualService).
				ToStable(stableRouteDestinationFilter.Build()).
				RemoveExperimentLabel().
				WithStableLabel().
				Build()

			instance.Status.TrafficSplit.Baseline = 0
			instance.Status.TrafficSplit.Candidate = 100
		}

		err = r.UpdateRemoveRules(ic)
	}
	return
}

func (r *IstioRoutingRules) UpdateRemoveRules(ic istioclient.Interface) error {
	if r.DestinationRule != nil {
		if dr, err := ic.NetworkingV1alpha3().
			DestinationRules(r.DestinationRule.Namespace).
			Update(r.DestinationRule); err != nil {
			return err
		} else {
			r.DestinationRule = dr.DeepCopy()
		}
	}

	if vs, err := ic.NetworkingV1alpha3().
		VirtualServices(r.VirtualService.Namespace).
		Update(r.VirtualService); err != nil {
		return err
	} else {
		r.VirtualService = vs.DeepCopy()
	}

	return nil
}

func (r *IstioRoutingRules) UpdateRolloutPercent(instance *iter8v1alpha1.Experiment, rolloutPercent int, ic istioclient.Interface) error {
	namespace := instance.ServiceNamespace()
	routeDestinationFilter := NewHTTPRouteDestination()
	if r.DestinationRule == nil {
		routeDestinationFilter = routeDestinationFilter.
			WithHost(util.ServiceToFullHostName(instance.Spec.TargetService.Candidate, namespace))
	} else {
		routeDestinationFilter = routeDestinationFilter.
			WithHost(util.ServiceToFullHostName(instance.Spec.TargetService.Name, namespace)).
			WithSubset(Candidate)
	}

	vs := NewVirtualServiceBuilder(r.VirtualService).
		WithRolloutPercent(routeDestinationFilter.Build(), int32(rolloutPercent)).
		Build()
	if vs, err := ic.NetworkingV1alpha3().VirtualServices(vs.Namespace).Update(vs); err != nil {
		return err
	} else {
		r.VirtualService = vs
	}

	instance.Status.TrafficSplit.Baseline = 100 - rolloutPercent
	instance.Status.TrafficSplit.Candidate = rolloutPercent
	return nil
}
