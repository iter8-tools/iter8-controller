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
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	iter8v1alpha2 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha2"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/util"
)

func (r *ReconcileExperiment) syncKubernetes(context context.Context, instance *iter8v1alpha2.Experiment) (reconcile.Result, error) {
	log := util.Logger(context)

	// check routing rules for this experiment
	// initialization will be defered until targets are detected
	// TODO: consider renaming the method checkOrInitRules
	err := r.checkOrInitRules(context, instance)
	if err != nil {
		return r.endRequest(context, instance)
	}

	// detect targets of this experiment if necessary
	if r.toDetectTargets(context, instance) {
		found, err := r.detectTargets(context, instance)
		if err != nil || !found {
			return r.endRequest(context, instance)
		}
	}

	if r.toUpdate(context, instance) {
		err := r.updateIteration(context, instance)
		if err != nil {
			return r.endRequest(context, instance)
		}
	}

	// complete experiment if required
	if r.toComplete(context, instance) {
		err := r.completeExperiment(context, instance)
		if err != nil {
			// retry
			return reconcile.Result{Requeue: true}, nil
		}
		return r.endRequest(context, instance)
	}

	// requeue for next iteration
	if r.hasProgress() {
		interval, _ := instance.Spec.GetInterval()
		r.endRequest(context, instance)
		log.Info("Requeue for next iteration", "interval", interval)
		return reconcile.Result{RequeueAfter: interval}, nil
	}

	log.Info("Request not processed")
	return r.endRequest(context, instance)
}

func (r *ReconcileExperiment) finalizeIstio(context context.Context, instance *iter8v1alpha2.Experiment) (reconcile.Result, error) {
	if !instance.Status.ExperimentCompleted() {
		instance.Spec.ManualOverride = &iter8v1alpha2.ManualOverride{
			Action: iter8v1alpha2.ActionTerminate,
		}
		if _, err := r.syncKubernetes(context, instance); err != nil {
			util.Logger(context).Error(err, "Fail to execute finalize sync process")
		}
		r.iter8Cache.RemoveExperiment(instance)
	}

	return reconcile.Result{}, removeFinalizer(context, r, instance, Finalizer)
}

func (r *ReconcileExperiment) toDetectTargets(context context.Context, instance *iter8v1alpha2.Experiment) bool {
	// Skip targets check if termination request issued from cache or
	// targets have been marked found during the experiment
	// refesh command force to do a new check
	if experimentAbstract(context) != nil && experimentAbstract(context).Terminate() {
		return false
	}

	if instance.Status.TargetsFound() && !r.needRefresh() {
		return false
	}

	return true
}

func (r *ReconcileExperiment) toUpdate(context context.Context, instance *iter8v1alpha2.Experiment) bool {
	if instance.Spec.Terminate() {
		return false
	}

	now := time.Now()
	// traffic := instance.Spec.TrafficControl
	interval, _ := instance.Spec.GetInterval()

	return instance.Status.LastUpdateTime == nil || now.After(instance.Status.LastUpdateTime.Add(interval))
}

func (r *ReconcileExperiment) toComplete(context context.Context, instance *iter8v1alpha2.Experiment) bool {
	return instance.Spec.GetMaxIterations() < *instance.Status.CurrentIteration ||
		instance.Spec.Terminate() || experimentAbstract(context).Terminate()
}

func (r *ReconcileExperiment) endRequest(context context.Context, instance *iter8v1alpha2.Experiment) (reconcile.Result, error) {
	if r.needStatusUpdate() {
		if err := r.Status().Update(context, instance); err != nil && !validUpdateErr(err) {
			log.Error(err, "Fail to update status")
		}
	}
	return reconcile.Result{}, nil
}
