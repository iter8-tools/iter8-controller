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
	"testing"

	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	analtyicsapi "github.com/iter8-tools/iter8/pkg/analytics/api/v1alpha2"
	iter8v1alpha2 "github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
	"github.com/iter8-tools/iter8/pkg/controller/experiment/routing/router/istio"
	"github.com/iter8-tools/iter8/pkg/controller/experiment/util"
	"github.com/iter8-tools/iter8/test"
)

const (
	//Bookinfo sample constant values
	ReviewsV1Image = "istio/examples-bookinfo-reviews-v1:1.11.0"
	ReviewsV2Image = "istio/examples-bookinfo-reviews-v2:1.11.0"
	ReviewsV3Image = "istio/examples-bookinfo-reviews-v3:1.11.0"

	ReviewsPort = 9080

	// routerID used through experiment
	routerID = "reviews-router"
)

// TestKubernetesExperiment tests various experiment scenarios on Kubernetes platform
func TestExperiment(t *testing.T) {
	service := test.StartAnalytics()
	defer service.Close()
	testCases := map[string]testCase{
		"rolltowinner": func(name string, exp *iter8v1alpha2.Experiment) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetRollToWinnerMockResponse(exp, 0),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsDeployment("v1"),
					getReviewsDeployment("v2"),
					getReviewsDeployment("v3"),
				},
				object: exp,
				wantState: test.WantAllStates(
					test.CheckExperimentCompleted,
				),
				wantResults: []runtime.Object{
					getDestinationRule("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]runtime.Object{getReviewsDeployment("v1"), getReviewsDeployment("v2"), getReviewsDeployment("v3")},
					),
					getVirtualServiceForDeployments("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]int32{0, 100, 0},
					),
				},
			}
		}("rolltowinner", getFastKubernetesExperiment("rolltowinner", "reviews", "reviews-v1", service.GetURL(), []string{"reviews-v2", "reviews-v3"})),
		"rollbackward": func(name string, exp *iter8v1alpha2.Experiment) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetRollbackMockResponse(exp),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsDeployment("v1"),
					getReviewsDeployment("v2"),
					getReviewsDeployment("v3"),
				},
				object: exp,
				wantState: test.WantAllStates(
					test.CheckExperimentCompleted,
				),
				wantResults: []runtime.Object{
					getDestinationRule("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]runtime.Object{getReviewsDeployment("v1"), getReviewsDeployment("v2"), getReviewsDeployment("v3")},
					),
					getVirtualServiceForDeployments("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]int32{100, 0, 0},
					),
				},
			}
		}("rollbackward", getFastKubernetesExperiment("rollbackward", "reviews", "reviews-v1", service.GetURL(), []string{"reviews-v2", "reviews-v3"})),
		"ongoingdelete": func(name string, exp *iter8v1alpha2.Experiment) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetRollToWinnerMockResponse(exp, 0),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsDeployment("v1"),
					getReviewsDeployment("v2"),
					getReviewsDeployment("v3"),
				},
				object:    exp,
				wantState: test.CheckServiceFound,
				wantResults: []runtime.Object{
					getDestinationRule("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]runtime.Object{getReviewsDeployment("v1"), getReviewsDeployment("v2"), getReviewsDeployment("v3")},
					),
					getVirtualServiceForDeployments("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]int32{100, 0, 0},
					),
				},
				postHook: test.DeleteExperiment("ongoingdelete", Flags.Namespace),
			}
		}("ongoingdelete", getSlowKubernetesExperiment("ongoingdelete", "reviews", "reviews-v1", service.GetURL(), []string{"reviews-v2", "reviews-v3"})),
		"completedelete": func(name string, exp *iter8v1alpha2.Experiment) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetRollToWinnerMockResponse(exp, 0),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsDeployment("v1"),
					getReviewsDeployment("v2"),
					getReviewsDeployment("v3"),
				},
				object:    exp,
				wantState: test.CheckExperimentCompleted,
				frozenObjects: []runtime.Object{
					getDestinationRule("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]runtime.Object{getReviewsDeployment("v1"), getReviewsDeployment("v2"), getReviewsDeployment("v3")},
					),
					getVirtualServiceForDeployments("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]int32{100, 0, 0},
					),
				},
				postHook: test.DeleteExperiment(name, Flags.Namespace),
			}
		}("completedelete", getFastKubernetesExperiment("completedelete", "reviews", "reviews-v1", service.GetURL(), []string{"reviews-v2", "reviews-v3"})),
		"abortexperiment": func(name string, exp *iter8v1alpha2.Experiment) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetAbortExperimentResponse(exp),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsDeployment("v1"),
					getReviewsDeployment("v2"),
					getReviewsDeployment("v3"),
				},
				object: exp,
				wantState: test.WantAllStates(
					test.CheckExperimentCompleted,
				),
				wantResults: []runtime.Object{
					getDestinationRule("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]runtime.Object{getReviewsDeployment("v1"), getReviewsDeployment("v2"), getReviewsDeployment("v3")},
					),
					getVirtualServiceForDeployments("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]int32{100, 0, 0},
					),
				},
			}
		}("abortexperiment", getSlowKubernetesExperiment("abortexperiment", "reviews", "reviews-v1", service.GetURL(), []string{"reviews-v2", "reviews-v3"})),
		"emptycriterion": func(name string, exp *iter8v1alpha2.Experiment) testCase {
			return testCase{
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsDeployment("v1"),
					getReviewsDeployment("v2"),
					getReviewsDeployment("v3"),
				},
				object: exp,
				wantState: test.WantAllStates(
					test.CheckExperimentCompleted,
				),
				wantResults: []runtime.Object{
					getDestinationRule("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]runtime.Object{getReviewsDeployment("v1"), getReviewsDeployment("v2"), getReviewsDeployment("v3")},
					),
					getVirtualServiceForDeployments("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]int32{100, 0, 0},
					),
				},
			}
		}("emptycriterion", getDefaultKubernetesExperiment("emptycriterion", "reviews", "reviews-v1", []string{"reviews-v2", "reviews-v3"})),
		"cleanupdelete": func(name string, exp *iter8v1alpha2.Experiment) testCase {
			return testCase{
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsDeployment("v1"),
					getReviewsDeployment("v2"),
					getReviewsDeployment("v3"),
				},
				object: exp,
				wantState: test.WantAllStates(
					test.CheckExperimentCompleted,
				),
				postHook: test.CheckObjectDeleted(getReviewsDeployment("v2"), getReviewsDeployment("v3")),
			}
		}("cleanupdelete", getCleanUpDeleteExperiment("cleanupdelete", "reviews", "reviews-v1", []string{"reviews-v2", "reviews-v3"})),
		"attach-gateway": func(name string, exp *iter8v1alpha2.Experiment) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetRollToWinnerMockResponse(exp, 0),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsDeployment("v1"),
					getReviewsDeployment("v2"),
					getReviewsDeployment("v3"),
				},
				object: exp,
				wantState: test.WantAllStates(
					test.CheckExperimentCompleted,
				),
				wantResults: []runtime.Object{
					getDestinationRule("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]runtime.Object{getReviewsDeployment("v1"), getReviewsDeployment("v2"), getReviewsDeployment("v3")},
					),
					getVirtualServiceWithGateway("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]int32{0, 100, 0}, "reviews.com", "gateway-testing",
					),
				},
			}
		}("attach-gateway", getExperimentWithGateway("attach-gateway", "reviews", "reviews-v1", service.GetURL(),
			[]string{"reviews-v2", "reviews-v3"}, "reviews.com", "gateway-testing")),
		"pauseresume": func(name string, exp *iter8v1alpha2.Experiment) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetRollToWinnerMockResponse(exp, 1),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsDeployment("v1"),
					getReviewsDeployment("v2"),
					getReviewsDeployment("v3"),
				},
				object: exp,
				wantState: test.WantAllStates(
					test.CheckExperimentPause,
				),
				postHook: test.ResumeExperiment(exp),
				wantResults: []runtime.Object{
					getDestinationRule("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]runtime.Object{getReviewsDeployment("v1"), getReviewsDeployment("v2"), getReviewsDeployment("v3")},
					),
					getVirtualServiceForDeployments("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]int32{0, 0, 100},
					),
				},
			}
		}("pauseresume", getPauseExperiment("pauseresume", "reviews", "reviews-v1", service.GetURL(), []string{"reviews-v2", "reviews-v3"})),
		"duplicate-service": func(name string, exp, expDup *iter8v1alpha2.Experiment) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetRollToWinnerMockResponse(exp, 1),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsDeployment("v1"),
					getReviewsDeployment("v2"),
					getReviewsDeployment("v3"),
				},
				preHook:   []test.Hook{test.CreateObject(exp)},
				object:    expDup,
				wantState: test.CheckServiceNotFound("TargetsError"),
				wantResults: []runtime.Object{
					getDestinationRule("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]runtime.Object{getReviewsDeployment("v1"), getReviewsDeployment("v2"), getReviewsDeployment("v3")},
					),
					getVirtualServiceForDeployments("reviews", name,
						[]string{istio.SubsetBaseline, istio.CandidateSubsetName(0), istio.CandidateSubsetName(1)},
						[]int32{0, 0, 100},
					),
				},
				finalizers: []test.Hook{
					test.DeleteObject(expDup),
				},
			}
		}("duplicate-service", getFastKubernetesExperiment("duplicate-service", "reviews", "reviews-v1", service.GetURL(), []string{"reviews-v2", "reviews-v3"}),
			getFastKubernetesExperiment("duplicate-service-duplicate", "reviews", "reviews-v1", service.GetURL(), []string{"reviews-v2", "reviews-v3"})),
		"rolltowinner-service": func(name string, exp *iter8v1alpha2.Experiment) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetRollToWinnerMockResponse(exp, 1),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsServiceWithVersion("v1"),
					getReviewsServiceWithVersion("v2"),
					getReviewsServiceWithVersion("v3"),
				},
				object: exp,
				wantState: test.WantAllStates(
					test.CheckExperimentCompleted,
				),
				wantResults: []runtime.Object{
					getVirtualServiceForServices("reviews", name,
						[]string{util.ServiceToFullHostName("reviews-v1", Flags.Namespace),
							util.ServiceToFullHostName("reviews-v2", Flags.Namespace),
							util.ServiceToFullHostName("reviews-v3", Flags.Namespace)},
						[]int32{0, 0, 100}),
				},
			}
		}("rolltowinner-service", getFastKubernetesExperimentForService("rolltowinner-service", "reviews", "reviews-v1", service.GetURL(), []string{"reviews-v2", "reviews-v3"})),
		"rollbackward-service": func(name string, exp *iter8v1alpha2.Experiment) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetRollbackMockResponse(exp),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsServiceWithVersion("v1"),
					getReviewsServiceWithVersion("v2"),
					getReviewsServiceWithVersion("v3"),
				},
				object: exp,
				wantState: test.WantAllStates(
					test.CheckExperimentCompleted,
				),
				wantResults: []runtime.Object{
					getVirtualServiceForServices("reviews", name,
						[]string{util.ServiceToFullHostName("reviews-v1", Flags.Namespace),
							util.ServiceToFullHostName("reviews-v2", Flags.Namespace),
							util.ServiceToFullHostName("reviews-v3", Flags.Namespace)},
						[]int32{100, 0, 0}),
				},
			}
		}("rollbackward-service", getFastKubernetesExperimentForService("rollbackward-service", "reviews", "reviews-v1", service.GetURL(), []string{"reviews-v2", "reviews-v3"})),
		"same-service": func(name string, exp *iter8v1alpha2.Experiment) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetRollToWinnerMockResponse(exp, 0),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsServiceWithVersion("v1"),
				},
				object: exp,
				wantState: test.WantAllStates(
					test.CheckExperimentCompleted,
				),
				wantResults: []runtime.Object{
					getVirtualServiceForServices("reviews", name,
						[]string{util.ServiceToFullHostName("reviews-v1", Flags.Namespace),
							util.ServiceToFullHostName("reviews-v1", Flags.Namespace)},
						[]int32{0, 100}),
				},
			}
		}("same-service", getFastKubernetesExperimentForService("same-service", "reviews", "reviews-v1", service.GetURL(), []string{"reviews-v1"})),
	}

	runTestCases(t, service, testCases)
}

