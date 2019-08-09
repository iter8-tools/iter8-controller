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
	"fmt"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
)

func getServiceNamespace(instance *iter8v1alpha1.Experiment) string {
	serviceNamespace := instance.Spec.TargetService.Namespace
	if serviceNamespace == "" {
		serviceNamespace = instance.Namespace
	}
	return serviceNamespace
}

func updateGrafanaURL(instance *iter8v1alpha1.Experiment, namespace string) {
	endTs := instance.Status.EndTimestamp
	if endTs == "" {
		endTs = "now"
	}
	instance.Status.GrafanaURL = instance.Spec.Analysis.GetGrafanaEndpoint() +
		"/d/eXPEaNnZz/iter8-application-metrics?" +
		"var-namespace=" + namespace +
		"&var-service=" + instance.Spec.TargetService.Name +
		"&var-baseline=" + instance.Spec.TargetService.Baseline +
		"&var-candidate=" + instance.Spec.TargetService.Candidate +
		"&from=" + instance.Status.StartTimestamp +
		"&to=" + endTs
}

func getStrategy(instance *iter8v1alpha1.Experiment) string {
	strategy := instance.Spec.TrafficControl.GetStrategy()
	if strategy == "check_and_increment" &&
		(instance.Spec.Analysis.SuccessCriteria == nil || len(instance.Spec.Analysis.SuccessCriteria) == 0) {
		strategy = "increment_without_check"
	}
	return strategy
}

func experimentSucceeded(instance *iter8v1alpha1.Experiment) bool {
	switch getStrategy(instance) {
	case "increment_without_check":
		return instance.Spec.Assessment == iter8v1alpha1.AssessmentOverrideSuccess ||
			instance.Spec.Assessment == iter8v1alpha1.AssessmentNull
	case "check_and_increment":
		return instance.Spec.Assessment == iter8v1alpha1.AssessmentOverrideSuccess ||
			instance.Status.AssessmentSummary.AllSuccessCriteriaMet && instance.Spec.Assessment == iter8v1alpha1.AssessmentNull
	default:
		return false
	}
}

func markExperimentCompleted(instance *iter8v1alpha1.Experiment) {
	// Clear analysis state
	instance.Status.AnalysisState.Raw = []byte("{}")

	// Update grafana url
	ts := metav1.Now().UTC().UnixNano() / int64(time.Millisecond)
	instance.Status.EndTimestamp = strconv.FormatInt(ts, 10)
	updateGrafanaURL(instance, getServiceNamespace(instance))

	instance.Status.MarkExperimentCompleted()
}

func markExperimentSuccessStatus(instance *iter8v1alpha1.Experiment, trafficMsg string) {
	if instance.Spec.Assessment == iter8v1alpha1.AssessmentOverrideSuccess {
		instance.Status.MarkExperimentSucceeded(fmt.Sprintf("OverrideSuccess, Traffic: %s", trafficMsg), "")
	} else if instance.Status.AssessmentSummary.AllSuccessCriteriaMet {
		instance.Status.MarkExperimentSucceeded(fmt.Sprintf("AllSuccessCriteriaMet, Traffic: %s", trafficMsg), "")
	} else {
		instance.Status.MarkExperimentSucceeded(fmt.Sprintf("IterationsExhausted, Traffic: %s", trafficMsg), "")
	}
}

func markExperimentFailureStatus(instance *iter8v1alpha1.Experiment, trafficMsg string) {
	if instance.Spec.Assessment == iter8v1alpha1.AssessmentOverrideFailure {
		instance.Status.MarkExperimentFailed(fmt.Sprintf("OverrideFailure, Traffic: %s", trafficMsg), "")
	} else if instance.Status.AssessmentSummary.AllSuccessCriteriaMet {
		instance.Status.MarkExperimentFailed(fmt.Sprintf("NotAllSuccessCriteriaMet, Traffic: %s", trafficMsg), "")
	} else {
		// Should not be reached
		instance.Status.MarkExperimentFailed(fmt.Sprintf("UnexpectedCondition"), "")
	}
}
