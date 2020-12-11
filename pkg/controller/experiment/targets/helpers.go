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
	"fmt"
	"time"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeout  = 15 * time.Second
	interval = 3 * time.Second
)

func waitForServiceReady(ctx context.Context, c client.Client, obj runtime.Object) error {
	return waitForState(ctx, c, obj, func(obj runtime.Object) (bool, error) {
		service, ok := obj.(*corev1.Service)
		if !ok {
			return false, fmt.Errorf("Expected a Kubernetes Service (got: %v)", obj)
		}

		selectors := service.Spec.Selector
		pods := &corev1.PodList{}

		err := c.List(ctx, pods, &client.ListOptions{
			Namespace:     service.Namespace,
			LabelSelector: labels.Set(selectors).AsSelector(),
		})

		if err != nil {
			return false, nil
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				return false, nil
			}

			for _, c := range pod.Status.Conditions {
				if c.Type == corev1.PodReady {
					if c.Status != corev1.ConditionTrue {
						return false, nil
					}
					break
				}
			}
		}

		return true, nil
	})
}

func waitForDeploymentReady(ctx context.Context, c client.Client, obj runtime.Object) error {
	return waitForState(ctx, c, obj, func(obj runtime.Object) (bool, error) {
		deploy, ok := obj.(*appsv1.Deployment)
		if !ok {
			return false, fmt.Errorf("Expected a Kubernetes Deployment (got: %v)", obj)
		}

		available := corev1.ConditionUnknown
		for _, c := range deploy.Status.Conditions {
			if c.Type == appsv1.DeploymentAvailable {
				available = c.Status
				break
			}
		}

		if deploy.Status.AvailableReplicas > 0 &&
			deploy.Status.Replicas == deploy.Status.ReadyReplicas &&
			available == corev1.ConditionTrue {
			return true, nil
		}
		return false, nil
	})
}

// waitForState polls the status of the object called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout
func waitForState(ctx context.Context, cl client.Client, obj runtime.Object, inState func(obj runtime.Object) (bool, error)) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	key := client.ObjectKey{Namespace: accessor.GetNamespace(), Name: accessor.GetName()}

	waitErr := wait.PollImmediate(interval, timeout, func() (bool, error) {
		var err error
		err = cl.Get(ctx, key, obj)
		if err != nil {
			return true, err
		}
		return inState(obj)
	})

	if waitErr != nil {
		return errors.Wrapf(waitErr, "object %q is not in desired state, got: %+v", accessor.GetName(), obj)
	}
	return nil
}

// Instantiate runtime object content from k8s cluster
func getObject(ctx context.Context, c client.Client, obj runtime.Object) error {
	accessor, err := meta.TypeAccessor(obj)
	if err != nil {
		return err
	}

	kind := accessor.GetKind()
	switch kind {
	case "Service":
		return waitForServiceReady(ctx, c, obj)
	case "Deployment":
		return waitForDeploymentReady(ctx, c, obj)
	}

	return fmt.Errorf("Unsupported kind %s", kind)
}

// Form runtime object with meta info and kind specified
func getRuntimeObject(om metav1.ObjectMeta, kind string) runtime.Object {
	switch kind {
	case "Service":
		return &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
			ObjectMeta: om,
		}
	default:
		// Deployment
		return &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
			},
			ObjectMeta: om,
		}
	}
}
