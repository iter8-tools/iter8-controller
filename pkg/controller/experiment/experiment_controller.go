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
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
)

var log = logf.Log.WithName("experiment-controller")

type loggerKeyType string

const (
	KubernetesService      = "v1"
	KnativeServiceV1Alpha1 = "serving.knative.dev/v1alpha1"

	Finalizer = "finalizer.iter8-tools"
	loggerKey = loggerKeyType("logger")
)

// Add creates a new Experiment Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileExperiment{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("experiment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Experiment
	err = c.Watch(&source.Kind{Type: &iter8v1alpha1.Experiment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	p := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if _, ok := e.MetaOld.GetLabels()[experimentLabel]; !ok {
				return false
			}
			return e.ObjectOld != e.ObjectNew
		},
		CreateFunc: func(e event.CreateEvent) bool {
			_, ok := e.Meta.GetLabels()[experimentLabel]
			return ok
		},
	}

	// Watch for Knative services changes
	mapFn := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			experiment := a.Meta.GetLabels()[experimentLabel]
			return []reconcile.Request{
				{NamespacedName: types.NamespacedName{
					Name:      experiment,
					Namespace: a.Meta.GetNamespace(),
				}},
			}
		})

	err = c.Watch(&source.Kind{Type: &servingv1alpha1.Service{}},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: mapFn},
		p)

	if err != nil {
		log.Info("NoKnativeServingWatch", zap.Error(err))
	}

	// Watch for k8s deployment updates
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: mapFn},
		p)

	if err != nil {
		// Just log error.
		log.Info("NoDeployementWatch", zap.Error(err))
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileExperiment{}

// ReconcileExperiment reconciles a Experiment object
type ReconcileExperiment struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Experiment object and makes changes based on the state read
// and what is in the Experiment.Spec
// +kubebuilder:rbac:groups=iter8-tools,resources=experiments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=iter8-tools,resources=experiments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.istio.io,resources=destinationrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=serving.knative.dev,resources=services,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=serving.knative.dev,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=serving.knative.dev,resources=revisions,verbs=get;list;watch
// +kubebuilder:rbac:groups=serving.knative.dev,resources=revisions/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
func (r *ReconcileExperiment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.Background()

	// Fetch the Experiment instance
	instance := &iter8v1alpha1.Experiment{}
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	log := log.WithValues("namespace", instance.Namespace, "name", instance.Name)
	ctx = context.WithValue(ctx, loggerKey, log)

	// Add finalizer to the experiment object
	if err = addFinalizerIfAbsent(ctx, r, instance, Finalizer); err != nil {
		return reconcile.Result{}, err
	}

	// Check whether object has been deleted
	if instance.DeletionTimestamp != nil {
		return r.finalize(ctx, instance)
	}

	// // Stop right here if the experiment is completed.
	completed := instance.Status.GetCondition(iter8v1alpha1.ExperimentConditionExperimentCompleted)
	if completed != nil && completed.Status == corev1.ConditionTrue {
		log.Info("RolloutCompleted", "Use a different name for experiment object to trigger a new experiment", "")
		return reconcile.Result{}, nil
	}

	log.Info("reconciling")

	// TODO: not sure why this is needed
	if instance.Status.LastIncrementTime.IsZero() {
		instance.Status.LastIncrementTime = metav1.NewTime(time.Unix(0, 0))
	}

	if instance.Status.AnalysisState.Raw == nil {
		instance.Status.AnalysisState.Raw = []byte("{}")
	}

	creationts := instance.ObjectMeta.GetCreationTimestamp()
	now := metav1.Now()
	if !creationts.Before(&now) {
		// Delay experiment by 1 sec
		return reconcile.Result{RequeueAfter: time.Second}, nil
	}

	// Update Grafana URL when experiment is created
	if instance.Status.StartTimestamp == "" {
		ts := now.UTC().UnixNano() / int64(time.Millisecond)
		instance.Status.StartTimestamp = strconv.FormatInt(ts, 10)
		updateGrafanaURL(instance, getServiceNamespace(instance))
	}

	instance.Status.InitializeConditions()

	// Sync metric definitions from the config map
	metricsSycned := instance.Status.GetCondition(iter8v1alpha1.ExperimentConditionMetricsSynced)
	if metricsSycned == nil || metricsSycned.Status != corev1.ConditionTrue {
		if err := readMetrics(ctx, r, instance); err != nil {
			instance.Status.MarkMetricsSyncedError("UnableToSyncWithMetricsConfigMap", "%s", err)
			log.Error(err, "UnableToSyncWithMetricsConfigMap")
			r.Status().Update(ctx, instance)
			return reconcile.Result{}, err
		}
		instance.Status.MarkMetricsSynced()
	}

	apiVersion := instance.Spec.TargetService.APIVersion

	switch apiVersion {
	case KubernetesService:
		return r.syncKubernetes(ctx, instance)
	case KnativeServiceV1Alpha1:
		return r.syncKnative(ctx, instance)
	default:
		instance.Status.MarkTargetsError("UnsupportedAPIVersion", "%s", apiVersion)
		err := r.Status().Update(ctx, instance)
		return reconcile.Result{}, err
	}
}

func (r *ReconcileExperiment) finalize(context context.Context, instance *iter8v1alpha1.Experiment) (reconcile.Result, error) {
	log := Logger(context)
	log.Info("finalizing")

	apiVersion := instance.Spec.TargetService.APIVersion
	switch apiVersion {
	case KubernetesService:
		return r.finalizeIstio(context, instance)
	case KnativeServiceV1Alpha1:
		return r.finalizeKnative(context, instance)
	}

	return reconcile.Result{}, removeFinalizer(context, r, instance, Finalizer)
}

// Logger gets the logger from the context.
func Logger(ctx context.Context) logr.Logger {
	return ctx.Value(loggerKey).(logr.Logger)
}
