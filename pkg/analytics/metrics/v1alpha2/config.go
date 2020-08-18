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
package metrics

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iter8v1alpha2 "github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
)

const (
	configMapName    = "iter8config-metrics"
	defaultNamespace = "iter8"

	counterMetricsName = "counter_metrics.yaml"
	ratioMetricsName   = "ratio_metrics.yaml"
)

// Read will read metrics from configmap into experiment.
// Configmap in the same namespace as the experiment will override the one in iter8 system namespace.
func Read(context context.Context, c client.Client, instance *iter8v1alpha2.Experiment) error {
	cmSystem := &corev1.ConfigMap{}

	errSystem := c.Get(context, types.NamespacedName{Name: configMapName, Namespace: getConfigMapNamespace()}, cmSystem)

	if errSystem != nil {
		return fmt.Errorf("Fail to read metrics configmaps: %v", errSystem)
	}

	instance.Spec.Metrics = &iter8v1alpha2.Metrics{}
	err := yaml.Unmarshal([]byte(cmSystem.Data[counterMetricsName]), &instance.Spec.Metrics.CounterMetrics)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal([]byte(cmSystem.Data[ratioMetricsName]), &instance.Spec.Metrics.RatioMetrics)
	if err != nil {
		return err
	}

	return nil
}

func getConfigMapNamespace() string {
	if namespace := os.Getenv("POD_NAMESPACE"); namespace != "" {
		return namespace
	}
	return defaultNamespace
}
