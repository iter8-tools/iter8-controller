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
	"context"
	"fmt"
	"gopkg.in/yaml.v2"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
)

func addFinalizerIfAbsent(context context.Context, c client.Client, instance *iter8v1alpha1.Experiment, fName string) (err error) {
	for _, finalizer := range instance.ObjectMeta.GetFinalizers() {
		if finalizer == fName {
			return
		}
	}

	instance.SetFinalizers(append(instance.GetFinalizers(), Finalizer))
	if err = c.Update(context, instance); err != nil {
		Logger(context).Info("setting finalizer failed. (retrying)", "error", err)
	}

	return
}

func removeFinalizer(context context.Context, c client.Client, instance *iter8v1alpha1.Experiment, fName string) (err error) {
	finalizers := make([]string, 0)
	for _, f := range instance.GetFinalizers() {
		if f != fName {
			finalizers = append(finalizers, f)
		}
	}
	instance.SetFinalizers(finalizers)
	if err = c.Update(context, instance); err != nil {
		Logger(context).Info("setting finalizer failed. (retrying)", "error", err)
	}

	Logger(context).Info("FinalizerRemoved")
	return
}

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

type Metrics []Metric
type Metric struct {
	Name               string `yaml:"name"`
	Type               string `yaml:"metric_type"`
	SampleSizeTemplate string `yaml:"sample_size_query_template"`
}

func readMetrics(context context.Context, c client.Client, instance *iter8v1alpha1.Experiment) error {
	log := Logger(context)
	cm := &corev1.ConfigMap{}
	err := c.Get(context, types.NamespacedName{Name: MetricsConfigMap, Namespace: Iter8Namespace}, cm)
	if err != nil {
		if err = c.Get(context, types.NamespacedName{Name: MetricsConfigMap, Namespace: instance.GetNamespace()}, cm); err != nil {
			log.Info("MetricsConfigMapNotFound")
			return nil
		}
	}

	data := cm.Data
	var templates map[string]string
	metrics := Metrics{}

	err = yaml.Unmarshal([]byte(data["query_templates"]), &templates)
	if err != nil {
		log.Error(err, "FailToReadYaml", "query_templates", data["query_templates"])
		return err
	}

	err = yaml.Unmarshal([]byte(data["metrics"]), &metrics)
	if err != nil {
		log.Error(err, "FailToReadYaml", "metrics", data["metrics"])
		return err
	}

	instance.Metrics = make(map[string]iter8v1alpha1.ExperimentMetric)
	criteria := make(map[string]bool)

	for _, criterion := range instance.Spec.Analysis.SuccessCriteria {
		criteria[criterion.MetricName] = true
	}

	for _, metric := range metrics {
		if !criteria[metric.Name] {
			continue
		}

		m := iter8v1alpha1.ExperimentMetric{
			Type: metric.Type,
		}
		qTpl, ok := templates[metric.Name]
		if !ok {
			return fmt.Errorf("FailToReadQueryTemplateForMetric %s", metric.Name)
		}
		sTpl, ok := templates[metric.SampleSizeTemplate]
		if !ok {
			return fmt.Errorf("FailToReadSampleSizeTemplateForMetric %s", metric.Name)
		}
		m.QueryTemplate = qTpl
		m.SampleSizeTemplate = sTpl
		instance.Metrics[metric.Name] = m
	}

	err = c.Update(context, instance)
	return err
}
