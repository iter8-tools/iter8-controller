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

	iter8v1alpha2 "github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
	"github.com/iter8-tools/iter8/pkg/controller/experiment/util"
)

var _ destinationHandler = serviceHandler{}

type serviceHandler struct {
}

func (h serviceHandler) validateAndInit(drl *v1alpha3.DestinationRuleList, vsl *v1alpha3.VirtualServiceList, instance *iter8v1alpha2.Experiment) (*istioRoutingRules, error) {
	out := &istioRoutingRules{}
	if len(vsl.Items) == 0 {
		// init rules
		out.virtualService = NewVirtualService(genereateRuleUUID(), util.FullExperimentName(instance), instance.ServiceNamespace()).
			WithInitLabel().
			Build()

	} else if len(vsl.Items) == 1 {
		vsrole, vsok := vsl.Items[0].GetLabels()[experimentRole]
		if vsok {
			if vsrole == roleStable {
				// Valid stable rules detected
				out.virtualService = vsl.Items[0].DeepCopy()
			} else {
				vsLabel, vsok := vsl.Items[0].GetLabels()[experimentLabel]
				if vsok {
					expName := util.FullExperimentName(instance)
					if vsLabel == expName {
						// valid progressing rules found
						out.virtualService = vsl.Items[0].DeepCopy()
					} else {
						return nil, fmt.Errorf("Progressing rules of other experiment are detected")
					}
				} else {
					return nil, fmt.Errorf("Experiment label missing in dr or vs")
				}
			}
		} else {
			return nil, fmt.Errorf("experiment role label missing in vs")
		}
	} else {
		return nil, fmt.Errorf("%d vs detected", len(vsl.Items))
	}
	return out, nil
}

// build destination content based on runtime target object and experiment info
func (h serviceHandler) buildDestination(instance *iter8v1alpha2.Experiment, opts destinationOptions) *networkingv1alpha3.HTTPRouteDestination {
	b := NewHTTPRouteDestination().
		WithHost(util.ServiceToFullHostName(opts.name, instance.ServiceNamespace())).
		WithWeight(opts.weight)

	if opts.port != nil {
		b = b.WithPort(uint32(*opts.port))
	}
	return b.Build()
}

// whether DestiantionRule is required by the handler
func (h serviceHandler) requireDestinationRule() bool {
	return false
}
