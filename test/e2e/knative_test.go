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
package e2e

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/iter8-tools/iter8-controller/pkg/analytics"

	"github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
	"github.com/iter8-tools/iter8-controller/test"
)

// TestKnativeExperiment tests various experiment scenarios on Knative platform
func TestKnativeExperiment(t *testing.T) {
	service := test.StartAnalytics()
	defer service.Close()
	testCases := map[string]testCase{
		"missingService": testCase{
			object:    getDoNotExistExperiment(),
			wantState: test.CheckServiceNotFound("TargetsNotFound"),
		},
		"missingbaseline": func(name string) testCase {
			return testCase{
				mocks: map[string]analytics.Response{
					name: test.GetSuccessMockResponse(),
				},
				initObjects: []runtime.Object{
					getBaseStockService(name),
				},
				object:    getMissingBaselineExperiment(name, name, service.GetURL()),
				wantState: test.CheckServiceNotFound("TargetsNotFound"),
			}
		}("stock-missingbaseline"),
		"missingcandidate": func(name string) testCase {
			return testCase{
				mocks: map[string]analytics.Response{
					name: test.GetSuccessMockResponse(),
				},
				initObjects: []runtime.Object{
					getBaseStockService(name),
				},
				object:    getMissingCandidateExperiment(name, name, service.GetURL()),
				wantState: test.CheckServiceNotFound("TargetsNotFound"),
			}
		}("stock-missingcandidate"),
		"rollforward": testCase{
			mocks: map[string]analytics.Response{
				"stock-rollforward": test.GetSuccessMockResponse(),
			},
			initObjects: []runtime.Object{
				getBaseStockService("stock-rollforward"),
			},
			preHook: newStockServiceRevision("stock-rollforward", 50),
			object:  getFastExperimentForService("stock-rollforward", "stock-rollforward", service.GetURL()),
			wantState: test.WantAllStates(
				test.CheckExperimentFinished,
				test.CheckExperimentSuccess,
			),
			wantResults: []runtime.Object{
				getRollforwardStockService("stock-rollforward"),
			},
		},
		"rollbackward": testCase{
			mocks: map[string]analytics.Response{
				"stock-rollbackward": test.GetFailureMockResponse(),
			},
			initObjects: []runtime.Object{
				getBaseStockService("stock-rollbackward"),
			},
			preHook: newStockServiceRevision("stock-rollbackward", 0),
			object:  getFastExperimentForService("stock-rollbackward", "stock-rollbackward", service.GetURL()),
			wantState: test.WantAllStates(
				test.CheckExperimentFinished,
				test.CheckExperimentFailure,
			),
			wantResults: []runtime.Object{
				getRollBackwardStockService("stock-rollbackward"),
			},
		},
		"ongoingdelete": testCase{
			mocks: map[string]analytics.Response{
				"stock-ongoingdelete": test.GetSuccessMockResponse(),
			},
			initObjects: []runtime.Object{
				getBaseStockService("stock-ongoingdelete"),
			},
			object:    getSlowExperimentForService("stock-ongoingdelete", "stock-ongoingdelete", service.GetURL()),
			preHook:   newStockServiceRevision("stock-ongoingdelete", 50),
			wantState: test.CheckServiceFound,
			wantResults: []runtime.Object{
				getRollBackwardStockService("stock-ongoingdelete"),
			},
			postHook: test.DeleteExperiment("stock-ongoingdelete", Flags.Namespace),
		},
		"completedelete": testCase{
			mocks: map[string]analytics.Response{
				"stock-completedelete": test.GetDefaultMockResponse(),
			},
			initObjects: []runtime.Object{
				getBaseStockService("stock-completedelete"),
			},
			object:    getFastExperimentForService("stock-completedelete", "stock-completedelete", service.GetURL()),
			preHook:   newStockServiceRevision("stock-completedelete", 0),
			wantState: test.CheckExperimentFinished,
			frozenObjects: []runtime.Object{
				test.NewKnativeService("stock-completedelete", Flags.Namespace).Build(),
			},
			postHook: test.DeleteExperiment("stock-completedelete", Flags.Namespace),
		},
	}

	runTestCases(t, service, testCases)
}

