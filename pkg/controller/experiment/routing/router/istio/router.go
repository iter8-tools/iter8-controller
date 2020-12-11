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

package istio

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	runtime "k8s.io/apimachinery/pkg/runtime"

	iter8v1alpha2 "github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
	"github.com/iter8-tools/iter8/pkg/controller/experiment/routing/router"
	"github.com/iter8-tools/iter8/pkg/controller/experiment/util"
)

const (
	// name of route receiving experimental traffic
	routeNameExperiment = "iter8-experiment"
	// name of route receving non-experimental traffic
	routeNameBase = "iter8-base"

	// the key of label used to reference to the router id
	routerID = "iter8-tools/router"
	// keyword used to replace wildcard host * in label value
	wildcard = "iter8-wildcard-host"
	// prefix of name of routing rules created by iter8
	ruleNameSuffix = "iter8router"

	// labels used in routing rules
	experimentInit  = "iter8-tools/init"
	experimentRole  = "iter8-tools/role"
	experimentLabel = "iter8-tools/experiment"

	// Kiali labels
	kialiWizard          = "kiali_wizard"
	kialiTrafficShifting = "traffic_shifting"

	// values for experimentRole
	roleInitializing = "initializing"
	roleStable       = "stable"
	roleProgressing  = "progressing"

	// SubsetBaseline is name of baseline subset
	SubsetBaseline = "iter8-baseline"
	// SubsetCandidate is name prefix of candidate subset
	SubsetCandidate = "iter8-candidate"
)

var _ router.Interface = &Router{}

type istioRoutingRules struct {
	destinationRule *v1alpha3.DestinationRule
	virtualService  *v1alpha3.VirtualService
}

func (r *istioRoutingRules) isProgressing() bool {
	return r.haveLabels(map[string]string{
		experimentRole: roleProgressing,
	})
}
func (r *istioRoutingRules) isInit() bool {
	return r.haveLabels(map[string]string{
		experimentInit: "True",
	})
}

func (r *istioRoutingRules) isInitializing() bool {
	return r.haveLabels(map[string]string{
		experimentRole: roleInitializing,
	})
}

// check if labels are in routing rules or not
func (r *istioRoutingRules) haveLabels(labels map[string]string) bool {
	for key, val := range labels {
		if r.destinationRule != nil {
			m := r.destinationRule.GetLabels()
			if m == nil {
				return false
			}
			drval, drok := m[key]
			if !drok || drval != val {
				return false
			}
		}

		if r.virtualService != nil {
			m := r.virtualService.GetLabels()
			if m == nil {
				return false
			}
			vsval, vsok := m[key]
			if !vsok || vsval != val {
				return false
			}
		}
	}
	return true
}

func (r *istioRoutingRules) print() string {
	out := ""
	if r.virtualService != nil {
		out += fmt.Sprintf("VirtualService: %+v", r.virtualService)
	}

	if r.destinationRule != nil {
		out += fmt.Sprintf(", DestinationRule: %+v", r.destinationRule)
	}

	return "{" + out + "}"
}

// Router is a router using istio routing rules
type Router struct {
	client  istioclient.Interface
	rules   *istioRoutingRules
	handler destinationHandler
	logger  logr.Logger
}

// GetRouter returns an instance of istio router
func GetRouter(ctx context.Context, instance *iter8v1alpha2.Experiment) router.Interface {
	out := Router{
		client: ctx.Value(util.IstioClientKey).(istioclient.Interface),
		logger: util.Logger(ctx),
	}

	switch instance.Spec.Service.Kind {
	case "Service":
		out.handler = serviceHandler{}
	default:
		out.handler = deploymentHandler{}
	}
	return &out
}

// Print prints detailed information about the router
func (r *Router) Print() string {
	out := "Istio Routes: " + r.rules.print()
	return out
}

