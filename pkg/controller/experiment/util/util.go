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
	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
)

type loggerKeyType string

const (
	LoggerKey = loggerKeyType("logger")
)

// Logger gets the logger from the context.
func Logger(ctx context.Context) logr.Logger {
	return ctx.Value(LoggerKey).(logr.Logger)
}

func EqualHost(host1, ns1, host2, ns2 string) bool {
	if host1 == host2 ||
		host1 == host2+"."+ns2+".svc.cluster.local" ||
		host1+"."+ns1+".svc.cluster.local" == host2 {
		return true
	}
	return false
}

func ServiceToFullHostName(svcName, namespace string) string {
	return svcName + "." + namespace + ".svc.cluster.local"
}

func FullExperimentName(instance *iter8v1alpha1.Experiment) string {
	return instance.GetName() + "." + instance.GetNamespace()
}

func GetHost(instance *iter8v1alpha1.Experiment) string {
	if instance.Spec.TargetService.Name != "" {
		return ServiceToFullHostName(instance.Spec.TargetService.Name, instance.ServiceNamespace())
	}

	if len(instance.Spec.TargetService.Hosts) > 0 {
		return instance.Spec.TargetService.Hosts[0].Name
	}

	return ""
}
