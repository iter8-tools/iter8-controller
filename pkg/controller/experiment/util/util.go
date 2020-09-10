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
package util

import (
	"context"

	"github.com/go-logr/logr"
	iter8v1alpha2 "github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
)

type loggerKeyType string

const (
	// LoggerKey is the key used to extract logger from context
	LoggerKey = loggerKeyType("logger")

	// IstioClientKey is the key used to extract istio client from context
	IstioClientKey = "istioClient"
)

// Logger gets the logger from the context.
func Logger(ctx context.Context) logr.Logger {
	return ctx.Value(LoggerKey).(logr.Logger)
}

// ServiceToFullHostName returns the full dns name for internal service
func ServiceToFullHostName(svcName, namespace string) string {
	return svcName + "." + namespace + ".svc.cluster.local"
}

// FullExperimentName returns the namespaced name for experiment
func FullExperimentName(instance *iter8v1alpha2.Experiment) string {
	return instance.GetName() + "." + instance.GetNamespace()
}

// GetDefaultHost returns the default host for experiment
func GetDefaultHost(instance *iter8v1alpha2.Experiment) string {
	if instance.Spec.Service.Name != "" {
		return ServiceToFullHostName(instance.Spec.Service.Name, instance.ServiceNamespace())
	}
	if len(instance.Spec.Networking.Hosts) > 0 {
		return instance.Spec.Networking.Hosts[0].Name
	}

	return "iter8"
}
