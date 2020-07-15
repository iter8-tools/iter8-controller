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

	iter8v1alpha2 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha2"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/util"
)

const (
	IstioRuleSuffix = ".iter8-experiment"
)

type DestinationRuleBuilder v1alpha3.DestinationRule
type VirtualServiceBuilder v1alpha3.VirtualService

func NewVirtualServiceBuilder(vs *v1alpha3.VirtualService) *VirtualServiceBuilder {
	return (*VirtualServiceBuilder)(vs)
}

func NewDestinationRuleBuilder(dr *v1alpha3.DestinationRule) *DestinationRuleBuilder {
	return (*DestinationRuleBuilder)(dr)
}

func NewDestinationRule(serviceName, name, namespace string) *DestinationRuleBuilder {
	dr := &v1alpha3.DestinationRule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha3.SchemeGroupVersion.String(),
			Kind:       "DestinationRule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName + "." + namespace + IstioRuleSuffix,
			Namespace: namespace,
			Labels: map[string]string{
				ExperimentLabel: name,
				ExperimentHost:  serviceName,
			},
		},
		Spec: networkingv1alpha3.DestinationRule{
			Host:    serviceName,
			Subsets: []*networkingv1alpha3.Subset{},
		},
	}

	return (*DestinationRuleBuilder)(dr)
}

func (b *DestinationRuleBuilder) WithStableLabel() *DestinationRuleBuilder {
	b.ObjectMeta.Labels[ExperimentRole] = RoleStable
	return b
}

func (b *DestinationRuleBuilder) WithProgressingLabel() *DestinationRuleBuilder {
	b.ObjectMeta.Labels[ExperimentRole] = RoleProgressing
	return b
}

func (b *DestinationRuleBuilder) WithInitLabel() *DestinationRuleBuilder {
	b.ObjectMeta.Labels[ExperimentInit] = "True"
	if _, ok := b.ObjectMeta.Labels[ExperimentRole]; !ok {
		b.ObjectMeta.Labels[ExperimentRole] = RoleInitializing
	}
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

func (b *DestinationRuleBuilder) WithHostRegistered(host string) *DestinationRuleBuilder {
	b.ObjectMeta.Labels[ExperimentHost] = host
	return b
}

func (b *DestinationRuleBuilder) InitSubsets(count int) *DestinationRuleBuilder {
	b.Spec.Subsets = make([]*networkingv1alpha3.Subset, count)
	return b
}

// WithSubset converts stable dr to progressing dr
func (b *DestinationRuleBuilder) WithSubset(d *appsv1.Deployment, subsetName string, idx int) *DestinationRuleBuilder {
	for idx >= len(b.Spec.Subsets) {
		b.Spec.Subsets = append(b.Spec.Subsets, &networkingv1alpha3.Subset{})
	}
	b.Spec.Subsets[idx] = &networkingv1alpha3.Subset{
		Name:   subsetName,
		Labels: d.Spec.Template.ObjectMeta.Labels,
	}

	return b
}

func (b *DestinationRuleBuilder) ProgressingToStable(stableSubsets map[string]string) *DestinationRuleBuilder {
	cnt := 0
	for i := 0; i < len(b.Spec.Subsets); i++ {
		if stableName, ok := stableSubsets[b.Spec.Subsets[i].Name]; ok {
			subset := b.Spec.Subsets[i]
			subset.Name = stableName
			b.Spec.Subsets[cnt] = subset
			cnt++
		}
	}

	b.Spec.Subsets = b.Spec.Subsets[:cnt]
	return b
}

func (b *DestinationRuleBuilder) WithName(name string) *DestinationRuleBuilder {
	b.ObjectMeta.Name = name + IstioRuleSuffix
	return b
}

func (b *DestinationRuleBuilder) Build() *v1alpha3.DestinationRule {
	return (*v1alpha3.DestinationRule)(b)
}

func NewVirtualService(serviceName, name, namespace string) *VirtualServiceBuilder {
	vs := &v1alpha3.VirtualService{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha3.SchemeGroupVersion.String(),
			Kind:       "VirtualService",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName + "." + namespace + IstioRuleSuffix,
			Namespace: namespace,
			Labels: map[string]string{
				ExperimentLabel: name,
				ExperimentHost:  serviceName,
			},
		},
		Spec: networkingv1alpha3.VirtualService{
			Hosts: []string{serviceName},
		},
	}

	return (*VirtualServiceBuilder)(vs)
}

func (b *VirtualServiceBuilder) WithInitLabel() *VirtualServiceBuilder {
	b.ObjectMeta.Labels[ExperimentInit] = "True"
	if _, ok := b.ObjectMeta.Labels[ExperimentRole]; !ok {
		b.ObjectMeta.Labels[ExperimentRole] = RoleInitializing
	}
	return b
}

func (b *VirtualServiceBuilder) WithProgressingLabel() *VirtualServiceBuilder {
	b.ObjectMeta.Labels[ExperimentRole] = RoleProgressing
	return b
}

func (b *VirtualServiceBuilder) WithStableLabel() *VirtualServiceBuilder {
	b.ObjectMeta.Labels[ExperimentRole] = RoleStable
	return b
}

