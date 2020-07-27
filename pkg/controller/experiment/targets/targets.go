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

package targets

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iter8v1alpha2 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha2"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/util"
)

type Role string

const (
	RoleService   Role = "service"
	RoleBaseline  Role = "baseline"
	RoleCandidate Role = "candidate"
)

// Targets contains details for target service of an experiment
type Targets struct {
	Service    *corev1.Service
	Baseline   *appsv1.Deployment
	Candidates []*appsv1.Deployment
	Port       *int32
	Hosts      []string
	Gateways   []string

	namespace string
	client    client.Client
}

// Init returns an initialized targets content for an expeirment
func Init(instance *iter8v1alpha2.Experiment, client client.Client) *Targets {
	out := &Targets{
		Service:    &corev1.Service{},
		Baseline:   &appsv1.Deployment{},
		Candidates: make([]*appsv1.Deployment, len(instance.Spec.Candidates)),
		namespace:  instance.ServiceNamespace(),
		client:     client,
	}

	mHosts, mGateways := make(map[string]bool), make(map[string]bool)
	service := instance.Spec.Service
	for _, host := range service.Hosts {
		if _, ok := mHosts[host.Name]; !ok {
			out.Hosts = append(out.Hosts, host.Name)
			mHosts[host.Name] = true
		}

		if _, ok := mHosts[host.Gateway]; !ok {
			out.Gateways = append(out.Gateways, host.Gateway)
			mGateways[host.Gateway] = true
		}
	}

	out.Port = instance.Spec.Service.Port
	return out
}

// GetService substantializes service in the targets
// returns non-nil error if there is problem in getting the runtime object from cluster
func (t *Targets) GetService(context context.Context, instance *iter8v1alpha2.Experiment) error {
	return t.client.Get(context, types.NamespacedName{
		Name:      instance.Spec.Service.Name,
		Namespace: t.namespace},
		t.Service)
}

// GetBaseline substantializes baseline in the targets
// returns non-nil error if there is problem in getting the runtime object from cluster
func (t *Targets) GetBaseline(context context.Context, instance *iter8v1alpha2.Experiment) error {
	return t.client.Get(context, types.NamespacedName{
		Name:      instance.Spec.Baseline,
		Namespace: t.namespace},
		t.Baseline)
}

// GetCandidates substantializes all candidates in the targets
// returns non-nil error if there is problem in getting the runtime objects from cluster
func (t *Targets) GetCandidates(context context.Context, instance *iter8v1alpha2.Experiment) (err error) {
	if len(t.Candidates) != len(instance.Spec.Candidates) {
		return fmt.Errorf("Mismatch of candidate list length, %d in targets while %d in instance",
			len(instance.Spec.Candidates), len(t.Candidates))
	}
	for i := range t.Candidates {
		t.Candidates[i] = &appsv1.Deployment{}
		err = t.client.Get(context, types.NamespacedName{
			Name:      instance.Spec.Candidates[i],
			Namespace: t.namespace},
			t.Candidates[i])
		if err != nil {
			return
		}
	}
	return
}

// Cleanup deletes cluster runtime objects of targets at the end of experiment
func (t *Targets) Cleanup(context context.Context, instance *iter8v1alpha2.Experiment) {
	if instance.Spec.Cleanup != nil && *instance.Spec.Cleanup {
		assessment := instance.Status.Assessment
		toKeep := make(map[string]bool)
		switch instance.Spec.GetOnTermination() {
		case iter8v1alpha2.OnTerminationToWinner:
			if instance.Status.IsWinnerFound() {
				toKeep[assessment.Winner.Winner] = true
			} else {
				toKeep[instance.Spec.Baseline] = true
			}
		case iter8v1alpha2.OnTerminationToBaseline:
			toKeep[instance.Spec.Baseline] = true
		case iter8v1alpha2.OnTerminationKeepLast:
			if assessment != nil {
				if assessment.Baseline.Weight > 0 {
					toKeep[assessment.Baseline.Name] = true
				}
				for _, candidate := range assessment.Candidates {
					if candidate.Weight > 0 {
						toKeep[candidate.Name] = true
					}
				}
			}
		}

		svcNamespace := instance.ServiceNamespace()
		// delete baseline
		if ok := toKeep[instance.Spec.Baseline]; !ok {
			err := t.client.Delete(context, &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: svcNamespace,
					Name:      instance.Spec.Baseline,
				},
			})
			if err != nil {
				util.Logger(context).Error(err, "Error when deleting baseline")
			}
		}

		// delete candidates
		for _, candidate := range instance.Spec.Candidates {
			if ok := toKeep[candidate]; !ok {
				err := t.client.Delete(context, &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: svcNamespace,
						Name:      candidate,
					},
				})
				if err != nil {
					util.Logger(context).Error(err, "Error when deleting candidate", "name", candidate)
				}
			}
		}
	}
}
