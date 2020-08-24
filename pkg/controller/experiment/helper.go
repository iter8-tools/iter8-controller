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
	"strings"

	iter8v1alpha2 "github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
	"github.com/iter8-tools/iter8/pkg/controller/experiment/adapter"
	"github.com/iter8-tools/iter8/pkg/controller/experiment/util"
)

// interState controlls the execution of inter functions in the reconcile logic
type interState struct {
	statusUpdate bool
	refresh      bool
	progress     bool
}

func (r *ReconcileExperiment) initState() {
	r.interState = interState{}
}

func (r *ReconcileExperiment) markStatusUpdate() {
	r.interState.statusUpdate = true
}

func (r *ReconcileExperiment) needStatusUpdate() bool {
	return r.interState.statusUpdate
}

func (r *ReconcileExperiment) markRefresh() {
	r.interState.refresh = true
}

func (r *ReconcileExperiment) needRefresh() bool {
	return r.interState.refresh
}

func (r *ReconcileExperiment) markProgress() {
	r.interState.progress = true
}

func (r *ReconcileExperiment) hasProgress() bool {
	return r.interState.progress
}

func (r *ReconcileExperiment) injectClients(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, util.IstioClientKey, r.istioClient)
	return ctx
}

func experimentAction(ctx context.Context) adapter.Action {
	if ctx.Value(adapter.ActionKey) == nil {
		return nil
	}
	return ctx.Value(adapter.ActionKey).(adapter.Action)
}

func validUpdateErr(err error) bool {
	if err == nil {
		return true
	}
	benignMsg := "the object has been modified"
	return strings.Contains(err.Error(), benignMsg)
}

// overrideAssessment sets the assessment when experiment is being terminated
func overrideAssessment(instance *iter8v1alpha2.Experiment) {
	// set onTermination strategy from manualOverrides if configured
	if instance.Spec.Terminate() && instance.Spec.ManualOverride != nil {
		onTermination := iter8v1alpha2.OnTerminationToBaseline
		if len(instance.Spec.ManualOverride.TrafficSplit) > 0 {
			trafficSplit := instance.Spec.ManualOverride.TrafficSplit
			if ts, ok := trafficSplit[instance.Status.Assessment.Baseline.Name]; ok {
				instance.Status.Assessment.Baseline.Weight = ts
			} else {
				instance.Status.Assessment.Baseline.Weight = 0
			}

			for i := range instance.Status.Assessment.Candidates {
				if ts, ok := trafficSplit[instance.Status.Assessment.Candidates[i].Name]; ok {
					instance.Status.Assessment.Candidates[i].Weight = ts
				} else {
					instance.Status.Assessment.Candidates[i].Weight = 0
				}
			}

			onTermination = iter8v1alpha2.OnTerminationKeepLast
		}

		instance.Spec.TrafficControl = &iter8v1alpha2.TrafficControl{
			OnTermination: &onTermination,
		}
	}

	// set final traffic status in assessment
	assessment := instance.Status.Assessment
	switch instance.Spec.GetOnTermination() {
	case iter8v1alpha2.OnTerminationToWinner:
		if instance.Status.IsWinnerFound() {
			// all traffic to winner
			if assessment.Winner.Winner == assessment.Baseline.ID {
				assessment.Baseline.Weight = 100
				for i := range assessment.Candidates {
					assessment.Candidates[i].Weight = 0
				}
			} else {
				assessment.Baseline.Weight = 0
				matchfound := false
				for i := range assessment.Candidates {
					if assessment.Candidates[i].ID == assessment.Winner.Winner {
						matchfound = true
						assessment.Candidates[i].Weight = 100
					} else {
						assessment.Candidates[i].Weight = 0
					}
				}
				// safe guard to make sure final traffic should be at leaset 100 percent sent to baseline
				if !matchfound {
					assessment.Baseline.Weight = 100
				}
			}
			break
		}
		fallthrough
	case iter8v1alpha2.OnTerminationToBaseline:
		// all traffic to baseline
		assessment.Baseline.Weight = 100
		for i := range assessment.Candidates {
			assessment.Candidates[i].Weight = 0
		}
	case iter8v1alpha2.OnTerminationKeepLast:
		// do nothing
	}

	instance.Status.Assessment = assessment
}
