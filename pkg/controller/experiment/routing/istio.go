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
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"

	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
)

const (
	IstioRuleSuffix = ".iter8-experiment"

	Baseline     = "baseline"
	Candidate    = "candidate"
	Stable       = "stable"
	Progressing  = "progressing"
	Initializing = "initializing"
	External     = "external"

	ExperimentInit  = "iter8-tools/init"
	ExperimentRole  = "iter8-tools/role"
	ExperimentLabel = "iter8-tools/experiment"
	ExperimentHost  = "iter8-tools/host"
)

type DestinationRuleBuilder v1alpha3.DestinationRule
type VirtualServiceBuilder v1alpha3.VirtualService

func NewVirtualServiceBuilder(vs *v1alpha3.VirtualService) *VirtualServiceBuilder {
	return (*VirtualServiceBuilder)(vs)
}

func NewDestinationRuleBuilder(dr *v1alpha3.DestinationRule) *DestinationRuleBuilder {
	return (*DestinationRuleBuilder)(dr)
}

func NewDestinationRule(host, name, namespace string) *DestinationRuleBuilder {
	dr := &v1alpha3.DestinationRule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha3.SchemeGroupVersion.String(),
			Kind:       "DestinationRule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      host + IstioRuleSuffix,
			Namespace: namespace,
			Labels: map[string]string{
				ExperimentLabel: name + "." + namespace,
				ExperimentHost:  host,
			},
		},
		Spec: networkingv1alpha3.DestinationRule{
			Host: host,
		},
	}

	return (*DestinationRuleBuilder)(dr)
}

func (b *DestinationRuleBuilder) WithDeployment(d *appsv1.Deployment, name string) *DestinationRuleBuilder {
	b.ObjectMeta.Labels[ExperimentRole] = Stable
	b.Spec.Subsets = append(b.Spec.Subsets, &networkingv1alpha3.Subset{
		Name:   name,
		Labels: d.Spec.Template.Labels,
	})

	return b
}

func (b *DestinationRuleBuilder) WithStableLabel() *DestinationRuleBuilder {
	b.ObjectMeta.Labels[ExperimentRole] = Stable
	return b
}

func (b *DestinationRuleBuilder) WithInitLabel() *DestinationRuleBuilder {
	b.ObjectMeta.Labels[ExperimentInit] = "True"
	return b
}

func (b *DestinationRuleBuilder) RemoveExperimentLabel() *DestinationRuleBuilder {
	if _, ok := b.ObjectMeta.Labels[ExperimentLabel]; ok {
		delete(b.ObjectMeta.Labels, ExperimentLabel)
	}

	if _, ok := b.ObjectMeta.Labels[ExperimentInit]; ok {
		delete(b.ObjectMeta.Labels, ExperimentInit)
	}
	return b
}

func (b *DestinationRuleBuilder) WithExperimentRegistered(exp string) *DestinationRuleBuilder {
	b.ObjectMeta.Labels[ExperimentLabel] = exp
	return b
}

func (b *DestinationRuleBuilder) ToStable(stableSubset string) *DestinationRuleBuilder {
	b = b.WithStableLabel()

	for _, subset := range b.Spec.Subsets {
		if subset.Name == stableSubset {
			b.Spec.Subsets[0] = subset
			// Remove old entries
			b.Spec.Subsets = b.Spec.Subsets[:1]
			break
		}
	}

	return b
}

func (b *DestinationRuleBuilder) InitSubsets() *DestinationRuleBuilder {
	b.Spec.Subsets = []*networkingv1alpha3.Subset{}
	return b
}

// WithSubset adds subset to the rule
func (b *DestinationRuleBuilder) WithSubset(name string, d *appsv1.Deployment) *DestinationRuleBuilder {
	// Add new subset to the slice
	b.Spec.Subsets = append(b.Spec.Subsets, &networkingv1alpha3.Subset{
		Name:   name,
		Labels: d.Spec.Template.Labels,
	})
	return b
}

func (b *DestinationRuleBuilder) WithProgressingLabel() *DestinationRuleBuilder {
	b.ObjectMeta.Labels[ExperimentRole] = Progressing
	return b
}

