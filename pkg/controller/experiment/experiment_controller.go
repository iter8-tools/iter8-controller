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
	"fmt"

	istioclient "istio.io/client-go/pkg/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	metricsv1alpha2 "github.com/iter8-tools/iter8/pkg/analytics/metrics/v1alpha2"
	iter8v1alpha2 "github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
	"github.com/iter8-tools/iter8/pkg/controller/experiment/adapter"
	"github.com/iter8-tools/iter8/pkg/controller/experiment/routing"
	"github.com/iter8-tools/iter8/pkg/controller/experiment/routing/router"
	"github.com/iter8-tools/iter8/pkg/controller/experiment/util"
	"github.com/iter8-tools/iter8/pkg/grafana"
	iter8notifier "github.com/iter8-tools/iter8/pkg/notifier"
)

var log = logf.Log.WithName("experiment-controller")

const (
	Iter8Controller = "iter8"
	Finalizer       = "finalizer.iter8-tools"
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
func newReconciler(mgr manager.Manager) (*ReconcileExperiment, error) {
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

	k8sCache := mgr.GetCache()

	// Set up notifier configmap handler
	nc := iter8notifier.NewNotificationCenter(log)
	err = nc.RegisterHandler(k8sCache)
	if err != nil {
		log.Error(err, "Failed to register notifier config handlers")
		return nil, err
	}

	grafanaConfig := grafana.NewConfigStore(log, mgr.GetClient())

	iter8Adapter := adapter.New(log)

	return &ReconcileExperiment{
		Client:             mgr.GetClient(),
		istioClient:        ic,
		scheme:             mgr.GetScheme(),
		eventRecorder:      mgr.GetEventRecorderFor(Iter8Controller),
		notificationCenter: nc,
		iter8Adapter:       iter8Adapter,
		grafanaConfig:      grafanaConfig,
	}, nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileExperiment) error {
	// Create a new controller
	c, err := controller.New("experiment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	deploymentPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			name, namespace := e.Meta.GetName(), e.Meta.GetNamespace()
			ok := r.iter8Adapter.MarkDeploymentDetected(name, namespace)
			if !ok {
				return false
			}

			log.Info("DeploymentDetected", "", name+"."+namespace)

			return true
		},
		UpdateFunc: func(event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			name, namespace := e.Meta.GetName(), e.Meta.GetNamespace()
			ok := r.iter8Adapter.MarkDeploymentDeleted(name, namespace)
			if !ok {
				return false
			}

			log.Info("DeploymentDeleted", "", name+"."+namespace)

			return true
		},
	}

	deploymentToExperiment := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			name, namespace := a.Meta.GetName(), a.Meta.GetNamespace()
			experimentName, experimentNamespace, ok := r.iter8Adapter.DeploymentToExperiment(name, namespace)
			if !ok {
				return nil
			}
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      experimentName,
						Namespace: experimentNamespace,
					},
				},
			}
		},
	)

	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: deploymentToExperiment},
		deploymentPredicate)

	servicePredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			name, namespace := e.Meta.GetName(), e.Meta.GetNamespace()
			ok := r.iter8Adapter.MarkServiceDetected(name, namespace)
			if !ok {
				return false
			}

			log.Info("ServiceDetected", "", name+"."+namespace)

			return true
		},
		UpdateFunc: func(event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			name, namespace := e.Meta.GetName(), e.Meta.GetNamespace()
			ok := r.iter8Adapter.MarkServiceDeleted(name, namespace)
			if !ok {
				return false
			}

			log.Info("ServiceDeleted", "", name+"."+namespace)
			return true
		},
	}

	serviceToExperiment := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			name, namespace := a.Meta.GetName(), a.Meta.GetNamespace()
			experimentName, experimentNamespace, ok := r.iter8Adapter.ServiceToExperiment(name, namespace)
			if !ok {
				return nil
			}
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      experimentName,
						Namespace: experimentNamespace,
					},
				},
			}
		},
	)

	err = c.Watch(&source.Kind{Type: &corev1.Service{}},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: serviceToExperiment},
		servicePredicate)

	// Watch for changes to Experiment
	err = c.Watch(&source.Kind{Type: &iter8v1alpha2.Experiment{}}, &handler.EnqueueRequestForObject{},
		// Ignore status update event
		predicate.GenerationChangedPredicate{},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldInstance, _ := e.ObjectOld.(*iter8v1alpha2.Experiment)
				newInstance, _ := e.ObjectNew.(*iter8v1alpha2.Experiment)
				// Ignore finalizer update
				if len(oldInstance.Finalizers) == 0 && len(newInstance.Finalizers) > 0 {
					log.Info("UpdateRequestDetected", "FinalizerChanged", "Reject")
					return false
				}

				// Ignore event of revert changes in action field
				if oldInstance.Spec.ManualOverride != nil && newInstance.Spec.ManualOverride == nil {
					log.Info("UpdateRequestDetected", "ManualOverride", "Reject")
					return false
				}

				// Ignore event of metrics load
				if oldInstance.Spec.Metrics == nil && newInstance.Spec.Metrics != nil {
					log.Info("UpdateRequestDetected", "MetrcisLoad", "Reject")
					return false
				}

				return true
			},
		})
	if err != nil {
		return err
	}

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
	iter8Adapter       adapter.Interface
	grafanaConfig      grafana.Interface

	router router.Interface
	interState
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
	instance := &iter8v1alpha2.Experiment{}
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
	ctx = context.WithValue(ctx, util.LoggerKey, log)

	// Init metadata of experiment instance
	if instance.Status.InitTimestamp == nil {
		instance.InitStatus()
		if err := instance.Spec.Service.Validate(); err != nil {
			r.markTargetsError(ctx, instance, "Invalid service spec: %v", err)
			return r.endRequest(ctx, instance)
		}

		if err := r.Status().Update(ctx, instance); err != nil && !validUpdateErr(err) {
			log.Error(err, "Failed to update status")
			return reconcile.Result{}, nil
		}
	}

	// Add finalizer to the experiment object
	if err = addFinalizerIfAbsent(ctx, r, instance, Finalizer); err != nil && !validUpdateErr(err) {
		return reconcile.Result{}, err
	}

	// Check whether object has been deleted
	if instance.DeletionTimestamp != nil {
		return r.finalize(ctx, instance)
	}

	if instance.Status.ExperimentCompleted() {
		log.Info("NotToProceed", "phase", instance.Status.Phase)
		return reconcile.Result{}, nil
	}

	ctx, err = r.iter8Adapter.RegisterExperiment(ctx, instance)
	if err != nil {
		r.markTargetsError(ctx, instance, "%v", err)
		return r.endRequest(ctx, instance)
	}
	ctx = r.syncExperiment(ctx, instance)

	if err := r.proceed(ctx, instance); err != nil {
		log.Info("NotToProceed", "status", err.Error())
		return r.endRequest(ctx, instance)
	}

	if err := r.syncMetrics(ctx, instance); err != nil {
		return r.endRequest(ctx, instance)
	}

	return r.syncKubernetes(ctx, instance)
}

