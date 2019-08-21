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
	"encoding/json"
	"fmt"
	"time"

	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/iter8-tools/iter8-controller/pkg/analytics/checkandincrement"
	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
)

func (r *ReconcileExperiment) syncKnative(context context.Context, instance *iter8v1alpha1.Experiment) (reconcile.Result, error) {
	log := Logger(context)

	// Get Knative service
	serviceName := instance.Spec.TargetService.Name
	serviceNamespace := instance.Spec.TargetService.Namespace
	if serviceNamespace == "" {
		serviceNamespace = instance.Namespace
	}

	kservice := &servingv1alpha1.Service{}
	err := r.Get(context, types.NamespacedName{Name: serviceName, Namespace: serviceNamespace}, kservice)
	if err != nil {
		instance.Status.MarkTargetsError("ServiceNotFound", "")
		err = r.Status().Update(context, instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	if kservice.Spec.Template == nil {
		instance.Status.MarkTargetsError("MissingTemplate", "")
		err = r.Status().Update(context, instance)
		return reconcile.Result{}, err
	}

	// link service to this experiment. Only one experiment can control a service
	labels := kservice.GetLabels()
	if experiment, found := labels[experimentLabel]; found && experiment != instance.GetName() {
		instance.Status.MarkTargetsError("ExistingExperiment", "service is already controlled by %v", experiment)
		err = r.Status().Update(context, instance)
		return reconcile.Result{}, err
	}

	if labels == nil {
		labels = make(map[string]string)
	}

	if _, ok := labels[experimentLabel]; !ok {
		labels[experimentLabel] = instance.GetName()
		kservice.SetLabels(labels)
		if err = r.Update(context, kservice); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Check the experiment targets existing traffic targets
	ksvctraffic := kservice.Spec.Traffic
	if ksvctraffic == nil {
		instance.Status.MarkTargetsError("MissingTraffic", "")
		err = r.Status().Update(context, instance)
		return reconcile.Result{}, err
	}

	baseline := instance.Spec.TargetService.Baseline
	baselineTraffic := getTrafficByName(kservice, baseline)
	candidate := instance.Spec.TargetService.Candidate
	candidateTraffic := getTrafficByName(kservice, candidate)

	if baselineTraffic == nil {
		instance.Status.MarkTargetsError("MissingBaselineRevision", "%s", baseline)
		instance.Status.TrafficSplit.Baseline = 0
		if candidateTraffic == nil {
			instance.Status.TrafficSplit.Candidate = 0
		} else {
			instance.Status.TrafficSplit.Candidate = candidateTraffic.Percent
		}
		err = r.Status().Update(context, instance)
		return reconcile.Result{}, err
	}

	if candidateTraffic == nil {
		instance.Status.MarkTargetsError("MissingCandidateRevision", "%s", candidate)
		instance.Status.TrafficSplit.Baseline = baselineTraffic.Percent
		instance.Status.TrafficSplit.Candidate = 0
		err = r.Status().Update(context, instance)
		return reconcile.Result{}, err
	}

	instance.Status.MarkTargetsFound()

	traffic := instance.Spec.TrafficControl
	now := time.Now()
	interval, _ := traffic.GetIntervalDuration() // TODO: admissioncontrollervalidation

	// check experiment is finished
	if traffic.GetMaxIterations() <= instance.Status.CurrentIteration ||
		instance.Spec.Assessment != iter8v1alpha1.AssessmentNull {

		update := false
		if experimentSucceeded(instance) {
			log.Info("Experiment completed with success", "onsuccess", traffic.GetOnSuccess())
			// experiment is successful
			msg := ""
			switch traffic.GetOnSuccess() {
			case "baseline":
				if candidateTraffic.Percent != 0 {
					candidateTraffic.Percent = 0
					update = true
				}
				if baselineTraffic.Percent != 100 {
					baselineTraffic.Percent = 100
					update = true
				}
				msg = "AllToBaseline"
				instance.Status.TrafficSplit.Baseline = 100
				instance.Status.TrafficSplit.Candidate = 0
			case "candidate":
				if candidateTraffic.Percent != 100 {
					candidateTraffic.Percent = 100
					update = true
				}
				if baselineTraffic.Percent != 0 {
					baselineTraffic.Percent = 0
					update = true
				}
				msg = "AllToCandidate"
				instance.Status.TrafficSplit.Baseline = 0
				instance.Status.TrafficSplit.Candidate = 100
			case "both":
				msg = "KeepOnBothVersions"
			}
			markExperimentSuccessStatus(instance, msg)
		} else {
			log.Info("Experiment completed with failure")

			markExperimentFailureStatus(instance, "AllToBaseline")

			// Switch traffic back to baseline
			if candidateTraffic.Percent != 0 {
				candidateTraffic.Percent = 0
				update = true
			}
			if baselineTraffic.Percent != 100 {
				baselineTraffic.Percent = 100
				update = true
			}
		}

		labels := kservice.GetLabels()
		_, has := labels[experimentLabel]
		if has {
			delete(labels, experimentLabel)
		}

		if has || update {
			err := r.Update(context, kservice)
			if err != nil {
				return reconcile.Result{}, err // retry
			}
		}

		markExperimentCompleted(instance)
		instance.Status.TrafficSplit.Baseline = baselineTraffic.Percent
		instance.Status.TrafficSplit.Candidate = candidateTraffic.Percent
		err = r.Status().Update(context, instance)
		return reconcile.Result{}, err
	}

	// Check if traffic should be updated.
	if now.After(instance.Status.LastIncrementTime.Add(interval)) {
		log.Info("process iteration.")

		newRolloutPercent := float64(candidateTraffic.Percent)

		switch getStrategy(instance) {
		case "increment_without_check":
			newRolloutPercent += traffic.GetStepSize()
		case "check_and_increment":
			// Get underlying k8s services
			// TODO: should just get the service name. See issue #83
			baselineService, err := r.getServiceForRevision(context, kservice, baselineTraffic.RevisionName)
			if err != nil {
				// TODO: maybe we want another condition
				instance.Status.MarkTargetsError("MissingCoreService", "%v", err)
				err = r.Status().Update(context, instance)
				return reconcile.Result{}, err
			}
			candidateService, err := r.getServiceForRevision(context, kservice, candidateTraffic.RevisionName)
			if err != nil {
				// TODO: maybe we want another condition
				instance.Status.MarkTargetsError("MissingCoreService", "%v", err)
				err = r.Status().Update(context, instance)
				return reconcile.Result{}, err
			}

			// Get latest analysis
			payload, err := MakeRequest(instance, baselineService, candidateService)
			if err != nil {
				instance.Status.MarkAnalyticsServiceError("CanNotComposePayload", "%v", err)
				log.Error(err, "CanNotComposePayload")
				r.Status().Update(context, instance)
				return reconcile.Result{RequeueAfter: 5 * time.Second}, err
			}
			response, err := checkandincrement.Invoke(log, instance.Spec.Analysis.GetServiceEndpoint(), payload)
			if err != nil {
				instance.Status.MarkAnalyticsServiceError("ErrorAnalytics", "%v", err)
				err = r.Status().Update(context, instance)
				return reconcile.Result{RequeueAfter: 5 * time.Second}, err
			}

			if response.Assessment.Summary.AbortExperiment {
				log.Info("ExperimentAborted. Rollback to Baseline.")
				if candidateTraffic.Percent != 0 || baselineTraffic.Percent != 100 {
					baselineTraffic.Percent = 100
					candidateTraffic.Percent = 0
					err := r.Update(context, kservice)
					if err != nil {
						return reconcile.Result{}, err // retry
					}
				}

				instance.Status.TrafficSplit.Baseline = 100
				instance.Status.TrafficSplit.Candidate = 0

				markExperimentCompleted(instance)
				instance.Status.MarkExperimentFailed("Aborted, Traffic: AllToBaseline.", "")
				err := r.Update(context, instance)
				if err != nil {
					return reconcile.Result{}, err // retry
				}
			}

			baselineTraffic := response.Baseline.TrafficPercentage
			candidateTraffic := response.Canary.TrafficPercentage
			log.Info("NewTraffic", "baseline", baselineTraffic, "candidate", candidateTraffic)
			newRolloutPercent = candidateTraffic

			if response.LastState == nil {
				instance.Status.AnalysisState.Raw = []byte("{}")
			} else {
				lastState, err := json.Marshal(response.LastState)
				if err != nil {
					instance.Status.MarkAnalyticsServiceError("ErrorAnalyticsResponse", "%v", err)
					err = r.Status().Update(context, instance)
					return reconcile.Result{RequeueAfter: 5 * time.Second}, err
				}
				instance.Status.AnalysisState = runtime.RawExtension{Raw: lastState}
			}
			instance.Status.AssessmentSummary = response.Assessment.Summary
		}

		// Set traffic percentable on all routes
		needUpdate := false
		for i := range ksvctraffic {
			target := &ksvctraffic[i]
			if target.RevisionName == baseline {
				if target.Percent != 100-int(newRolloutPercent) {
					target.Percent = 100 - int(newRolloutPercent)
					needUpdate = true
				}
			} else if target.RevisionName == candidate {
				if target.Percent != int(newRolloutPercent) {
					target.Percent = int(newRolloutPercent)
					needUpdate = true
				}
			} else {
				if target.Percent != 0 {
					target.Percent = 0
					needUpdate = true
				}
			}
		}
		if needUpdate {
			log.Info("update traffic", "rolloutPercent", newRolloutPercent)

			err = r.Update(context, kservice) // TODO: patch?
			if err != nil {
				// TODO: the analysis service will be called again upon retry. Maybe we do want that.
				return reconcile.Result{}, err
			}
		}

		instance.Status.CurrentIteration++
		instance.Status.LastIncrementTime = metav1.NewTime(now)
	}

	instance.Status.MarkExperimentNotCompleted(fmt.Sprintf("Iteration %d Completed", instance.Status.CurrentIteration), "")
	instance.Status.TrafficSplit.Baseline = baselineTraffic.Percent
	instance.Status.TrafficSplit.Candidate = candidateTraffic.Percent
	err = r.Status().Update(context, instance)
	return reconcile.Result{RequeueAfter: interval}, err
}

func getTrafficByName(service *servingv1alpha1.Service, name string) *servingv1alpha1.TrafficTarget {
	for i := range service.Spec.Traffic {
		traffic := &service.Spec.Traffic[i]
		if traffic.RevisionName == name {
			return traffic
		}
	}
	return nil
}

func (r *ReconcileExperiment) getServiceForRevision(context context.Context, ksvc *servingv1alpha1.Service, revisionName string) (*corev1.Service, error) {
	revision := &servingv1alpha1.Revision{}
	err := r.Get(context, types.NamespacedName{Name: revisionName, Namespace: ksvc.GetNamespace()}, revision)
	if err != nil {
		return nil, err
	}
	service := &corev1.Service{}
	err = r.Get(context, types.NamespacedName{Name: revision.Status.ServiceName, Namespace: ksvc.GetNamespace()}, service)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func (r *ReconcileExperiment) finalizeKnative(context context.Context, instance *iter8v1alpha1.Experiment) (reconcile.Result, error) {
	completed := instance.Status.GetCondition(iter8v1alpha1.ExperimentConditionExperimentCompleted)
	if completed != nil && completed.Status != corev1.ConditionTrue {
		// Do a rollback

		// Get Knative service
		serviceName := instance.Spec.TargetService.Name
		serviceNamespace := getServiceNamespace(instance)

		kservice := &servingv1alpha1.Service{}
		err := r.Get(context, types.NamespacedName{Name: serviceName, Namespace: serviceNamespace}, kservice)
		if err != nil {
			return reconcile.Result{}, removeFinalizer(context, r, instance, Finalizer)
		}

		// Check the experiment targets existing traffic targets
		ksvctraffic := kservice.Spec.Traffic
		if ksvctraffic == nil {
			return reconcile.Result{}, removeFinalizer(context, r, instance, Finalizer)
		}

		baseline := instance.Spec.TargetService.Baseline
		baselineTraffic := getTrafficByName(kservice, baseline)
		if baselineTraffic == nil {
			return reconcile.Result{}, removeFinalizer(context, r, instance, Finalizer)
		}

		candidate := instance.Spec.TargetService.Candidate
		candidateTraffic := getTrafficByName(kservice, candidate)
		if candidateTraffic == nil {
			return reconcile.Result{}, removeFinalizer(context, r, instance, Finalizer)
		}

		if baselineTraffic.Percent != 100 || candidateTraffic.Percent != 0 {
			baselineTraffic.Percent = 100
			candidateTraffic.Percent = 0

			err = r.Update(context, kservice) // TODO: patch?
			if err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	return reconcile.Result{}, removeFinalizer(context, r, instance, Finalizer)
}
