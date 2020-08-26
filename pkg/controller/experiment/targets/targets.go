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

// This file contains functions used for getting/removing runtime objects of target service specified
// in an iter8 experiment.

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iter8v1alpha2 "github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
	"github.com/iter8-tools/iter8/pkg/controller/experiment/util"
)

// Role of target
type Role string

const (
	RoleService   = Role("service")
	RoleBaseline  = Role("baseline")
	RoleCandidate = Role("candidate")
)

// Targets contains substantiated runtime objects of internal service, baseline and candidates
// that are specified inside an experiment cr; and also supplementray objects to help fulfill the getter functions
type Targets struct {
	Service    *corev1.Service
	Baseline   runtime.Object
	Candidates []runtime.Object

	service   iter8v1alpha2.Service
	namespace string
	client    client.Client
}

// Init initialize a Targets object with k8s client and namespace of the target service
func Init(instance *iter8v1alpha2.Experiment, client client.Client) *Targets {
	return &Targets{
		client:    client,
		namespace: instance.ServiceNamespace(),
		service:   instance.Spec.Service,
	}
}

// GetService substantializes internal service in targets
// returns non-nil error if there is problem in getting the runtime object from cluster
func (t *Targets) GetService(context context.Context) error {
	if t.service.Name == "" {
		return nil
	}

	t.Service = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.service.Name,
			Namespace: t.namespace,
		},
	}

	return getObject(context, t.client, t.Service)
}

// GetBaseline substantializes baseline in the targets
// returns non-nil error if there is problem in getting the runtime object from cluster
func (t *Targets) GetBaseline(context context.Context) error {
	t.Baseline = getRuntimeObject(metav1.ObjectMeta{
		Name:      t.service.Baseline,
		Namespace: t.namespace,
	}, t.service.Kind)

	return getObject(context, t.client, t.Baseline)
}

// GetCandidates substantializes all candidates in the targets
// returns non-nil error if there is problem in getting the runtime objects from cluster
func (t *Targets) GetCandidates(context context.Context) (err error) {
	t.Candidates = make([]runtime.Object, len(t.service.Candidates))

	for i := range t.Candidates {
		t.Candidates[i] = getRuntimeObject(metav1.ObjectMeta{
			Name:      t.service.Candidates[i],
			Namespace: t.namespace,
		}, t.service.Kind)

		err = getObject(context, t.client, t.Candidates[i])
		if err != nil {
			return
		}
	}
	return
}

// Cleanup deletes cluster runtime objects of targets at the end of experiment
func Cleanup(context context.Context, instance *iter8v1alpha2.Experiment, client client.Client) {
	if instance.Spec.GetCleanup() {
		assessment := instance.Status.Assessment
		toKeep := make(map[string]bool)

		switch instance.Spec.GetOnTermination() {
		case iter8v1alpha2.OnTerminationToWinner:
			if instance.Status.IsWinnerFound() {
				toKeep[*assessment.Winner.Name] = true
				break
			}
			fallthrough
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

		kind := instance.Spec.Service.Kind
		svcNamespace := instance.ServiceNamespace()

		// delete baseline if not receiving traffic
		if ok := toKeep[instance.Spec.Baseline]; !ok {
			err := client.Delete(context, getRuntimeObject(metav1.ObjectMeta{
				Namespace: svcNamespace,
				Name:      instance.Spec.Baseline,
			}, kind))
			if err != nil {
				util.Logger(context).Error(err, "Error when deleting baseline")
			}
		}

		// delete candidates that are not receiving traffic
		for _, candidate := range instance.Spec.Candidates {
			if ok := toKeep[candidate]; !ok {
				err := client.Delete(context, getRuntimeObject(metav1.ObjectMeta{
					Namespace: svcNamespace,
					Name:      candidate,
				}, kind))
				if err != nil {
					util.Logger(context).Error(err, "Error when deleting candidate", "name", candidate)
				}
			}
		}
	}
}

// Instantiate runtime object content from k8s cluster
func getObject(ctx context.Context, c client.Client, obj runtime.Object) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	return c.Get(ctx, types.NamespacedName{
		Name:      accessor.GetName(),
		Namespace: accessor.GetNamespace()},
		obj)
}

// Form runtime object with meta info and kind specified
func getRuntimeObject(om metav1.ObjectMeta, kind string) runtime.Object {
	switch kind {
	case "Service":
		return &corev1.Service{
			ObjectMeta: om,
		}
	default:
		// Deployment
		return &appsv1.Deployment{
			ObjectMeta: om,
		}
	}
}