func (r *ReconcileExperiment) syncMetrics(ctx context.Context, instance *iter8v1alpha2.Experiment) error {
	if instance.Spec.Criteria == nil {
		return nil
	}
	// Sync metric definitions from the config map
	if !instance.Status.MetricsSynced() {
		if err := metricsv1alpha2.Read(ctx, r, instance); err != nil && !validUpdateErr(err) {
			r.markSyncMetricsError(ctx, instance, "Fail to read metrics: %v", err)

			if err := r.Status().Update(ctx, instance); err != nil && !validUpdateErr(err) {
				log.Error(err, "Fail to update status")
				// TODO: need a better way of handling this error
				return err
			}

			return err
		}
		if err := r.Update(ctx, instance); err != nil && !validUpdateErr(err) {
			log.Error(err, "Fail to update instance")
			return err
		}
		r.markSyncMetrics(ctx, instance, "")
	}

	return nil
}

func addFinalizerIfAbsent(context context.Context, c client.Client, instance *iter8v1alpha2.Experiment, fName string) (err error) {
	for _, finalizer := range instance.ObjectMeta.GetFinalizers() {
		if finalizer == fName {
			return
		}
	}

	instance.SetFinalizers(append(instance.GetFinalizers(), Finalizer))
	if err = c.Update(context, instance); err != nil {
		util.Logger(context).Info("setting finalizer failed. (retrying)", "error", err)
	}

	return
}

func removeFinalizer(context context.Context, c client.Client, instance *iter8v1alpha2.Experiment, fName string) (err error) {
	finalizers := make([]string, 0)
	for _, f := range instance.GetFinalizers() {
		if f != fName {
			finalizers = append(finalizers, f)
		}
	}
	instance.SetFinalizers(finalizers)
	if err = c.Update(context, instance); err != nil {
		util.Logger(context).Info("setting finalizer failed. (retrying)", "error", err)
	}

	util.Logger(context).Info("FinalizerRemoved")
	return
}

func (r *ReconcileExperiment) syncExperiment(context context.Context, instance *iter8v1alpha2.Experiment) context.Context {
	r.initState()

	eas := experimentAction(context)
	if eas != nil && (eas.Refresh() || eas.Resume()) {
		r.markRefresh()
	}

	context = r.injectClients(context)
	r.router = routing.GetRouter(context, instance)

	return context
}

// proceed determines whether reconciliation of experiment should continue or not
// refresh/terminate-command > pause-action >resume-action
func (r *ReconcileExperiment) proceed(context context.Context, instance *iter8v1alpha2.Experiment) (err error) {
	// proceed
	if r.needRefresh() || instance.Spec.Terminate() {
		return
	}

	// pause experiment
	if instance.Spec.Pause() {
		util.Logger(context).Info("processing pause")
		instance.Spec.ManualOverride = nil
		if err := r.Update(context, instance); err != nil && !validUpdateErr(err) {
			log.Error(err, "fail to update instance")
			return err
		}
		r.markActionPause(context, instance, "")
		err = fmt.Errorf("phase: %v, action: %s", instance.Status.Phase, instance.Spec.GetAction())
		return err
	}

	// resume experiment
	if instance.Spec.Resume() {
		instance.Spec.ManualOverride = nil
		if err := r.Update(context, instance); err != nil && !validUpdateErr(err) {
			log.Error(err, "fail to update instance")
			return err
		}
		r.markActionResume(context, instance, "")
		return
	}

	if instance.Status.Phase == iter8v1alpha2.PhasePause {
		err = fmt.Errorf("experiment paused")
	}

	return
}
