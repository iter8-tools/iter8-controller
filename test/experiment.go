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

	"github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
)

// ExperimentBuilder builds experiment object
type ExperimentBuilder v1alpha1.Experiment

// NewExperiment create a new miminal experiment object
func NewExperiment(name string, namespace string) *ExperimentBuilder {
	experiment := &v1alpha1.Experiment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "Experiment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	experiment.Status.Init()
	return (*ExperimentBuilder)(experiment)
}

// Build the experiment object
func (b *ExperimentBuilder) Build() *v1alpha1.Experiment {
	return (*v1alpha1.Experiment)(b)
}

// WithKNativeService adds KNative target service
func (b *ExperimentBuilder) WithKNativeService(name string) *ExperimentBuilder {
	b.Spec.TargetService = v1alpha1.TargetService{
		ObjectReference: &corev1.ObjectReference{
			APIVersion: "serving.knative.dev/v1alpha1",
			Name:       name,
		},
	}
	return b
}

// WithKubernetesTargetService adds Kubernetes targetService
func (b *ExperimentBuilder) WithKubernetesTargetService(name, baseline, candidate string) *ExperimentBuilder {
	b.Spec.TargetService = v1alpha1.TargetService{
		ObjectReference: &corev1.ObjectReference{
			APIVersion: "v1",
			Name:       name,
		},
		Baseline:  baseline,
		Candidate: candidate,
	}
	return b
}

// WithKubernetesTargetServiceKind specifies kind of baseline/candidate
func (b *ExperimentBuilder) WithKubernetesTargetServiceKind(kind string) *ExperimentBuilder {
	b.Spec.TargetService.Kind = kind
	return b
}

// WithDummySuccessCriterion adds a dummy success criterion
func (b *ExperimentBuilder) WithDummySuccessCriterion() *ExperimentBuilder {
	return b.WithSuccessCriterion(v1alpha1.SuccessCriterion{
		MetricName:    "iter8_latency",
		ToleranceType: v1alpha1.ToleranceTypeDelta,
		Tolerance:     0.02,
	})
}

func (b *ExperimentBuilder) WithAnalyticsHost(host string) *ExperimentBuilder {
	b.Spec.Analysis.AnalyticsService = host
	return b
}

// WithSuccessCriterion adds a success criterion
func (b *ExperimentBuilder) WithSuccessCriterion(sc v1alpha1.SuccessCriterion) *ExperimentBuilder {
	b.Spec.Analysis.SuccessCriteria = append(b.Spec.Analysis.SuccessCriteria, sc)
	return b
}

func (b *ExperimentBuilder) WithResumeAction() *ExperimentBuilder {
	b.Action = v1alpha1.ActionResume
	return b
}

func CheckExperimentPause(obj runtime.Object) (bool, error) {
	exp, ok := obj.(*v1alpha1.Experiment)
	if !ok {
		return false, fmt.Errorf("Expected an experiment instance (got: %v)", obj)
	}

	return exp.Status.Phase == v1alpha1.PhasePause, nil
}

func CheckExperimentFinished(obj runtime.Object) (bool, error) {
	exp, ok := obj.(*v1alpha1.Experiment)
	if !ok {
		return false, fmt.Errorf("Expected an experiment instance (got: %v)", obj)
	}

	return exp.Status.GetCondition(v1alpha1.ExperimentConditionExperimentCompleted).IsTrue(), nil
}

func CheckExperimentSuccess(obj runtime.Object) (bool, error) {
	exp, ok := obj.(*v1alpha1.Experiment)
	if !ok {
		return false, fmt.Errorf("Expected an experiment instance (got: %v)", obj)
	}

	return exp.Status.GetCondition(v1alpha1.ExperimentConditionExperimentSucceeded).IsTrue(), nil
}

func CheckExperimentFailure(obj runtime.Object) (bool, error) {
	exp, ok := obj.(*v1alpha1.Experiment)
	if !ok {
		return false, fmt.Errorf("Expected an experiment service (got: %v)", obj)
	}

	return exp.Status.GetCondition(v1alpha1.ExperimentConditionExperimentSucceeded).IsFalse(), nil
}

func CheckServiceFound(obj runtime.Object) (bool, error) {
	exp, ok := obj.(*v1alpha1.Experiment)
	if !ok {
		return false, fmt.Errorf("Expected an experiment service (got: %v)", obj)
	}

	return exp.Status.GetCondition(v1alpha1.ExperimentConditionTargetsProvided).IsTrue(), nil
}

func CheckServiceNotFound(reason string) func(obj runtime.Object) (bool, error) {
	return func(obj runtime.Object) (bool, error) {
		exp, ok := obj.(*v1alpha1.Experiment)
		if !ok {
			return false, fmt.Errorf("Expected an experiment service (got: %v)", obj)
		}
		cond := exp.Status.GetCondition(v1alpha1.ExperimentConditionTargetsProvided)

		return cond.IsFalse() && cond.Reason == reason, nil
	}
}

func DeleteExperiment(name string, namespace string) Hook {
	return DeleteObject(NewExperiment(name, namespace).Build())
}

func ResumeExperiment(exp *v1alpha1.Experiment) Hook {
	exp = (*ExperimentBuilder)(exp).
		WithResumeAction().
		Build()
	return UpdateObject(exp)
}
