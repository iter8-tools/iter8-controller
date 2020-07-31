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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	iter8v1alpha2 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha2"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/util"
)

func (r *ReconcileExperiment) markTargetsError(context context.Context, instance *iter8v1alpha2.Experiment,
	messageFormat string, messageA ...interface{}) {
	if updated, reason := instance.Status.MarkTargetsError(messageFormat, messageA...); updated {
		util.Logger(context).Info(reason + ", " + fmt.Sprintf(messageFormat, messageA...))
		r.eventRecorder.Eventf(instance, corev1.EventTypeWarning, reason, messageFormat, messageA...)
		r.notificationCenter.Notify(instance, reason, messageFormat, messageA...)
		r.markStatusUpdate()
	}
}

func (r *ReconcileExperiment) markTargetsFound(context context.Context, instance *iter8v1alpha2.Experiment,
	messageFormat string, messageA ...interface{}) {
	if updated, reason := instance.Status.MarkTargetsFound(messageFormat, messageA...); updated {
		util.Logger(context).Info(reason + ", " + fmt.Sprintf(messageFormat, messageA...))
		r.eventRecorder.Eventf(instance, corev1.EventTypeNormal, reason, messageFormat, messageA...)
		r.notificationCenter.Notify(instance, reason, messageFormat, messageA...)
		r.markStatusUpdate()
	}
}

func (r *ReconcileExperiment) markAnalyticsServiceError(context context.Context, instance *iter8v1alpha2.Experiment,
	messageFormat string, messageA ...interface{}) {
	if updated, reason := instance.Status.MarkAnalyticsServiceError(messageFormat, messageA...); updated {
		util.Logger(context).Info(reason + ", " + fmt.Sprintf(messageFormat, messageA...))
		r.eventRecorder.Eventf(instance, corev1.EventTypeWarning, reason, messageFormat, messageA...)
		r.notificationCenter.Notify(instance, reason, messageFormat, messageA...)
		r.markStatusUpdate()
	}
}

func (r *ReconcileExperiment) markAnalyticsServiceRunning(context context.Context, instance *iter8v1alpha2.Experiment,
	messageFormat string, messageA ...interface{}) {
	if updated, reason := instance.Status.MarkAnalyticsServiceRunning(messageFormat, messageA...); updated {
		util.Logger(context).Info(reason)
		r.eventRecorder.Eventf(instance, corev1.EventTypeNormal, reason, "")
		r.notificationCenter.Notify(instance, reason, "")
		r.markStatusUpdate()
	}
}

func (r *ReconcileExperiment) markIterationUpdate(context context.Context, instance *iter8v1alpha2.Experiment,
	messageFormat string, messageA ...interface{}) {
	if updated, reason := instance.Status.MarkIterationUpdate(messageFormat, messageA...); updated {
		util.Logger(context).Info(reason + ", " + fmt.Sprintf(messageFormat, messageA...))
		r.eventRecorder.Eventf(instance, corev1.EventTypeNormal, reason, messageFormat, messageA...)
		r.notificationCenter.Notify(instance, reason, messageFormat, messageA...)
		r.markStatusUpdate()
		r.markProgress()
	}
}

func (r *ReconcileExperiment) markAssessmentUpdate(context context.Context, instance *iter8v1alpha2.Experiment,
	messageFormat string, messageA ...interface{}) {
	if updated, reason := instance.Status.MarkAssessmentUpdate(messageFormat, messageA...); updated {
		util.Logger(context).Info(reason + ", " + fmt.Sprintf(messageFormat, messageA...))
		r.eventRecorder.Eventf(instance, corev1.EventTypeNormal, reason, messageFormat, messageA...)
		r.notificationCenter.Notify(instance, reason, messageFormat, messageA...)
		r.markStatusUpdate()
	}
}

func (r *ReconcileExperiment) markTrafficUpdate(context context.Context, instance *iter8v1alpha2.Experiment,
	messageFormat string, messageA ...interface{}) {
	if updated, reason := instance.Status.MarkTrafficUpdate(messageFormat, messageA...); updated {
		util.Logger(context).Info(reason + ", " + fmt.Sprintf(messageFormat, messageA...))
		r.eventRecorder.Eventf(instance, corev1.EventTypeNormal, reason, messageFormat, messageA...)
		r.notificationCenter.Notify(instance, reason, messageFormat, messageA...)
		r.markStatusUpdate()
	}
}