// Fetch routing rules from cluster
func (r *Router) Fetch(ctx context.Context, instance *iter8v1alpha2.Experiment) error {
	selector := map[string]string{
		routerID: getRouterID(instance)}

	drl, err := r.client.NetworkingV1alpha3().DestinationRules(instance.ServiceNamespace()).
		List(ctx, metav1.ListOptions{LabelSelector: labels.Set(selector).String()})
	if err != nil {
		return err
	}

	vsl, err := r.client.NetworkingV1alpha3().VirtualServices(instance.ServiceNamespace()).
		List(ctx, metav1.ListOptions{LabelSelector: labels.Set(selector).String()})
	if err != nil {
		return err
	}

	rules, err := r.handler.validateAndInit(drl, vsl, instance)
	if err != nil {
		return err
	}
	r.rules = rules
	return nil
}

// UpdateRouteWithBaseline updates routing rules with runtime object of baseline
func (r *Router) UpdateRouteWithBaseline(ctx context.Context, instance *iter8v1alpha2.Experiment, baseline runtime.Object) (err error) {
	if r.rules.isProgressing() || r.rules.isInitializing() {
		return nil
	}
	service := instance.Spec.Service

	vsb := NewVirtualServiceBuilder(r.rules.virtualService).
		WithExperimentRegistered(util.FullExperimentName(instance)).
		WithRouterRegistered(getRouterID(instance)).
		WithInitializingLabel().
		RemoveKialiLabel().
		InitGateways().
		InitHosts().
		InitHTTPRoutes()

	// inject internal host
	if service.Name != "" {
		vsb = vsb.
			WithHosts([]string{util.ServiceToFullHostName(service.Name, instance.ServiceNamespace())}).
			WithMeshGateway()
	}

	if nwk := instance.Spec.Networking; nwk != nil {
		// inject external hosts
		mHosts, mGateways := make(map[string]bool), make(map[string]bool)
		hosts, gateways := make([]string, 0), make([]string, 0)
		for _, host := range nwk.Hosts {
			if _, ok := mHosts[host.Name]; !ok {
				hosts = append(hosts, host.Name)
				mHosts[host.Name] = true
			}

			if _, ok := mHosts[host.Gateway]; !ok {
				gateways = append(gateways, host.Gateway)
				mGateways[host.Gateway] = true
			}
		}
		vsb = vsb.WithHosts(hosts).WithGateways(gateways)
	}

	experimentRoute := NewEmptyHTTPRoute(routeNameExperiment)

	// inject baseline destination to route
	baselineDestination := r.handler.buildDestination(instance, destinationOptions{
		name:   service.Baseline,
		weight: 100,
		subset: SubsetBaseline,
		port:   service.Port,
	})
	experimentRoute = experimentRoute.WithDestination(baselineDestination)

	// inject match clauses to route
	trafficControl := instance.Spec.TrafficControl
	if trafficControl != nil && trafficControl.Match != nil && len(trafficControl.Match.HTTP) > 0 {
		experimentRoute = experimentRoute.WithHTTPMatch(trafficControl.Match.HTTP)
	}

	// update virtualservice with experiment route
	vsb = vsb.WithHTTPRoute(experimentRoute.Build())

	// inject base-route if matching clauses exist
	if trafficControl != nil && trafficControl.Match != nil && len(trafficControl.Match.HTTP) > 0 {
		baseRoute := NewEmptyHTTPRoute(routeNameBase).WithDestination(baselineDestination)
		vsb = vsb.WithHTTPRoute(baseRoute.Build())
	}
	vs := (*v1alpha3.VirtualService)(nil)
	if _, ok := vsb.GetLabels()[experimentInit]; ok {
		vs, err = r.client.NetworkingV1alpha3().
			VirtualServices(r.rules.virtualService.GetNamespace()).
			Create(ctx, vsb.Build(), metav1.CreateOptions{})
	} else {
		vs, err = r.client.NetworkingV1alpha3().
			VirtualServices(r.rules.virtualService.GetNamespace()).
			Update(ctx, vsb.Build(), metav1.UpdateOptions{})
	}
	if err != nil {
		return err
	}
	r.rules.virtualService = vs.DeepCopy()

	// Update destinationrule
	if r.handler.requireDestinationRule() {
		dr := (*v1alpha3.DestinationRule)(nil)
		drb := NewDestinationRuleBuilder(r.rules.destinationRule).
			InitSubsets().
			WithSubset(baseline.(*appsv1.Deployment), SubsetBaseline).
			WithInitializingLabel().
			RemoveKialiLabel().
			WithRouterRegistered(getRouterID(instance)).
			WithExperimentRegistered(util.FullExperimentName(instance))

		if _, ok := drb.GetLabels()[experimentInit]; ok {
			dr, err = r.client.NetworkingV1alpha3().
				DestinationRules(r.rules.destinationRule.GetNamespace()).
				Create(ctx, drb.Build(), metav1.CreateOptions{})
		} else {
			dr, err = r.client.NetworkingV1alpha3().
				DestinationRules(r.rules.destinationRule.GetNamespace()).
				Update(ctx, drb.Build(), metav1.UpdateOptions{})
		}
		if err != nil {
			return err
		}
		r.rules.destinationRule = dr.DeepCopy()
	}

	instance.Status.Assessment.Baseline.Weight = 100
	return nil
}

