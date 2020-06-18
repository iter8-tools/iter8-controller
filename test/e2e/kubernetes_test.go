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

	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	analtyicsapi "github.com/iter8-tools/iter8-controller/pkg/analytics/api"
	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/routing"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/util"
	"github.com/iter8-tools/iter8-controller/test"
)

const (
	//Bookinfo sample constant values
	ReviewsV1Image = "istio/examples-bookinfo-reviews-v1:1.11.0"
	ReviewsV2Image = "istio/examples-bookinfo-reviews-v2:1.11.0"
	ReviewsV3Image = "istio/examples-bookinfo-reviews-v3:1.11.0"
	RatingsImage   = "istio/examples-bookinfo-ratings-v1:1.11.0"

	ReviewsPort = 9080
	RatingsPort = 9080
)

// TestKubernetesExperiment tests various experiment scenarios on Kubernetes platform
func TestKubernetesExperiment(t *testing.T) {
	service := test.StartAnalytics()
	defer service.Close()
	testCases := map[string]testCase{
		"rollforward": testCase{
			mocks: map[string]analtyicsapi.Response{
				"rollforward": test.GetSuccessMockResponse(),
			},
			initObjects: []runtime.Object{
				getReviewsService(),
				getReviewsDeployment("v1"),
				getReviewsDeployment("v2"),
			},
			object: getFastKubernetesExperiment("rollforward", "reviews", "reviews-v1", "reviews-v2", service.GetURL()),
			wantState: test.WantAllStates(
				test.CheckExperimentFinished,
				test.CheckExperimentSuccess,
			),
			wantResults: []runtime.Object{
				getStableDestinationRule("reviews", "rollforward", "candidate", getReviewsDeployment("v2")),
				getStableVirtualService("reviews", "rollforward", "candidate"),
			},
		},
		"rollbackward": testCase{
			mocks: map[string]analtyicsapi.Response{
				"rollbackward": test.GetFailureMockResponse(),
			},
			initObjects: []runtime.Object{
				getReviewsService(),
				getReviewsDeployment("v1"),
				getReviewsDeployment("v2"),
			},
			object: getFastKubernetesExperiment("rollbackward", "reviews", "reviews-v1", "reviews-v2", service.GetURL()),
			wantState: test.WantAllStates(
				test.CheckExperimentFinished,
				test.CheckExperimentFailure,
			),
			wantResults: []runtime.Object{
				getStableDestinationRule("reviews", "rollbackward", "baseline", getReviewsDeployment("v1")),
				getStableVirtualService("reviews", "rollbackward", "baseline"),
			},
		},
		"ongoingdelete": testCase{
			mocks: map[string]analtyicsapi.Response{
				"ongoingdelete": test.GetSuccessMockResponse(),
			},
			initObjects: []runtime.Object{
				getReviewsService(),
				getReviewsDeployment("v1"),
				getReviewsDeployment("v2"),
			},
			object:    getSlowKubernetesExperiment("ongoingdelete", "reviews", "reviews-v1", "reviews-v2", service.GetURL()),
			wantState: test.CheckServiceFound,
			wantResults: []runtime.Object{
				// rollback to baseline
				getStableDestinationRule("reviews", "ongoingdelete", "baseline", getReviewsDeployment("v1")),
				getStableVirtualService("reviews", "ongoingdelete", "baseline"),
			},
			postHook: test.DeleteExperiment("ongoingdelete", Flags.Namespace),
		},
		"completedelete": testCase{
			mocks: map[string]analtyicsapi.Response{
				"completedelete": test.GetSuccessMockResponse(),
			},
			initObjects: []runtime.Object{
				getReviewsService(),
				getReviewsDeployment("v1"),
				getReviewsDeployment("v2"),
			},
			object:    getFastKubernetesExperiment("completedelete", "reviews", "reviews-v1", "reviews-v2", service.GetURL()),
			wantState: test.CheckExperimentFinished,
			frozenObjects: []runtime.Object{
				// desired end-of-experiment status
				getStableDestinationRule("reviews", "completedelete", "candidate", getReviewsDeployment("v2")),
				getStableVirtualService("reviews", "completedelete", "candidate"),
			},
			postHook: test.DeleteExperiment("completedelete", Flags.Namespace),
		},
		"abortexperiment": testCase{
			mocks: map[string]analtyicsapi.Response{
				"abortexperiment": test.GetAbortExperimentResponse(),
			},
			initObjects: []runtime.Object{
				getReviewsService(),
				getReviewsDeployment("v1"),
				getReviewsDeployment("v2"),
			},
			object: getSlowKubernetesExperiment("abortexperiment", "reviews", "reviews-v1", "reviews-v2", service.GetURL()),
			wantState: test.WantAllStates(
				test.CheckExperimentFinished,
				test.CheckExperimentFailure,
			),
			wantResults: []runtime.Object{
				// rollback to baseline
				getStableDestinationRule("reviews", "abortexperiment", "baseline", getReviewsDeployment("v1")),
				getStableVirtualService("reviews", "abortexperiment", "baseline"),
			},
		},
		"emptycriterion": testCase{
			initObjects: []runtime.Object{
				getReviewsService(),
				getReviewsDeployment("v1"),
				getReviewsDeployment("v2"),
			},
			object: getDefaultKubernetesExperiment("emptycriterion", "reviews", "reviews-v1", "reviews-v2"),
			wantState: test.WantAllStates(
				test.CheckExperimentFinished,
				test.CheckExperimentSuccess,
			),
			wantResults: []runtime.Object{
				// rollforward
				getStableDestinationRule("reviews", "emptycriterion", "candidate", getReviewsDeployment("v2")),
				getStableVirtualService("reviews", "emptycriterion", "candidate"),
			},
		},
		// "externalreference": testCase{
		// 	initObjects: []runtime.Object{
		// 		getReviewsService(),
		// 		getReviewsDeployment("v1"),
		// 		getReviewsDeployment("v2"),
		// 		getSampleEdgeVirtualService(),
		// 	},
		// 	preHook: []test.Hook{
		// 		test.DeleteObjectIfExists(getStableDestinationRule("reviews", "", "candidate", getReviewsDeployment("v2"))),
		// 		test.DeleteObjectIfExists(getStableVirtualService("reviews", "", "candidate")),
		// 	},
		// 	object:    getExperimentWithExternalReference("externalreference", "reviews", "reviews-v1", "reviews-v2"),
		// 	wantState: test.CheckExperimentFinished,
		// 	wantResults: []runtime.Object{
		// 		// rollforward
		// 		getStableDestinationRule("reviews", "externalreference", "candidate", getReviewsDeployment("v2")),
		// 		getSampleStableEdgeVirtualService(),
		// 	},
		// 	finalizers: []test.Hook{
		// 		test.DeleteObject(getStableDestinationRule("reviews", "externalreference", "candidate", getReviewsDeployment("v2"))),
		// 		test.DeleteObject(getSampleStableEdgeVirtualService()),
		// 	},
		// },
		"cleanupdelete": testCase{
			initObjects: []runtime.Object{
				getReviewsService(),
				getReviewsDeployment("v1"),
				getReviewsDeployment("v2"),
			},
			object: getCleanUpDeleteExperiment("cleanupdelete", "reviews", "reviews-v1", "reviews-v2"),
			wantState: test.WantAllStates(
				test.CheckExperimentFinished,
				test.CheckExperimentSuccess,
			),
			postHook: test.CheckObjectDeleted(getReviewsDeployment("v1")),
		},
		"greedy-rollforward": testCase{
			mocks: map[string]analtyicsapi.Response{
				"greedy-rollforward": test.GetSuccessMockResponse(),
			},
			initObjects: []runtime.Object{
				getReviewsService(),
				getReviewsDeployment("v1"),
				getReviewsDeployment("v2"),
			},
			object: getGreedyFastKubernetesExperiment("greedy-rollforward", "reviews", "reviews-v1", "reviews-v2", service.GetURL()),
			wantState: test.WantAllStates(
				test.CheckExperimentFinished,
				test.CheckExperimentSuccess,
			),
			wantResults: []runtime.Object{
				getStableDestinationRule("reviews", "greedy-rollforward", "candidate", getReviewsDeployment("v2")),
				getStableVirtualService("reviews", "greedy-rollforward", "candidate"),
			},
		},
		"greedy-rollbackward": testCase{
			mocks: map[string]analtyicsapi.Response{
				"greedy-rollbackward": test.GetFailureMockResponse(),
			},
			initObjects: []runtime.Object{
				getReviewsService(),
				getReviewsDeployment("v1"),
				getReviewsDeployment("v2"),
			},
			object: getGreedyFastKubernetesExperiment("greedy-rollbackward", "reviews", "reviews-v1", "reviews-v2", service.GetURL()),
			wantState: test.WantAllStates(
				test.CheckExperimentFinished,
				test.CheckExperimentFailure,
			),
			wantResults: []runtime.Object{
				getStableDestinationRule("reviews", "greedy-rollbackward", "baseline", getReviewsDeployment("v1")),
				getStableVirtualService("reviews", "greedy-rollbackward", "baseline"),
			},
		},
		"reward-rollforward": testCase{
			mocks: map[string]analtyicsapi.Response{
				"reward-rollforward": test.GetSuccessMockResponse(),
			},
			initObjects: []runtime.Object{
				getReviewsService(),
				getReviewsDeployment("v1"),
				getReviewsDeployment("v2"),
			},
			object: getRewardFastKubernetesExperiment("reward-rollforward", "reviews", "reviews-v1", "reviews-v2", service.GetURL()),
			wantState: test.WantAllStates(
				test.CheckExperimentFinished,
				test.CheckExperimentSuccess,
			),
			wantResults: []runtime.Object{
				getStableDestinationRule("reviews", "reward-rollforward", "candidate", getReviewsDeployment("v2")),
				getStableVirtualService("reviews", "reward-rollforward", "candidate"),
			},
		},
		"pauseresume": testCase{
			mocks: map[string]analtyicsapi.Response{
				"pauseresume": test.GetSuccessMockResponse(),
			},
			initObjects: []runtime.Object{
				getReviewsService(),
				getReviewsDeployment("v1"),
				getReviewsDeployment("v2"),
			},
			object: getPauseExperiment("pauseresume", "reviews", "reviews-v1", "reviews-v2", service.GetURL()),
			wantState: test.WantAllStates(
				test.CheckExperimentPause,
			),
			postHook: test.ResumeExperiment(getPauseExperiment("pauseresume", "reviews", "reviews-v1", "reviews-v2", service.GetURL())),
			wantResults: []runtime.Object{
				getStableDestinationRule("reviews", "pauseresume", "candidate", getReviewsDeployment("v2")),
				getStableVirtualService("reviews", "pauseresume", "candidate"),
			},
		},
		"deletebaseline": func(name string) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetSuccessMockResponse(),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsDeployment("v1"),
					getReviewsDeployment("v2"),
				},
				object:    getSlowKubernetesExperiment(name, "reviews", "reviews-v1", "reviews-v2", service.GetURL()),
				wantState: test.CheckServiceFound,
				postHook:  test.DeleteObject(getReviewsDeployment("v1")),
				wantResults: []runtime.Object{
					getStableDestinationRule("reviews", name, "candidate", getReviewsDeployment("v2")),
					getStableVirtualService("reviews", name, "candidate"),
				},
			}
		}("deletebaseline"),
		"duplicate-service": func(name string) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetSuccessMockResponse(),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsDeployment("v1"),
					getReviewsDeployment("v2"),
				},
				preHook:   []test.Hook{test.CreateObject(getDefaultKubernetesExperiment(name, "reviews", "reviews-v1", "reviews-v2"))},
				object:    getDefaultKubernetesExperiment(name+"duplicate", "reviews", "reviews-v1", "reviews-v2"),
				wantState: test.CheckServiceNotFound("TargetsNotFound"),
				wantResults: []runtime.Object{
					getStableDestinationRule("reviews", name, "candidate", getReviewsDeployment("v2")),
					getStableVirtualService("reviews", name, "candidate"),
				},
				finalizers: []test.Hook{
					test.DeleteObject(getDefaultKubernetesExperiment(name+"duplicate", "reviews", "reviews-v1", "reviews-v2")),
				},
			}
		}("duplicate-service"),
		"attach-gateway": func(name string) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetSuccessMockResponse(),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsDeployment("v1"),
					getReviewsDeployment("v2"),
				},
				object: getExperimentWithGateway(name, "reviews", "reviews-v1", "reviews-v2", service.GetURL(), "reviews.com", "gateway-testing"),
				wantState: test.WantAllStates(
					test.CheckExperimentFinished,
					test.CheckExperimentSuccess,
				),
				wantResults: []runtime.Object{
					getStableDestinationRule("reviews", name, "candidate", getReviewsDeployment("v2")),
					getStableVirtualServiceWithGateway("reviews", name, "candidate", "reviews.com", "gateway-testing"),
				},
			}
		}("attach-gateway"),
		"rollforward-service": func(name string) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetSuccessMockResponse(),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsServiceWithVersion("v1"),
					getReviewsServiceWithVersion("v2"),
				},
				object: getFastKubernetesExperimentWithService(name, "reviews", "reviews-v1", "reviews-v2", service.GetURL()),
				wantState: test.WantAllStates(
					test.CheckExperimentFinished,
					test.CheckExperimentSuccess,
				),
				wantResults: []runtime.Object{
					getVirtualServiceWithService("reviews", name, Flags.Namespace, "reviews-v2"),
				},
			}
		}("rollforward-service"),
		"rollbackward-service": func(name string) testCase {
			return testCase{
				mocks: map[string]analtyicsapi.Response{
					name: test.GetFailureMockResponse(),
				},
				initObjects: []runtime.Object{
					getReviewsService(),
					getReviewsServiceWithVersion("v1"),
					getReviewsServiceWithVersion("v2"),
				},
				object: getFastKubernetesExperimentWithService(name, "reviews", "reviews-v1", "reviews-v2", service.GetURL()),
				wantState: test.WantAllStates(
					test.CheckExperimentFinished,
					test.CheckExperimentFailure,
				),
				wantResults: []runtime.Object{
					getVirtualServiceWithService("reviews", name, Flags.Namespace, "reviews-v1"),
				},
			}
		}("rollbackward-service"),
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