func (r *ReconcileExperiment) markExperimentCompleted(context context.Context, instance *iter8v1alpha2.Experiment,
	messageFormat string, messageA ...interface{}) {
	if updated, reason := instance.Status.MarkExperimentCompleted(messageFormat, messageA...); updated {
		util.Logger(context).Info(reason + ", " + fmt.Sprintf(messageFormat, messageA...))
		r.eventRecorder.Eventf(instance, corev1.EventTypeNormal, reason, messageFormat, messageA...)
		r.notificationCenter.Notify(instance, reason, messageFormat, messageA...)
		// Clear analysis state
		instance.Status.AnalysisState.Raw = []byte("{}")
		// Update grafana url
		now := metav1.Now()
		instance.Status.EndTimestamp = &now
		r.grafanaConfig.UpdateGrafanaURL(instance)
		r.markStatusUpdate()
	}
}

func (r *ReconcileExperiment) markSyncMetricsError(context context.Context, instance *iter8v1alpha2.Experiment,
	messageFormat string, messageA ...interface{}) {
	if updated, reason := instance.Status.MarkMetricsSyncedError(messageFormat, messageA...); updated {
		util.Logger(context).Info(reason + ", " + fmt.Sprintf(messageFormat, messageA...))
		r.eventRecorder.Eventf(instance, corev1.EventTypeWarning, reason, messageFormat, messageA...)
		r.notificationCenter.Notify(instance, reason, messageFormat, messageA...)
		r.markStatusUpdate()
	}
}

func (r *ReconcileExperiment) markSyncMetrics(context context.Context, instance *iter8v1alpha2.Experiment,
	messageFormat string, messageA ...interface{}) {
	if updated, reason := instance.Status.MarkMetricsSynced(messageFormat, messageA...); updated {
		util.Logger(context).Info(reason)
		r.eventRecorder.Eventf(instance, corev1.EventTypeNormal, reason, "")
		r.notificationCenter.Notify(instance, reason, "")
		r.markStatusUpdate()
	}
}

func (r *ReconcileExperiment) markRoutingRulesError(context context.Context, instance *iter8v1alpha2.Experiment,
	messageFormat string, messageA ...interface{}) {
	if updated, reason := instance.Status.MarkRoutingRulesError(messageFormat, messageA...); updated {
		util.Logger(context).Info(reason + ", " + fmt.Sprintf(messageFormat, messageA...))
		r.eventRecorder.Eventf(instance, corev1.EventTypeWarning, reason, messageFormat, messageA...)
		r.notificationCenter.Notify(instance, reason, messageFormat, messageA...)
		r.markStatusUpdate()
	}
}

func (r *ReconcileExperiment) markRoutingRulesReady(context context.Context, instance *iter8v1alpha2.Experiment,
	messageFormat string, messageA ...interface{}) {
	if updated, reason := instance.Status.MarkRoutingRulesReady(messageFormat, messageA...); updated {
		util.Logger(context).Info(reason + ", " + fmt.Sprintf(messageFormat, messageA...))
		r.eventRecorder.Eventf(instance, corev1.EventTypeNormal, reason, messageFormat, messageA...)
		r.notificationCenter.Notify(instance, reason, messageFormat, messageA...)
		r.markStatusUpdate()
	}
}

func (r *ReconcileExperiment) markActionPause(context context.Context, instance *iter8v1alpha2.Experiment,
	messageFormat string, messageA ...interface{}) {
	if updated, reason := instance.Status.MarkExperimentPause(messageFormat, messageA...); updated {
		util.Logger(context).Info(reason + ", " + fmt.Sprintf(messageFormat, messageA...))
		r.eventRecorder.Eventf(instance, corev1.EventTypeNormal, reason, "")
		r.notificationCenter.Notify(instance, reason, "")
		r.markStatusUpdate()
	}
}

func (r *ReconcileExperiment) markActionResume(context context.Context, instance *iter8v1alpha2.Experiment,
	messageFormat string, messageA ...interface{}) {
	if updated, reason := instance.Status.MarkExperimentResume(messageFormat, messageA...); updated {
		util.Logger(context).Info(reason + ", " + fmt.Sprintf(messageFormat, messageA...))
		r.eventRecorder.Eventf(instance, corev1.EventTypeNormal, reason, messageFormat, messageA...)
		r.notificationCenter.Notify(instance, reason, messageFormat, messageA...)
		r.markStatusUpdate()
		// need to refresh the whole flow
		r.markRefresh()
	}
}