func (b *DestinationRuleBuilder) WithInitializingLabel() *DestinationRuleBuilder {
	b.ObjectMeta.Labels[ExperimentRole] = Initializing
	return b
}

func (b *DestinationRuleBuilder) WithName(name string) *DestinationRuleBuilder {
	b.ObjectMeta.Name = name + IstioRuleSuffix
	return b
}

func (b *DestinationRuleBuilder) Build() *v1alpha3.DestinationRule {
	return (*v1alpha3.DestinationRule)(b)
}

func NewVirtualService(host, name, namespace string) *VirtualServiceBuilder {
	vs := &v1alpha3.VirtualService{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha3.SchemeGroupVersion.String(),
			Kind:       "VirtualService",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      host + IstioRuleSuffix,
			Namespace: namespace,
			Labels: map[string]string{
				ExperimentLabel: name + "." + namespace,
				ExperimentHost:  host,
			},
		},
	}

	return (*VirtualServiceBuilder)(vs)
}

func (b *VirtualServiceBuilder) WithProgressingLabel() *VirtualServiceBuilder {
	b.ObjectMeta.Labels[ExperimentRole] = Progressing
	return b
}

func (b *VirtualServiceBuilder) WithInitializingLabel() *VirtualServiceBuilder {
	b.ObjectMeta.Labels[ExperimentRole] = Initializing
	return b
}

func (b *VirtualServiceBuilder) WithStableLabel() *VirtualServiceBuilder {
	b.ObjectMeta.Labels[ExperimentRole] = Stable
	return b
}

func (b *VirtualServiceBuilder) RemoveExperimentLabel() *VirtualServiceBuilder {
	if _, ok := b.ObjectMeta.Labels[ExperimentLabel]; ok {
		delete(b.ObjectMeta.Labels, ExperimentLabel)
	}

	if _, ok := b.ObjectMeta.Labels[ExperimentInit]; ok {
		delete(b.ObjectMeta.Labels, ExperimentInit)
	}
	return b
}

func (b *VirtualServiceBuilder) WithRolloutPercent(rolloutFilter *networkingv1alpha3.HTTPRouteDestination, rolloutPercent int32) *VirtualServiceBuilder {
	if b.Spec.Http != nil || len(b.Spec.Http) > 0 {
		http := b.Spec.Http[0]
		for _, route := range http.Route {
			if routeFilter(route, rolloutFilter) {
				route.Weight = rolloutPercent
			} else {
				route.Weight = 100 - rolloutPercent
			}
		}
	}

	return b
}

func (b *VirtualServiceBuilder) WithNewStableSet(host, subset string) *VirtualServiceBuilder {
	b.ObjectMeta.Labels[ExperimentRole] = Stable
	b.Spec = networkingv1alpha3.VirtualService{
		Hosts: []string{host},
		Http: []*networkingv1alpha3.HTTPRoute{
			{
				Route: []*networkingv1alpha3.HTTPRouteDestination{
					{
						Destination: &networkingv1alpha3.Destination{
							Host:   host,
							Subset: subset,
						},
						Weight: 100,
					},
				},
			},
		},
	}

	return b
}

func (b *VirtualServiceBuilder) WithInitLabel() *VirtualServiceBuilder {
	b.ObjectMeta.Labels[ExperimentInit] = "True"
	return b
}

func (b *VirtualServiceBuilder) WithExternalLabel() *VirtualServiceBuilder {
	b.ObjectMeta.Labels[External] = "True"
	return b
}

func (b *VirtualServiceBuilder) WithExperimentRegistered(exp string) *VirtualServiceBuilder {
	b.ObjectMeta.Labels[ExperimentLabel] = exp
	return b
}

func routeFilter(route, filter *networkingv1alpha3.HTTPRouteDestination) bool {
	if len(filter.Destination.Host) > 0 {
		if route.Destination.Host != filter.Destination.Host {
			return false
		}
	}

	if len(filter.Destination.Subset) > 0 {
		if route.Destination.Subset != filter.Destination.Subset {
			return false
		}
	}

	return true
}

func (b *VirtualServiceBuilder) ToStable(stableFilter *networkingv1alpha3.HTTPRouteDestination) *VirtualServiceBuilder {
	for _, http := range b.Spec.Http {
		for _, route := range http.Route {
			if routeFilter(route, stableFilter) {
				route.Weight = 100
				// Remove other entries
				http.Route = []*networkingv1alpha3.HTTPRouteDestination{route}
				return b
			}
		}
	}
	return b
}

