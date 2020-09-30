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

package router

import (
	"context"

	runtime "k8s.io/apimachinery/pkg/runtime"

	iter8v1alpha2 "github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
)

// Interface declares functions to be implemented so as to be used by iter8 router
type Interface interface {
	// Fetch gets routing rules from cluster
	Fetch(ctx context.Context, instance *iter8v1alpha2.Experiment) error
	// UpdateRouteWithBaseline updates routing rules with runtime object of baseline
	UpdateRouteWithBaseline(ctx context.Context, instance *iter8v1alpha2.Experiment, baseline runtime.Object) error
	// UpdateRouteWithCandidates updates routing rules with runtime objects of candidates
	UpdateRouteWithCandidates(ctx context.Context, instance *iter8v1alpha2.Experiment, candidates []runtime.Object) error
	// UpdateRouteWithTrafficUpdate updates routing rules with new traffic state from assessment
	UpdateRouteWithTrafficUpdate(ctx context.Context, instance *iter8v1alpha2.Experiment) error
	// UpdateRouteToStable updates routing rules to desired stable state
	UpdateRouteToStable(ctx context.Context, instance *iter8v1alpha2.Experiment) error
	// Print prints detailed information about the router
	Print() string
}
