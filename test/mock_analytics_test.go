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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/iter8-tools/iter8-istio/pkg/analytics"
	analyticsapi "github.com/iter8-tools/iter8-istio/pkg/analytics/api/v1alpha2"
)

func TestMockAnalytics(t *testing.T) {
	logger := Logger(t)
	service := StartAnalytics()
	defer service.Close()

	name, namespace := "test-0", "test-ns"
	experiment := NewExperiment(name, namespace).
		WithKubernetesTargetService("", "baseline", []string{"candiate1, candidate2"}).
		Build()

	want := GetRollToWinnerMockResponse(experiment, 0)
	service.AddMock(name, want)

	got, err := analytics.Invoke(logger, service.GetURL(), dummyRequest())
	if err != nil {
		t.Fatalf("%v", err)
	}

	if diff := cmp.Diff(want, *got); diff != "" {
		t.Errorf("unexpected reponse diff (-want, +got) = %v", diff)
	}
}

func dummyRequest() *analyticsapi.Request {
	return &analyticsapi.Request{
		Name: "test-0",
	}
}