func (b *VirtualServiceBuilder) InitMeshGateway() *VirtualServiceBuilder {
	b.Spec.Gateways = []string{"mesh"}
	return b
}

func (b *VirtualServiceBuilder) InitHosts() *VirtualServiceBuilder {
	b.Spec.Hosts = []string{}
	return b
}

func (b *VirtualServiceBuilder) WithGateways(gws []string) *VirtualServiceBuilder {
	b.Spec.Gateways = append(b.Spec.Gateways, gws...)
	return b
}

func (b *VirtualServiceBuilder) WithHosts(hosts []string) *VirtualServiceBuilder {
	b.Spec.Hosts = append(b.Spec.Hosts, hosts...)
	return b
}

func (b *VirtualServiceBuilder) WithName(name string) *VirtualServiceBuilder {
	b.ObjectMeta.Name = name + IstioRuleSuffix
	return b
}

func (b *VirtualServiceBuilder) InitHTTPRoutes() *VirtualServiceBuilder {
	b.Spec.Http = []*networkingv1alpha3.HTTPRoute{}
	return b
}

func (b *VirtualServiceBuilder) WithHTTPRoute(route *networkingv1alpha3.HTTPRoute) *VirtualServiceBuilder {
	b.Spec.Http = append(b.Spec.Http, route)
	return b
}

func (b *VirtualServiceBuilder) Build() *v1alpha3.VirtualService {
	return (*v1alpha3.VirtualService)(b)
}

func getWeight(subset string, vs *v1alpha3.VirtualService) int32 {
	for _, route := range vs.Spec.Http[0].Route {
		if route.Destination.Subset == subset {
			return route.Weight
		}
	}
	return 0
}

func removeExperimentLabel(objs ...runtime.Object) (err error) {
	for _, obj := range objs {
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return err
		}
		labels := accessor.GetLabels()
		delete(labels, ExperimentLabel)
		if _, ok := labels[ExperimentInit]; ok {
			delete(labels, ExperimentInit)
		}
		accessor.SetLabels(labels)
	}

	return nil
}

type HTTPRouteBuilder networkingv1alpha3.HTTPRoute

func NewEmptyHTTPRoute() *HTTPRouteBuilder {
	return (*HTTPRouteBuilder)(&networkingv1alpha3.HTTPRoute{})
}

func NewHTTPRoute(route *networkingv1alpha3.HTTPRoute) *HTTPRouteBuilder {
	return (*HTTPRouteBuilder)(route)
}

func (b *HTTPRouteBuilder) WithDestination(d *networkingv1alpha3.HTTPRouteDestination) *HTTPRouteBuilder {
	b.Route = append(b.Route, d)
	return b
}

func (b *HTTPRouteBuilder) Build() *networkingv1alpha3.HTTPRoute {
	return (*networkingv1alpha3.HTTPRoute)(b)
}

type HTTPRouteDestinationBuilder networkingv1alpha3.HTTPRouteDestination

func NewHTTPRouteDestination() *HTTPRouteDestinationBuilder {
	return (*HTTPRouteDestinationBuilder)(&networkingv1alpha3.HTTPRouteDestination{
		Destination: &networkingv1alpha3.Destination{},
	})
}

func (b *HTTPRouteDestinationBuilder) WithWeight(w int32) *HTTPRouteDestinationBuilder {
	b.Weight = w
	return b
}

func (b *HTTPRouteDestinationBuilder) WithHost(host string) *HTTPRouteDestinationBuilder {
	b.Destination.Host = host
	return b
}

func (b *HTTPRouteDestinationBuilder) WithSubset(subset string) *HTTPRouteDestinationBuilder {
	b.Destination.Subset = subset
	return b
}

func (b *HTTPRouteDestinationBuilder) WithPort(port uint32) *HTTPRouteDestinationBuilder {
	b.Destination.Port.Number = port
	return b
}

func (b *HTTPRouteDestinationBuilder) Build() *networkingv1alpha3.HTTPRouteDestination {
	return (*networkingv1alpha3.HTTPRouteDestination)(b)
}