func getRatingsService() runtime.Object {
	return test.NewKubernetesService("ratings", Flags.Namespace).
		WithSelector(map[string]string{"app": "ratings"}).
		WithPorts(map[string]int{"http": RatingsPort}).
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

func getRatingsDeployment() runtime.Object {
	labels := map[string]string{
		"app":     "ratings",
		"version": "v1",
	}

	return test.NewKubernetesDeployment("ratings-v1", Flags.Namespace).
		WithLabels(labels).
		WithContainer("ratings", RatingsImage, RatingsPort).
		Build()
}

func getPauseExperiment(name, serviceName, baseline, candidate, analyticsHost string) *iter8v1alpha1.Experiment {
	exp := getFastKubernetesExperiment(name, serviceName, baseline, candidate, analyticsHost)
	exp.Action = iter8v1alpha1.ActionPause
	return exp
}

func getCleanUpDeleteExperiment(name, serviceName, baseline, candidate string) *iter8v1alpha1.Experiment {
	exp := getDefaultKubernetesExperiment(name, serviceName, baseline, candidate)
	exp.Spec.CleanUp = iter8v1alpha1.CleanUpDelete
	return exp
}

func getDefaultKubernetesExperiment(name, serviceName, baseline, candidate string) *iter8v1alpha1.Experiment {
	exp := test.NewExperiment(name, Flags.Namespace).
		WithKubernetesTargetService(serviceName, baseline, candidate).
		Build()

	onesec := "1s"
	one := 1
	exp.Spec.TrafficControl.Interval = &onesec
	exp.Spec.TrafficControl.MaxIterations = &one

	return exp
}

func getExperimentWithExternalReference(name, serviceName, baseline, candidate string) *iter8v1alpha1.Experiment {
	exp := getDefaultKubernetesExperiment(name, serviceName, baseline, candidate)
	exp.Spec.RoutingReference = &corev1.ObjectReference{
		Name:       "reviews-external",
		APIVersion: v1alpha3.SchemeGroupVersion.String(),
		Kind:       "VirtualService",
	}

	return exp
}

func getExperimentWithGateway(name, serviceName, baseline, candidate, analyticsHost, host, gw string) *iter8v1alpha1.Experiment {
	experiment := test.NewExperiment(name, Flags.Namespace).
		WithKubernetesTargetService(serviceName, baseline, candidate).
		WithHostInTargetService(host, gw).
		WithAnalyticsHost(analyticsHost).
		WithDummySuccessCriterion().
		Build()

	onesec := "1s"
	one := 1
	experiment.Spec.TrafficControl.Interval = &onesec
	experiment.Spec.TrafficControl.MaxIterations = &one

	return experiment
}

func getFastKubernetesExperiment(name, serviceName, baseline, candidate, analyticsHost string) *iter8v1alpha1.Experiment {
	experiment := test.NewExperiment(name, Flags.Namespace).
		WithKubernetesTargetService(serviceName, baseline, candidate).
		WithAnalyticsHost(analyticsHost).
		WithDummySuccessCriterion().
		Build()

	onesec := "1s"
	one := 1
	experiment.Spec.TrafficControl.Interval = &onesec
	experiment.Spec.TrafficControl.MaxIterations = &one

	return experiment
}

func getFastKubernetesExperimentWithService(name, serviceName, baseline, candidate, analyticsHost string) *iter8v1alpha1.Experiment {
	experiment := test.NewExperiment(name, Flags.Namespace).
		WithKubernetesTargetService(serviceName, baseline, candidate).
		WithKubernetesTargetServiceKind("Service").
		WithAnalyticsHost(analyticsHost).
		WithDummySuccessCriterion().
		Build()

	onesec := "1s"
	one := 1
	experiment.Spec.TrafficControl.Interval = &onesec
	experiment.Spec.TrafficControl.MaxIterations = &one

	return experiment
}

func getGreedyFastKubernetesExperiment(name, serviceName, baseline, candidate, analyticsHost string) *iter8v1alpha1.Experiment {
	experiment := getFastKubernetesExperiment(name, serviceName, baseline, candidate, analyticsHost)

	greedy := "epsilon_greedy"
	experiment.Spec.TrafficControl.Strategy = &greedy

	return experiment
}

func getRewardFastKubernetesExperiment(name, serviceName, baseline, candidate, analyticsHost string) *iter8v1alpha1.Experiment {
	experiment := getFastKubernetesExperiment(name, serviceName, baseline, candidate, analyticsHost)

	experiment.Spec.Analysis.Reward = &iter8v1alpha1.Reward{
		MetricName: "iter8_latency",
		MinMax: &iter8v1alpha1.MinMax{
			Min: 0,
			Max: 10,
		},
	}

	return experiment
}

func getSlowKubernetesExperiment(name, serviceName, baseline, candidate, analyticsHost string) *iter8v1alpha1.Experiment {
	experiment := test.NewExperiment(name, Flags.Namespace).
		WithKubernetesTargetService(serviceName, baseline, candidate).
		WithAnalyticsHost(analyticsHost).
		WithDummySuccessCriterion().
		Build()

	tensecs := "10s"
	two := 2
	experiment.Spec.TrafficControl.Interval = &tensecs
	experiment.Spec.TrafficControl.MaxIterations = &two

	return experiment
}

func getStableDestinationRule(serviceName, name, subset string, obj runtime.Object) runtime.Object {
	deploy := obj.(*appsv1.Deployment)

	return routing.NewDestinationRule(util.ServiceToFullHostName(serviceName, Flags.Namespace), name, Flags.Namespace).
		WithDeployment(deploy, subset).
		Build()
}

func getStableVirtualService(serviceName, name, subset string) runtime.Object {
	host := util.ServiceToFullHostName(serviceName, Flags.Namespace)
	return routing.NewVirtualService(host, name, Flags.Namespace).
		WithNewStableSet(host, subset).
		InitMeshGateway().
		Build()
}

func getVirtualServiceWithService(serviceName, name, namespace, stableService string) runtime.Object {
	host := util.ServiceToFullHostName(serviceName, Flags.Namespace)
	stableHost := util.ServiceToFullHostName(stableService, Flags.Namespace)
	return routing.NewVirtualService(host, name, namespace).
		WithHosts([]string{host}).
		WithHTTPRoute(&networkingv1alpha3.HTTPRoute{
			Route: []*networkingv1alpha3.HTTPRouteDestination{{
				Destination: &networkingv1alpha3.Destination{
					Host: stableHost,
				},
				Weight: 100,
			},
			},
		}).
		InitMeshGateway().
		Build()
}

func getStableVirtualServiceWithGateway(serviceName, name, subset, host, gw string) runtime.Object {
	internalHost := util.ServiceToFullHostName(serviceName, Flags.Namespace)
	return routing.NewVirtualService(internalHost, name, Flags.Namespace).
		WithNewStableSet(internalHost, subset).
		WithHosts([]string{host}).
		InitMeshGateway().
		WithGateways([]string{gw}).
		Build()
}

func getSampleEdgeVirtualService() runtime.Object {
	return &v1alpha3.VirtualService{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha3.SchemeGroupVersion.String(),
			Kind:       "VirtualService",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "reviews-external",
			Namespace: Flags.Namespace,
		},
		Spec: networkingv1alpha3.VirtualService{
			Hosts:    []string{"reviews.com"},
			Gateways: []string{"reviews-gateway"},
			Http: []*networkingv1alpha3.HTTPRoute{
				{
					Route: []*networkingv1alpha3.HTTPRouteDestination{
						{
							Destination: &networkingv1alpha3.Destination{
								Host: "reviews",
								Port: &networkingv1alpha3.PortSelector{
									Number: 9080,
								},
							},
						},
					},
				},
			},
		},
	}
}

func getSampleStableEdgeVirtualService() runtime.Object {
	vs := getSampleEdgeVirtualService()
	vs.(*v1alpha3.VirtualService).Spec.Http[0].Route[0].Destination.Subset = "stable"
	vs.(*v1alpha3.VirtualService).Spec.Http[0].Route[0].Weight = 100

	return vs
}