func getDoNotExistExperiment() *v1alpha1.Experiment {
	return test.NewExperiment("experiment-missing-service", Flags.Namespace).
		WithKNativeService("doesnotexist").
		Build()
}

func getMissingBaselineExperiment(name string, serviceName string, analyticsHost string) *v1alpha1.Experiment {
	experiment := test.NewExperiment(name, Flags.Namespace).
		WithKNativeService(serviceName).
		Build()
	experiment.Spec.TargetService.Baseline = serviceName + "-donotexist"
	return experiment
}

func getMissingCandidateExperiment(name string, serviceName string, analyticsHost string) *v1alpha1.Experiment {
	experiment := test.NewExperiment(name, Flags.Namespace).
		WithKNativeService(serviceName).
		Build()
	experiment.Spec.TargetService.Baseline = serviceName + "-one"
	experiment.Spec.TargetService.Candidate = serviceName + "-donotexist"
	return experiment
}

func getFastExperimentForService(name string, serviceName string, analyticsHost string) *v1alpha1.Experiment {
	experiment := test.NewExperiment(name, Flags.Namespace).
		WithKNativeService(serviceName).
		WithAnalyticsHost(analyticsHost).
		WithDummySuccessCriterion().
		Build()
	onesec := "1s"
	one := 1
	experiment.Spec.TargetService.Baseline = serviceName + "-one"
	experiment.Spec.TargetService.Candidate = serviceName + "-two"
	experiment.Spec.TrafficControl.Interval = &onesec
	experiment.Spec.TrafficControl.MaxIterations = &one
	return experiment
}

func getSlowExperimentForService(name string, serviceName string, analyticsHost string) *v1alpha1.Experiment {
	experiment := test.NewExperiment(name, Flags.Namespace).
		WithKNativeService(serviceName).
		WithAnalyticsHost(analyticsHost).
		WithDummySuccessCriterion().
		Build()

	twentysecs := "10s"
	two := 2
	experiment.Spec.TargetService.Baseline = serviceName + "-one"
	experiment.Spec.TargetService.Candidate = serviceName + "-two"
	experiment.Spec.TrafficControl.Interval = &twentysecs
	experiment.Spec.TrafficControl.MaxIterations = &two
	return experiment
}

func getBaseStockService(name string) runtime.Object {
	return test.NewKnativeService(name, Flags.Namespace).
		WithImage(test.StockImageName).
		WithRevision(name+"-one", 100).
		Build()
}

func getRollforwardStockService(name string) runtime.Object {
	ksvc := test.NewKnativeService(name, Flags.Namespace).
		WithImage(test.StockImageName).
		WithRevision(name+"-one", 0).
		WithRevision(name+"-two", 100)

	return ksvc.Build()
}

func getRollBackwardStockService(name string) runtime.Object {
	ksvc := test.NewKnativeService(name, Flags.Namespace).
		WithImage(test.StockImageName).
		WithRevision(name+"-one", 100).
		WithRevision(name+"-two", 0)

	return ksvc.Build()
}

func newStockServiceRevision(name string, percent int) test.Hook {
	return func(ctx context.Context, cl client.Client) error {
		ksvc := test.NewKnativeService(name, Flags.Namespace).Build()

		err := test.WaitForState(ctx, cl, ksvc, test.CheckServiceReady)
		if err != nil {
			return err
		}

		(*test.KnativeServiceBuilder)(ksvc).
			WithEnv("RESOURCE", "share").
			WithRevision(name+"-two", percent)

		ksvc.Spec.RouteSpec.Traffic[0].Percent = 100 - percent

		if err := cl.Update(ctx, ksvc); err != nil {
			return errors.Wrapf(err, "Cannot update service %s", name)
		}

		return test.WaitForState(ctx, cl, ksvc, test.CheckLatestReadyRevisionName(name+"-two"))
	}
}
