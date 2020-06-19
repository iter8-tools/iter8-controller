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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/util"
)

const (
	DefaultKind = "Deployment"
)

type Role string

const (
	RoleService   Role = "service"
	RoleBaseline  Role = "baseline"
	RoleCandidate Role = "candidate"
)

type Targets struct {
	Service   *corev1.Service
	Baseline  runtime.Object
	Candidate runtime.Object
	Port      *int32
	Hosts     []string
	Gateways  []string

	client client.Client
}

// InitTargets initializes object instances with
func InitTargets(instance *iter8v1alpha1.Experiment, client client.Client) *Targets {
	out := &Targets{
		client: client,
	}
	ts := instance.Spec.TargetService
	svcNamespace := instance.ServiceNamespace()
	if ts.Name != "" {
		out.Service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ts.Name,
				Namespace: svcNamespace,
			},
		}
	}

	mHosts, mGateways := make(map[string]bool), make(map[string]bool)
	for _, host := range ts.Hosts {
		if _, ok := mHosts[host.Name]; !ok {
			out.Hosts = append(out.Hosts, host.Name)
			mHosts[host.Name] = true
		}

		if _, ok := mHosts[host.Gateway]; !ok {
			out.Gateways = append(out.Gateways, host.Gateway)
			mGateways[host.Gateway] = true
		}
	}

	out.Port = ts.Port

	kind := DefaultKind
	if instance.Spec.TargetService.Kind != "" {
		kind = instance.Spec.TargetService.Kind
	}

	switch kind {
	case "Deployment":
		out.Baseline = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instance.Spec.TargetService.Baseline,
				Namespace: svcNamespace,
			},
		}
		out.Candidate = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instance.Spec.TargetService.Candidate,
				Namespace: svcNamespace,
			},
		}
	case "Service":
		out.Baseline = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instance.Spec.TargetService.Baseline,
				Namespace: svcNamespace,
			},
		}
		out.Candidate = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instance.Spec.TargetService.Candidate,
				Namespace: svcNamespace,
			},
		}
	}

	return out
}

// ValidateSpec validates whether targetService is valid or not
func ValidateSpec(instance *iter8v1alpha1.Experiment) error {
	ts := instance.Spec.TargetService

	// check service/hosts specification
	if ts.Name == "" && len(ts.Hosts) == 0 {
		return fmt.Errorf("Either Name or Hosts should be specified in targetService")
	}

	// check kind/apiVersion specification
	switch ts.Kind {
	case "Deployment", "":
		if !(ts.APIVersion == "" || ts.APIVersion == "apps/v1" || ts.APIVersion == "v1") {
			return fmt.Errorf("Invalid kind/apiVerison pair: %s, %s", ts.Kind, ts.APIVersion)
		}
		instance.Spec.TargetService.Kind = "Deployment"
	case "Service":
		if !(ts.APIVersion == "" || ts.APIVersion == "v1") {
			return fmt.Errorf("Invalid kind/apiVerison pair: %s, %s", ts.Kind, ts.APIVersion)
		}
	default:
		return fmt.Errorf("Invalid kind/apiVerison pair: %s, %s", ts.Kind, ts.APIVersion)
	}

	return nil
}

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

// GetService substantializes service in the targets
// returns non-nil error if there is problem in getting the runtime object from cluster
func (t *Targets) GetService(context context.Context) error {
	if t.Service == nil {
		return nil
	}

	return getObject(context, t.client, t.Service)
}

// GetBaseline substantializes baseline in the targets
// returns non-nil error if there is problem in getting the runtime object from cluster
func (t *Targets) GetBaseline(context context.Context) error {
	return getObject(context, t.client, t.Baseline)
}

// GetCandidate substantializes candidate in the targets
// returns non-nil error if there is problem in getting the runtime object from cluster
func (t *Targets) GetCandidate(context context.Context) error {
	return getObject(context, t.client, t.Candidate)
}

// Cleanup removes target instances that will not receive traffic after experiment ended if necessary
func Cleanup(context context.Context, instance *iter8v1alpha1.Experiment, client client.Client) {
	if instance.Spec.CleanUp == iter8v1alpha1.CleanUpDelete {
		var toDelete runtime.Object

		success := instance.Succeeded()
		onSuccess := instance.Spec.TrafficControl.GetOnSuccess()
		if !success || onSuccess == "baseline" {
			switch instance.Spec.TargetService.Kind {
			case "Service":
				toDelete = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      instance.Spec.TargetService.Candidate,
						Namespace: instance.ServiceNamespace(),
					},
				}
			case "Deployment", "":
				toDelete = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      instance.Spec.TargetService.Candidate,
						Namespace: instance.ServiceNamespace(),
					},
				}
			}
			// delete candidate
			if err := client.Delete(context, toDelete); err != nil && errors.IsNotFound(err) {
				util.Logger(context).Error(err, "Delete Candidate", "")
			}
			instance.Status.TrafficSplit.Baseline = 100
			instance.Status.TrafficSplit.Candidate = 0
		} else if onSuccess == "candidate" {
			// delete baseline
			switch instance.Spec.TargetService.Kind {
			case "Service":
				toDelete = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      instance.Spec.TargetService.Baseline,
						Namespace: instance.ServiceNamespace(),
					},
				}
			case "Deployment", "":
				toDelete = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      instance.Spec.TargetService.Baseline,
						Namespace: instance.ServiceNamespace(),
					},
				}
			}
			if err := client.Delete(context, toDelete); err != nil && errors.IsNotFound(err) {
				util.Logger(context).Error(err, "Delete Baseline", "")
			}
			instance.Status.TrafficSplit.Baseline = 0
			instance.Status.TrafficSplit.Candidate = 100
		}
	}
}