// UpdateRouteWithCandidates updates routing rules with runtime objects of candidates
func (r *Router) UpdateRouteWithCandidates(ctx context.Context, instance *iter8v1alpha2.Experiment, candidates []runtime.Object) (err error) {
	if r.rules.isProgressing() {
		return
	}

	vs := r.rules.virtualService

	route := getExperimentRoute(vs)
	if route == nil {
		return fmt.Errorf("Fail to update route with candidates: experiment route missing in vs")
	}
	rb := NewHTTPRoute(route)

	service := instance.Spec.Service
	// update candidates
	for i, candidate := range instance.Spec.Candidates {
		destination := r.handler.buildDestination(instance, destinationOptions{
			name:   candidate,
			weight: 0,
			subset: CandidateSubsetName(i),
			port:   service.Port,
		})

		rb = rb.WithDestination(destination)
	}

	// update vs to progressing
	vs = NewVirtualServiceBuilder(vs).
		WithProgressingLabel().
		Build()

	vs, err = r.client.NetworkingV1alpha3().
		VirtualServices(r.rules.virtualService.GetNamespace()).
		Update(ctx, vs, metav1.UpdateOptions{})
	if err != nil {
		return
	}
	r.rules.virtualService = vs.DeepCopy()

	// Update destination rule to progressing
	if r.handler.requireDestinationRule() {
		drb := NewDestinationRuleBuilder(r.rules.destinationRule)
		for i, candidate := range candidates {
			drb = drb.WithSubset(candidate.(*appsv1.Deployment), CandidateSubsetName(i))
		}

		dr := drb.WithProgressingLabel().Build()

		dr, err = r.client.NetworkingV1alpha3().
			DestinationRules(dr.GetNamespace()).
			Update(ctx, dr, metav1.UpdateOptions{})
		if err != nil {
			return
		}
		r.rules.destinationRule = dr.DeepCopy()
	}
	return
}

// UpdateRouteWithTrafficUpdate updates routing rules with new traffic state from assessment
func (r *Router) UpdateRouteWithTrafficUpdate(ctx context.Context, instance *iter8v1alpha2.Experiment) (err error) {
	vs := r.rules.virtualService
	if route := getExperimentRoute(vs); route != nil {
		r.updateRouteFromExperiment(route, instance)
	}

	vs, err = r.client.NetworkingV1alpha3().VirtualServices(vs.Namespace).Update(ctx, vs, metav1.UpdateOptions{})
	if err != nil {
		return
	}
	r.rules.virtualService = vs.DeepCopy()

	return
}

