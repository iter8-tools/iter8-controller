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

package experiment

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/iter8-tools/iter8-controller/pkg/analytics/checkandincrement"
	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
)

const (
	MetricsConfigMap = "iter8-metrics"
)

func MakeRequest(instance *iter8v1alpha1.Experiment, baseline, experiment interface{}) *checkandincrement.Request {
	spec := instance.Spec

	criteria := make([]checkandincrement.SuccessCriterion, len(spec.Analysis.SuccessCriteria))
	for i, criterion := range spec.Analysis.SuccessCriteria {
		iter8metric, ok := instance.Metrics[criterion.MetricName]
		if !ok {
			// Metric template not found
			return nil
		}
		criteria[i] = checkandincrement.SuccessCriterion{
			MetricName:         criterion.MetricName,
			Type:               criterion.ToleranceType,
			Value:              criterion.Tolerance,
			Template:           iter8metric.QueryTemplate,
			SampleSizeTemplate: iter8metric.SampleSizeTemplate,
			MetricType:         iter8metric.Type,
		}

		criteria[i].SampleSize = criterion.GetSampleSize()
		criteria[i].StopOnFailure = criterion.GetStopOnFailure()
	}
	now := time.Now().Format(time.RFC3339)
	destinationKey, namespaceKey, baseVal, experimentVal, baseNsVal, experimentNsVal := "", "", "", "", "", ""
	switch instance.Spec.TargetService.APIVersion {
	case KubernetesService:
		destinationKey = "destination_workload"
		namespaceKey = "destination_service_namespace"
		baseVal = baseline.(*appsv1.Deployment).GetName()
		experimentVal = experiment.(*appsv1.Deployment).GetName()
		baseNsVal = baseline.(*appsv1.Deployment).GetNamespace()
		experimentNsVal = experiment.(*appsv1.Deployment).GetNamespace()
	case KnativeServiceV1Alpha1:
		destinationKey = "destination_service_name"
		namespaceKey = "destination_service_namespace"
		baseVal = baseline.(*corev1.Service).GetName()
		experimentVal = experiment.(*corev1.Service).GetName()
		baseNsVal = baseline.(*corev1.Service).GetNamespace()
		experimentNsVal = experiment.(*corev1.Service).GetNamespace()
	default:
		// TODO: add err information
		return &checkandincrement.Request{}
	}

	// TODO: change analytics server API to modify "canary" to "candidate"
	onSuccess := instance.Spec.TrafficControl.GetOnSuccess()
	if onSuccess == "candidate" {
		onSuccess = "canary"
	}

	return &checkandincrement.Request{
		Name: instance.Name,
		Baseline: checkandincrement.Window{
			StartTime: instance.ObjectMeta.GetCreationTimestamp().Format(time.RFC3339),
			EndTime:   now,
			Tags: map[string]string{
				destinationKey: baseVal,
				namespaceKey:   baseNsVal,
			},
		},
		Canary: checkandincrement.Window{
			StartTime: instance.ObjectMeta.GetCreationTimestamp().Format(time.RFC3339),
			EndTime:   now,
			Tags: map[string]string{
				destinationKey: experimentVal,
				namespaceKey:   experimentNsVal,
			},
		},
		TrafficControl: checkandincrement.TrafficControl{
			MaxTrafficPercent: instance.Spec.TrafficControl.GetMaxTrafficPercentage(),
			StepSize:          instance.Spec.TrafficControl.GetStepSize(),
			OnSuccess:         onSuccess,
			SuccessCriteria:   criteria,
		},
		LastState: instance.Status.AnalysisState,
	}
}
