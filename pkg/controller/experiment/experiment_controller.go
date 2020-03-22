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
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	runtime "k8s.io/apimachinery/pkg/runtime"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1 "k8s.io/api/core/v1"

	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
	iter8notifier "github.com/iter8-tools/iter8-controller/pkg/notifier"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
)

var log = logf.Log.WithName("experiment-controller")

type loggerKeyType string

const (
	KubernetesService      = "v1"
	KnativeServiceV1Alpha1 = "serving.knative.dev/v1alpha1"

	Iter8Controller = "iter8-controller"
	Finalizer       = "finalizer.iter8-tools"
	loggerKey       = loggerKeyType("logger")
)

// Add creates a new Experiment Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}
	return add(mgr, r)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) (reconcile.Reconciler, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to get client config")
		return nil, err
	}

	ic, err := istioclient.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "Failed to create istio client")
		return nil, err
	}

	nc := iter8notifier.NewNotificationCenter(log)

	cm := &corev1.ConfigMap{}
	configmapInformer, err := mgr.GetCache().GetInformer(cm)
	if err != nil {
		return nil, err
	}

	configmapInformer.AddEventHandler(toolscache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			cm := obj.(*corev1.ConfigMap)
			if cm.GetName() == iter8notifier.ConfigMapName && cm.GetNamespace() == Iter8Namespace {
				return true
			}

			return false
		},
		Handler: toolscache.ResourceEventHandlerFuncs{
			AddFunc: iter8notifier.UpdateConfigFromConfigmap(nc),
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldCm, newCm := oldObj.(*corev1.ConfigMap), newObj.(*corev1.ConfigMap)
				if !reflect.DeepEqual(oldCm.Data, newCm.Data) {
					iter8notifier.UpdateConfigFromConfigmap(nc)(newObj)
				}
			},
			DeleteFunc: iter8notifier.RemoveNotifiers(nc),
		},
	})

	return &ReconcileExperiment{
		Client:             mgr.GetClient(),
		istioClient:        ic,
		scheme:             mgr.GetScheme(),
		eventRecorder:      mgr.GetEventRecorderFor(Iter8Controller),
		notificationCenter: nc,
	}, nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("experiment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Experiment
	// Ignore status update event
	err = c.Watch(&source.Kind{Type: &iter8v1alpha1.Experiment{}}, &handler.EnqueueRequestForObject{},
		predicate.GenerationChangedPredicate{})
	if err != nil {
		return err
	}

	// Watch for changes to ConfigMap

	// ****** Skip knative logic for now *******
	// p := predicate.Funcs{
	// 	UpdateFunc: func(e event.UpdateEvent) bool {
	// 		if _, ok := e.MetaOld.GetLabels()[experimentLabel]; !ok {
	// 			return false
	// 		}
	// 		return e.ObjectOld != e.ObjectNew
	// 	},
	// 	CreateFunc: func(e event.CreateEvent) bool {
	// 		_, ok := e.Meta.GetLabels()[experimentLabel]
	// 		return ok
	// 	},
	// }

	// Watch for Knative services changes
	// mapFn := handler.ToRequestsFunc(
	// 	func(a handler.MapObject) []reconcile.Request {
	// 		experiment := a.Meta.GetLabels()[experimentLabel]
	// 		return []reconcile.Request{
	// 			{NamespacedName: types.NamespacedName{
	// 				Name:      experiment,
	// 				Namespace: a.Meta.GetNamespace(),
	// 			}},
	// 		}
	// 	})

	// err = c.Watch(&source.Kind{Type: &servingv1alpha1.Service{}},
	// 	&handler.EnqueueRequestsFromMapFunc{ToRequests: mapFn},
	// 	p)

	// if err != nil {
	// 	log.Info("NoKnativeServingWatch", zap.Error(err))
	// }
	// ****** Skip knative logic for now *******
	return nil
}

var _ reconcile.Reconciler = &ReconcileExperiment{}

// ReconcileExperiment reconciles a Experiment object
type ReconcileExperiment struct {
	client.Client
	scheme             *runtime.Scheme
	eventRecorder      record.EventRecorder
	notificationCenter *iter8notifier.NotificationCenter
	istioClient        istioclient.Interface

	targets *Targets
	rules   *IstioRoutingRules
}

// Reconcile reads that state of the cluster for a Experiment object and makes changes based on the state read
// and what is in the Experiment.Spec
// +kubebuilder:rbac:groups=iter8.tools,resources=experiments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=iter8.tools,resources=experiments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.istio.io,resources=destinationrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=serving.knative.dev,resources=services,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=serving.knative.dev,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=serving.knative.dev,resources=revisions,verbs=get;list;watch
// +kubebuilder:rbac:groups=serving.knative.dev,resources=revisions/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch;delete
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

	if instance.Status.CreateTimestamp == 0 {
		instance.Status.Init()
	}

	// Sync metric definitions from the config map
	metricsSycned := instance.Status.GetCondition(iter8v1alpha1.ExperimentConditionMetricsSynced)
	if metricsSycned == nil || metricsSycned.Status != corev1.ConditionTrue {
		if err := readMetrics(ctx, r, instance); err != nil && !validUpdateErr(err) {
			r.MarkSyncMetricsError(ctx, instance, "Fail to read metrics: %v", err)

			if err := r.Status().Update(ctx, instance); err != nil && !validUpdateErr(err) {
				log.Info("Fail to update status: %v", err)
				// End experiment
				return reconcile.Result{}, nil
			}

			log.Info("Retry in 5s")
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}
		r.MarkSyncMetrics(ctx, instance)
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
