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

package adapter

import (
	"strings"

	iter8v1alpha2 "github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
)

const (
	keySeparator = "/"
)

func experimentKey(instance *iter8v1alpha2.Experiment) string {
	return instance.Namespace + keySeparator + instance.Name
}

func targetKey(name, namespace string) string {
	return namespace + keySeparator + name
}

// return namespace, name of experiment
func resolveExperimentKey(val string) (string, string) {
	out := strings.Split(val, keySeparator)
	if len(out) != 2 {
		return "", ""
	}
	return out[0], out[1]
}
