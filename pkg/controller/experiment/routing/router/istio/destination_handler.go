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
	iter8v1alpha2 "github.com/iter8-tools/iter8-istio/pkg/apis/iter8/v1alpha2"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
)

type destinationOptions struct {
	// name of destination
	name   string
	subset string
	weight int32
	port   *int32
}

// destinationHandler interface defines functions that a destination handler should implement
type destinationHandler interface {
	// Validate detected routing lists from cluster to see if they are valid for next experiment
	// Init routing rules after if validation passes
	validateAndInit(drl *v1alpha3.DestinationRuleList, vsl *v1alpha3.VirtualServiceList, instance *iter8v1alpha2.Experiment) (*istioRoutingRules, error)

	// build destination content based on target info and experiment info
	buildDestination(instance *iter8v1alpha2.Experiment, opts destinationOptions) *networkingv1alpha3.HTTPRouteDestination

	// whether DestiantionRule is required by the handler
	requireDestinationRule() bool
}
