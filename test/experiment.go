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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
)

// ExperimentBuilder builds experiment object
type ExperimentBuilder v1alpha2.Experiment

// NewExperiment create a new miminal experiment object
func NewExperiment(name string, namespace string) *ExperimentBuilder {
	experiment := &v1alpha2.Experiment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha2.SchemeGroupVersion.String(),
			Kind:       "Experiment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	experiment.InitStatus()
	return (*ExperimentBuilder)(experiment)
}

// Build the experiment object
func (b *ExperimentBuilder) Build() *v1alpha2.Experiment {
	return (*v1alpha2.Experiment)(b)
}

// WithKubernetesTargetService adds Kubernetes targetService
func (b *ExperimentBuilder) WithKubernetesTargetService(name, baseline string, candidates []string) *ExperimentBuilder {
	b.Spec.Service = v1alpha2.Service{
		ObjectReference: &corev1.ObjectReference{
			Name: name,
		},
		Baseline:   baseline,
		Candidates: candidates,
	}
	return b
}

// WithDummyCriterion adds a dummy criterion
func (b *ExperimentBuilder) WithDummyCriterion() *ExperimentBuilder {
	return b.WithCriterion(v1alpha2.Criterion{
		Metric: "iter8_latency",
		Threshold: &v1alpha2.Threshold{
			Type:  "relative",
			Value: 3000,
		}})
}

func (b *ExperimentBuilder) WithAnalyticsEndpoint(host string) *ExperimentBuilder {
	b.Spec.AnalyticsEndpoint = &host
	return b
}

// WithCriterion adds a criterion
func (b *ExperimentBuilder) WithCriterion(c v1alpha2.Criterion) *ExperimentBuilder {
	b.Spec.Criteria = append(b.Spec.Criteria, c)
	return b
}

func (b *ExperimentBuilder) WithRouterID(id string) *ExperimentBuilder {
	if b.Spec.Networking == nil {
		b.Spec.Networking = &v1alpha2.Networking{}
	}
	b.Spec.Networking.ID = &id
	return b
}

func (b *ExperimentBuilder) WithResumeAction() *ExperimentBuilder {
	b.Spec.ManualOverride = &v1alpha2.ManualOverride{
		Action: v1alpha2.ActionResume,
	}
	return b
}

func (b *ExperimentBuilder) WithExternalHost(host, gateway string) *ExperimentBuilder {
	if b.Spec.Networking == nil {
		b.Spec.Networking = &v1alpha2.Networking{}
	}
	b.Spec.Networking.Hosts = []v1alpha2.Host{{
		Name:    host,
		Gateway: gateway,
	}}

	return b
}

func CheckExperimentPause(obj runtime.Object) (bool, error) {
	exp, ok := obj.(*v1alpha2.Experiment)
	if !ok {
		return false, fmt.Errorf("Expected an experiment instance (got: %v)", obj)
	}

	return exp.Status.Phase == v1alpha2.PhasePause, nil
}

func CheckExperimentCompleted(obj runtime.Object) (bool, error) {
	exp, ok := obj.(*v1alpha2.Experiment)
	if !ok {
		return false, fmt.Errorf("Expected an experiment instance (got: %v)", obj)
	}

	return exp.Status.GetCondition(v1alpha2.ExperimentConditionExperimentCompleted).IsTrue(), nil
}

func CheckServiceFound(obj runtime.Object) (bool, error) {
	exp, ok := obj.(*v1alpha2.Experiment)
	if !ok {
		return false, fmt.Errorf("Expected an experiment service (got: %v)", obj)
	}

	return exp.Status.GetCondition(v1alpha2.ExperimentConditionTargetsProvided).IsTrue(), nil
}

func CheckServiceNotFound(reason string) func(obj runtime.Object) (bool, error) {
	return func(obj runtime.Object) (bool, error) {
		exp, ok := obj.(*v1alpha2.Experiment)
		if !ok {
			return false, fmt.Errorf("Expected an experiment service (got: %v)", obj)
		}
		cond := exp.Status.GetCondition(v1alpha2.ExperimentConditionTargetsProvided)

		return cond.IsFalse() && *cond.Reason == reason, nil
	}
}

func DeleteExperiment(name string, namespace string) Hook {
	return DeleteObject(NewExperiment(name, namespace).Build())
}

func ResumeExperiment(exp *v1alpha2.Experiment) Hook {
	newexp := exp.DeepCopy()
	return UpdateObject((*ExperimentBuilder)(newexp).
		WithResumeAction().
		Build())
}