// UpdateRouteToStable updates routing rules to desired stable state
func (r *Router) UpdateRouteToStable(ctx context.Context, instance *iter8v1alpha2.Experiment) (err error) {
	if r.rules == nil || !(r.rules.isProgressing() || r.rules.isInitializing()) {
		r.logger.Info("NoOpInUpdateRouteToStable", "routing rules not initialized", "")
		return nil
	}

	if instance.Spec.GetCleanup() && r.rules.isInit() {
		// delete routing rules
		if err = r.client.NetworkingV1alpha3().VirtualServices(r.rules.virtualService.Namespace).
			Delete(ctx, r.rules.virtualService.Name, metav1.DeleteOptions{}); err != nil {
			r.logger.Info("Err in deleting vs", "err", err)
			return
		}

		if r.handler.requireDestinationRule() {
			if err = r.client.NetworkingV1alpha3().DestinationRules(r.rules.destinationRule.Namespace).
				Delete(ctx, r.rules.destinationRule.Name, metav1.DeleteOptions{}); err != nil {
				r.logger.Info("Err in deleting dr", "err", err)
				return
			}
		}
	} else {
		// only applied to progressing(fully configured) routing rules
		// otherwise, the routing rule will be remained as its last state
		vs := r.rules.virtualService
		if r.rules.isProgressing() {
			// retain experiment route only, and rename it to base route
			route := getExperimentRoute(vs)

			if route != nil {
				r.updateRouteFromExperiment(route, instance)
				route.Name = ""
				route.Match = nil
				vs = NewVirtualServiceBuilder(vs).
					InitHTTPRoutes().
					WithHTTPRoute(route).
					Build()
			}
		}

		// update vs
		vs = NewVirtualServiceBuilder(vs).
			WithStableLabel().
			WithKialiLabel().
			RemoveExperimentLabel().Build()
		if _, err = r.client.NetworkingV1alpha3().
			VirtualServices(vs.Namespace).
			Update(ctx, vs, metav1.UpdateOptions{}); err != nil {
			return err
		}

		// update dr if required
		if r.handler.requireDestinationRule() {
			dr := NewDestinationRuleBuilder(r.rules.destinationRule).
				WithStableLabel().
				WithKialiLabel().
				RemoveExperimentLabel().
				Build()
			if _, err = r.client.NetworkingV1alpha3().
				DestinationRules(r.rules.destinationRule.Namespace).
				Update(ctx, dr, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Router) updateRouteFromExperiment(route *networkingv1alpha3.HTTPRoute, instance *iter8v1alpha2.Experiment) {
	rb := NewHTTPRoute(route).ClearRoute()
	assessment := instance.Status.Assessment

	// update baseline
	baselineDestination := r.handler.buildDestination(instance, destinationOptions{
		name:   assessment.Baseline.Name,
		weight: assessment.Baseline.Weight,
		subset: SubsetBaseline,
		port:   instance.Spec.Service.Port,
	})

	rb = rb.WithDestination(baselineDestination)

	// update candidates
	for i, candidate := range assessment.Candidates {
		destination := r.handler.buildDestination(instance, destinationOptions{
			name:   candidate.Name,
			weight: candidate.Weight,
			subset: CandidateSubsetName(i),
			port:   instance.Spec.Service.Port,
		})

		rb = rb.WithDestination(destination)
	}
}

func getExperimentRoute(vs *v1alpha3.VirtualService) *networkingv1alpha3.HTTPRoute {
	httproutes := vs.Spec.GetHttp()
	experimentRouteIndex := -1
	for i := range httproutes {
		if httproutes[i].Name == routeNameExperiment {
			experimentRouteIndex = i
			break
		}
	}

	if experimentRouteIndex == -1 {
		// experiment route not found
		return nil
	}

	return httproutes[experimentRouteIndex]
}

// CandidateSubsetName returns subset name of a candidate with respect to its index in service spec
func CandidateSubsetName(idx int) string {
	return SubsetCandidate + "-" + strconv.Itoa(idx)
}

// returns the id of router used by this experiment
func getRouterID(instance *iter8v1alpha2.Experiment) string {
	nwk := instance.Spec.Networking
	if nwk != nil && nwk.ID != nil {
		return *nwk.ID
	}

	host := util.GetDefaultHost(instance)
	if host == "*" {
		return wildcard
	}
	return host
}

// GetRoutingRuleName returns name of routing rule with router id as input
func GetRoutingRuleName(routerID string) string {
	return fmt.Sprintf("%s.%s", routerID, ruleNameSuffix)
}