func (b *VirtualServiceBuilder) WithExperimentRegistered(exp string) *VirtualServiceBuilder {
	b.ObjectMeta.Labels[ExperimentLabel] = exp
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

// WithTrafficSplit will update http route with specified traffic split
func (b *VirtualServiceBuilder) WithTrafficSplit(host string, trafficSplit map[string]int32) *VirtualServiceBuilder {
	b.Spec.Http = make([]*networkingv1alpha3.HTTPRoute, 0)
	for name, traffic := range trafficSplit {
		b.Spec.Http = append(b.Spec.Http, &networkingv1alpha3.HTTPRoute{
			Route: []*networkingv1alpha3.HTTPRouteDestination{
				{
					Destination: &networkingv1alpha3.Destination{
						Host:   host,
						Subset: name,
					},
					Weight: traffic,
				},
			},
		})
	}

	return b
}

func (b *VirtualServiceBuilder) WithHTTPMatch(httpMatch []*iter8v1alpha2.HTTPMatchRequest) *VirtualServiceBuilder {
	if b.Spec.Http == nil || len(b.Spec.Http) == 0 {
		b.Spec.Http = append(b.Spec.Http, &networkingv1alpha3.HTTPRoute{})
	}
	for _, match := range httpMatch {
		b.Spec.Http[0].Match = append(b.Spec.Http[0].Match, convertMatchToIstio(match))
	}
	return b
}

// ExternalToProgressing mark external reference vs as progressing mode
func (b *VirtualServiceBuilder) ExternalToProgressing(service, ns string, candidateCount int) *VirtualServiceBuilder {
	for _, http := range b.Spec.Http {
		match := false
		var port *networkingv1alpha3.PortSelector
		for _, route := range http.Route {
			if util.EqualHost(route.Destination.Host, ns, service, ns) {
				port = route.Destination.Port
				match = true
				break
			}
		}
		if match {
			http.Route = make([]*networkingv1alpha3.HTTPRouteDestination, candidateCount+1)
			http.Route[0] = &networkingv1alpha3.HTTPRouteDestination{
				Destination: &networkingv1alpha3.Destination{
					Host:   service,
					Subset: SubsetBaseline,
					Port:   port,
				},
				Weight: 100,
			}

			for i := 0; i < candidateCount; i++ {
				http.Route[i+1] = &networkingv1alpha3.HTTPRouteDestination{
					Destination: &networkingv1alpha3.Destination{
						Host:   service,
						Subset: candiateSubsetName(i),
						Port:   port,
					},
					Weight: 0,
				}
			}
			break
		}
	}
	return b
}

func (b *VirtualServiceBuilder) ToProgressing(service string, candidateCount int) *VirtualServiceBuilder {
	if b.Spec.Http == nil || len(b.Spec.Http) == 0 {
		b.Spec.Http = append(b.Spec.Http, &networkingv1alpha3.HTTPRoute{})
	}

	b.Spec.Http[0].Route = make([]*networkingv1alpha3.HTTPRouteDestination, candidateCount+1)
	b.Spec.Http[0].Route[0] = &networkingv1alpha3.HTTPRouteDestination{
		Destination: &networkingv1alpha3.Destination{
			Host:   service,
			Subset: SubsetBaseline,
		},
		Weight: 100,
	}

	for i := 0; i < candidateCount; i++ {
		b.Spec.Http[0].Route[i+1] = &networkingv1alpha3.HTTPRouteDestination{
			Destination: &networkingv1alpha3.Destination{
				Host:   service,
				Subset: candiateSubsetName(i),
			},
			Weight: 0,
		}
	}

	return b
}

func (b *VirtualServiceBuilder) ProgressingToStable(weight map[string]int32, service, ns string) *VirtualServiceBuilder {
	for _, http := range b.Spec.Http {
		match := false
		var port *networkingv1alpha3.PortSelector
		for _, route := range http.Route {
			if util.EqualHost(route.Destination.Host, ns, service, ns) {
				port = route.Destination.Port
				match = true
				break
			}
		}
		if match {
			http.Route = make([]*networkingv1alpha3.HTTPRouteDestination, len(weight))
			i := 0
			for name, w := range weight {
				http.Route[i] = &networkingv1alpha3.HTTPRouteDestination{
					Destination: &networkingv1alpha3.Destination{
						Host:   service,
						Subset: name,
						Port:   port,
					},
					Weight: w,
				}
				i++
			}
			break
		}
	}
	return b
}

func (b *VirtualServiceBuilder) WithHostRegistered(host string) *VirtualServiceBuilder {
	b.ObjectMeta.Labels[ExperimentHost] = host
	return b
}

func (b *VirtualServiceBuilder) WithExternalLabel() *VirtualServiceBuilder {
	b.ObjectMeta.Labels[ExternalReference] = "True"
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

func convertMatchToIstio(m *iter8v1alpha2.HTTPMatchRequest) *networkingv1alpha3.HTTPMatchRequest {
	out := &networkingv1alpha3.HTTPMatchRequest{
		Name:          m.Name,
		Port:          m.Port,
		IgnoreUriCase: m.IgnoreURICase,
	}

	if m.URI != nil && m.URI.IsValid() {
		out.Uri = toStringMatchExact(m.URI)
	}

	return out
}

func toStringMatchExact(s *iter8v1alpha2.StringMatch) *networkingv1alpha3.StringMatch {
	if s.Exact != nil {
		return &networkingv1alpha3.StringMatch{
			MatchType: &networkingv1alpha3.StringMatch_Exact{
				Exact: *(s.Exact)},
		}
	}
	if s.Prefix != nil {
		return &networkingv1alpha3.StringMatch{
			MatchType: &networkingv1alpha3.StringMatch_Prefix{
				Prefix: *(s.Prefix)},
		}
	}

	return &networkingv1alpha3.StringMatch{
		MatchType: &networkingv1alpha3.StringMatch_Regex{
			Regex: *(s.Regex)},
	}
}