func getReviewsService() runtime.Object {
	return test.NewKubernetesService("reviews", Flags.Namespace).
		WithSelector(map[string]string{"app": "reviews"}).
		WithPorts(map[string]int{"http": ReviewsPort}).
		Build()
}

func getReviewsServiceWithVersion(version string) runtime.Object {
	return test.NewKubernetesService("reviews-"+version, Flags.Namespace).
		WithSelector(map[string]string{"app": "reviews"}).
		WithPorts(map[string]int{"http": ReviewsPort}).
		Build()
}

func getReviewsDeployment(version string) runtime.Object {
	labels := map[string]string{
		"app":     "reviews",
		"version": version,
	}

	image := ""

	switch version {
	case "v1":
		image = ReviewsV1Image
	case "v2":
		image = ReviewsV2Image
	case "v3":
		image = ReviewsV3Image
	default:
		image = ReviewsV1Image
	}

	return test.NewKubernetesDeployment("reviews-"+version, Flags.Namespace).
		WithLabels(labels).
		WithContainer("reviews", image, ReviewsPort).
		Build()
}

func getPauseExperiment(name, serviceName, baseline, analyticsHost string, candidates []string) *iter8v1alpha2.Experiment {
	exp := getFastKubernetesExperiment(name, serviceName, baseline, analyticsHost, candidates)
	exp.Spec.ManualOverride = &iter8v1alpha2.ManualOverride{
		Action: iter8v1alpha2.ActionPause,
	}
	return exp
}

