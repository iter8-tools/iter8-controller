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
	"fmt"

	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"

	iter8v1alpha2 "github.com/iter8-tools/iter8-istio/pkg/apis/iter8/v1alpha2"
	"github.com/iter8-tools/iter8-istio/pkg/controller/experiment/util"
)

var _ destinationHandler = deploymentHandler{}

type deploymentHandler struct{}

func (h deploymentHandler) validateAndInit(drl *v1alpha3.DestinationRuleList, vsl *v1alpha3.VirtualServiceList, instance *iter8v1alpha2.Experiment) (*istioRoutingRules, error) {
	out := &istioRoutingRules{}

	svcNamespace := instance.ServiceNamespace()
	expFullName := util.FullExperimentName(instance)
	if len(drl.Items) == 0 && len(vsl.Items) == 0 {
		// init rule
		name := GetRoutingRuleName(getRouterID(instance))
		host := util.GetDefaultHost(instance)
		out.destinationRule = NewDestinationRule(name, host, expFullName, svcNamespace).
			WithInitLabel().
			Build()
		out.virtualService = NewVirtualService(name, expFullName, svcNamespace).
			WithInitLabel().
			Build()
	} else if len(drl.Items) == 1 && len(vsl.Items) == 1 {
		drrole, drok := drl.Items[0].GetLabels()[experimentRole]
		vsrole, vsok := vsl.Items[0].GetLabels()[experimentRole]
		if drok && vsok {
			if drrole == roleStable && vsrole == roleStable {
				// Valid stable rules detected
				out.destinationRule = drl.Items[0].DeepCopy()
				out.virtualService = vsl.Items[0].DeepCopy()
			} else {
				drLabel, drok := drl.Items[0].GetLabels()[experimentLabel]
				vsLabel, vsok := vsl.Items[0].GetLabels()[experimentLabel]
				if drok && vsok {
					if drLabel == expFullName && vsLabel == expFullName {
						// valid progressing rules found
						out.destinationRule = drl.Items[0].DeepCopy()
						out.virtualService = vsl.Items[0].DeepCopy()
					} else {
						return nil, fmt.Errorf("Progressing rules being involved in other experiments")
					}
				} else {
					return nil, fmt.Errorf("Experiment label missing in dr or vs")
				}
			}
		} else {
			return nil, fmt.Errorf("experiment role label missing in dr or vs")
		}
	} else if len(drl.Items) == 0 && len(vsl.Items) == 1 {
		vsrole, vsok := vsl.Items[0].GetLabels()[experimentRole]
		if vsok && vsrole == roleStable {
			// Valid stable rules detected
			name := GetRoutingRuleName(getRouterID(instance))
			out.virtualService = vsl.Items[0].DeepCopy()
			out.destinationRule = NewDestinationRule(name, util.GetDefaultHost(instance), expFullName, svcNamespace).
				WithInitLabel().
				Build()
		} else {
			return nil, fmt.Errorf("0 dr and 1 unstable vs found")
		}
	} else {
		return nil, fmt.Errorf("%d dr and %d vs detected", len(drl.Items), len(vsl.Items))
	}

	return out, nil
}

// build destination content based on runtime target object and experiment info
func (h deploymentHandler) buildDestination(instance *iter8v1alpha2.Experiment, opts destinationOptions) *networkingv1alpha3.HTTPRouteDestination {
	b := NewHTTPRouteDestination().
		WithHost(util.ServiceToFullHostName(instance.Spec.Service.Name, instance.ServiceNamespace())).
		WithSubset(opts.subset).
		WithWeight(opts.weight)

	if opts.port != nil {
		b = b.WithPort(uint32(*opts.port))
	}

	return b.Build()
}

// whether DestiantionRule is required by the handler
func (h deploymentHandler) requireDestinationRule() bool {
	return true
}
