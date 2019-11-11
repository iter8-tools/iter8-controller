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
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	servingalpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	servingbeta1 "github.com/knative/serving/pkg/apis/serving/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" //
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/iter8-tools/iter8-controller/pkg/analytics/checkandincrement"
	"github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
	"github.com/iter8-tools/iter8-controller/test"
)

// Flags holds the command line flags or defaults for settings in the user's environment.
// See EnvironmentFlags for a list of supported fields.
var Flags = initializeFlags()
var compoptions = []cmp.Option{
	cmpopts.IgnoreTypes(metav1.TypeMeta{}, metav1.ObjectMeta{}, metav1.Time{},
		corev1.ResourceRequirements{}, servingalpha1.ServiceStatus{}),
	cmpopts.IgnoreFields(servingbeta1.RevisionSpec{}, "TimeoutSeconds"),
}

// EnvironmentFlags define the flags that are needed to run the e2e tests.
type EnvironmentFlags struct {
	Namespace string // K8s namespace (blank by default, to be overwritten by test suite)
}

func initializeFlags() *EnvironmentFlags {
	var f EnvironmentFlags

	flag.StringVar(&f.Namespace, "namespace", "",
		"Provide the namespace you would like to use for these tests.")

	return &f
}

// GetClient returns generic Kube client configure with experiment scheme
func GetClient() client.Client {
	// Get a config to talk to the apiserver

	cfg, err := config.GetConfig()
	if err != nil {
		panic(fmt.Errorf("unable to set up client config (%v)", err))
	}

	sch := scheme.Scheme
	if err := v1alpha1.AddToScheme(sch); err != nil {
		panic(fmt.Errorf("unable to add scheme (%v)", err))
	}
	if err := servingalpha1.AddToScheme(sch); err != nil {
		panic(fmt.Errorf("unable to add scheme (%v)", err))
	}
	if err := servingbeta1.AddToScheme(sch); err != nil {
		panic(fmt.Errorf("unable to add scheme (%v)", err))
	}
	if err := istiov1alpha3.AddToScheme(sch); err != nil {
		panic(fmt.Errorf("unable to add scheme (%v)", err))
	}

	cl, err := client.New(cfg, client.Options{Scheme: sch})
	if err != nil {
		panic(fmt.Errorf("unable to set up client config (%v)", err))
	}

	return cl
}

type testCase struct {
	mocks map[string]checkandincrement.Response

	// Initial set of objects,
	initObjects []runtime.Object

	// Pre-reconcile hook
	preHook test.Hook

	// Object to reconcile
	object runtime.Object

	// Wait for object state before checking results
	wantState test.InStateFunc

	// Objects to freeze and should not change after postHook
	frozenObjects []runtime.Object

	// Post-reconcile hook
	postHook test.Hook

	// Want the results
	wantResults []runtime.Object

	// Finalize the test case
	finalizers []test.Hook
}

func (tc *testCase) createInitObjects(ctx context.Context, cl client.Client) error {
	for _, obj := range tc.initObjects {
		if _, err := getObject(ctx, cl, obj); err != nil {
			err = cl.Create(ctx, obj)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (tc *testCase) createObject(ctx context.Context, cl client.Client) error {
	if tc.object == nil {
		return nil
	}
	if _, err := getObject(ctx, cl, tc.object); err == nil {
		return nil
	}
	return cl.Create(ctx, tc.object)
}

func (tc *testCase) runPreHook(ctx context.Context, cl client.Client) error {
	if tc.preHook == nil {
		return nil
	}
	return tc.preHook(ctx, cl)
}

func (tc *testCase) runPostHook(ctx context.Context, cl client.Client) error {
	if tc.postHook == nil {
		return nil
	}

	return tc.postHook(ctx, cl)
}

func (tc *testCase) checkHasResults(ctx context.Context, cl client.Client) error {
	for _, result := range tc.wantResults {
		retries := 10
		for {
			obj, err := getObject(ctx, cl, result)
			if err != nil {
				return err
			}

			if diff := cmp.Diff(result, obj, compoptions...); diff != "" {
				if retries == 0 {
					return fmt.Errorf("unexpected reponse diff (-want, +got) = %v", diff)
				}
			} else {
				break
			}

			time.Sleep(time.Second)

			retries--
		}
	}

	return nil
}

func (tc *testCase) freezeObjects(ctx context.Context, cl client.Client) error {
	if tc.frozenObjects == nil {
		return nil
	}
	for _, obj := range tc.frozenObjects {
		frozen, err := getObject(ctx, cl, obj)
		if err != nil {
			return err
		}

		tc.wantResults = append(tc.wantResults, frozen)
	}
	return nil
}

func (tc *testCase) checkHasState(ctx context.Context, cl client.Client) error {
	if tc.wantState == nil {
		return nil
	}
	return test.WaitForState(ctx, cl, tc.object, tc.wantState)
}

func (tc *testCase) runFinalizers(ctx context.Context, cl client.Client) error {
	if tc.finalizers == nil {
		return nil
	}

	for _, finalizer := range tc.finalizers {
		if err := finalizer(ctx, cl); err != nil {
			return err
		}
	}
	return nil
}

func getObject(ctx context.Context, cl client.Client, obj runtime.Object) (runtime.Object, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	nobj, err := scheme.Scheme.New(obj.GetObjectKind().GroupVersionKind())
	if err != nil {
		return nil, err
	}

	err = cl.Get(ctx, client.ObjectKey{Namespace: accessor.GetNamespace(), Name: accessor.GetName()}, nobj)
	if err != nil {
		return nil, err
	}
	nobj.GetObjectKind().SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
	return nobj, nil
}

func runTestCases(t *testing.T, service *test.AnalyticsService, testCases map[string]testCase) {

	client := GetClient()
	ctx := context.Background()

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			service.Mock = tc.mocks

			if err := tc.createInitObjects(ctx, client); err != nil {
				t.Fatalf("Failed initializing objects %v", err)
			}

			if err := tc.runPreHook(ctx, client); err != nil {
				t.Fatalf("Failed running prehook %v", err)
			}

			if err := tc.createObject(ctx, client); err != nil {
				t.Fatalf("Failed creating object %v", err)
			}

			if err := tc.checkHasState(ctx, client); err != nil {
				t.Fatalf("Failed checking state %v", err)
			}

			if err := tc.freezeObjects(ctx, client); err != nil {
				t.Fatalf("Failed freezing objects %v", err)
			}

			if err := tc.runPostHook(ctx, client); err != nil {
				t.Fatalf("Failed running post hook %v", err)
			}

			if err := tc.checkHasResults(ctx, client); err != nil {
				t.Fatalf("Failed checking results %v", err)
			}

			if err := tc.runFinalizers(ctx, client); err != nil {
				t.Fatalf("Failed running finalizers %v", err)
			}
		})
	}
}