func getCleanUpDeleteExperiment(name, serviceName, baseline string, candidates []string) *iter8v1alpha2.Experiment {
	exp := getDefaultKubernetesExperiment(name, serviceName, baseline, candidates)
	cleanup := true
	exp.Spec.Cleanup = &cleanup
	return exp
}

func getDefaultKubernetesExperiment(name, serviceName, baseline string, candidates []string) *iter8v1alpha2.Experiment {
	exp := test.NewExperiment(name, Flags.Namespace).
		WithKubernetesTargetService(serviceName, baseline, candidates).
		WithRouterID(routerID).
		Build()

	onesec := "1s"
	one := int32(1)
	exp.Spec.Duration = &iter8v1alpha2.Duration{
		Interval:      &onesec,
		MaxIterations: &one,
	}

	return exp
}

func getFastKubernetesExperiment(name, serviceName, baseline, analyticsHost string, candidates []string) *iter8v1alpha2.Experiment {
	experiment := test.NewExperiment(name, Flags.Namespace).
		WithKubernetesTargetService(serviceName, baseline, candidates).
		WithRouterID(routerID).
		WithAnalyticsEndpoint(analyticsHost).
		WithDummyCriterion().
		Build()

	onesec := "1s"
	one := int32(1)
	experiment.Spec.Duration = &iter8v1alpha2.Duration{
		Interval:      &onesec,
		MaxIterations: &one,
	}

	return experiment
}

