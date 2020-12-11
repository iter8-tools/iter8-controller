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
package test

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ServiceBuilder corev1.Service
type DeploymentBuilder appsv1.Deployment

// NewKubernetesService returns a kubernetes service
func NewKubernetesService(name, namespace string) *ServiceBuilder {
	s := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return (*ServiceBuilder)(s)
}

// NewKubernetesDeployment returns a kubernetes deployment
func NewKubernetesDeployment(name, namespace string) *DeploymentBuilder {
	d := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	return (*DeploymentBuilder)(d)
}

// WithSelector adds selector labels to the service
func (s *ServiceBuilder) WithSelector(selector map[string]string) *ServiceBuilder {
	if s.Spec.Selector == nil {
		s.Spec.Selector = make(map[string]string)
	}

	for key, val := range selector {
		s.Spec.Selector[key] = val
	}

	return s
}

// WithPorts adds ports to the service
func (s *ServiceBuilder) WithPorts(ports map[string]int) *ServiceBuilder {
	for name, port := range ports {
		s.Spec.Ports = append(s.Spec.Ports, corev1.ServicePort{
			Name: name,
			Port: int32(port),
		})
	}

	return s
}

// Build converts builder to service
func (s *ServiceBuilder) Build() *corev1.Service {
	return (*corev1.Service)(s)
}

// WithLabels adds labels to the deployment
func (d *DeploymentBuilder) WithLabels(l map[string]string) *DeploymentBuilder {
	d.ObjectMeta.SetLabels(l)
	d.Spec.Template.ObjectMeta.SetLabels(l)
	d.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: make(map[string]string),
	}

	for key, val := range l {
		d.Spec.Selector.MatchLabels[key] = val
	}

	return d
}

// WithContainer adds a container spec to the deployment
func (d *DeploymentBuilder) WithContainer(name, image string, port int) *DeploymentBuilder {
	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers,
		corev1.Container{
			Name:  name,
			Image: image,
			Ports: []corev1.ContainerPort{{
				ContainerPort: int32(port),
			}},
		})
	return d
}

// Build converts builder to deployment
func (d *DeploymentBuilder) Build() *appsv1.Deployment {
	return (*appsv1.Deployment)(d)
}