func getFastKubernetesExperimentForService(name, serviceName, baseline, analyticsHost string, candidates []string) *iter8v1alpha2.Experiment {
	experiment := getFastKubernetesExperiment(name, serviceName, baseline, analyticsHost, candidates)
	experiment.Spec.Service.Kind = "Service"

	return experiment
}

func getExperimentWithGateway(name, serviceName, baseline, analyticsHost string, candidates []string, host, gw string) *iter8v1alpha2.Experiment {
	experiment := test.NewExperiment(name, Flags.Namespace).
		WithKubernetesTargetService(serviceName, baseline, candidates).
		WithRouterID(routerID).
		WithHostInTargetService(host, gw).
		WithAnalyticsEndpoint(analyticsHost).
		WithDummyCriterion().
		Build()

	onesec := "1s"
	one := int32(1)
	experiment.Spec.Duration = &iter8v1alpha2.Duration{
		Interval:      &onesec,
		MaxIterations: &one,
	}

	return experiment
}

func getSlowKubernetesExperiment(name, serviceName, baseline, analyticsHost string, candidates []string) *iter8v1alpha2.Experiment {
	experiment := test.NewExperiment(name, Flags.Namespace).
		WithKubernetesTargetService(serviceName, baseline, candidates).
		WithRouterID(routerID).
		WithAnalyticsEndpoint(analyticsHost).
		WithDummyCriterion().
		Build()

	tensecs := "10s"
	two := int32(2)
	experiment.Spec.Duration = &iter8v1alpha2.Duration{
		Interval:      &tensecs,
		MaxIterations: &two,
	}

	return experiment
}

func getDestinationRule(serviceName, name string, subsets []string, objs []runtime.Object) runtime.Object {
	ruleName := istio.GetRoutingRuleName(routerID)
	drb := istio.NewDestinationRule(ruleName, util.ServiceToFullHostName(serviceName, Flags.Namespace), name, Flags.Namespace)
	for i, subset := range subsets {
		drb.WithSubset(objs[i].(*appsv1.Deployment), subset)
	}
	return drb.Build()
}

func getVirtualServiceForDeployments(serviceName, name string, subsets []string, weights []int32) runtime.Object {
	host := util.ServiceToFullHostName(serviceName, Flags.Namespace)
	ruleName := istio.GetRoutingRuleName(routerID)
	vsb := istio.NewVirtualService(ruleName, name, Flags.Namespace)
	rb := istio.NewEmptyHTTPRoute()
	for i, subset := range subsets {
		destination := istio.NewHTTPRouteDestination().
			WithHost(host).
			WithSubset(subset).
			WithWeight(weights[i]).Build()
		rb = rb.WithDestination(destination)
	}

	return vsb.WithHTTPRoute(rb.Build()).WithMeshGateway().WithHosts([]string{host}).Build()
}

func getVirtualServiceForServices(serviceName, name string, destinations []string, weights []int32) runtime.Object {
	host := util.ServiceToFullHostName(serviceName, Flags.Namespace)
	ruleName := istio.GetRoutingRuleName(routerID)
	vsb := istio.NewVirtualService(ruleName, name, Flags.Namespace)
	rb := istio.NewEmptyHTTPRoute()
	for i, name := range destinations {
		destination := istio.NewHTTPRouteDestination().
			WithHost(name).
			WithWeight(weights[i]).Build()
		rb = rb.WithDestination(destination)
	}

	return vsb.WithHTTPRoute(rb.Build()).WithMeshGateway().WithHosts([]string{host}).Build()
}

func getVirtualServiceWithGateway(serviceName, name string, subsets []string, weights []int32, host, gw string) runtime.Object {
	return istio.NewVirtualServiceBuilder(getVirtualServiceForDeployments(serviceName, name, subsets, weights).(*v1alpha3.VirtualService)).
		WithGateways([]string{gw}).
		WithHosts([]string{host}).
		Build()
}
