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

package abstract

type snapshotKeyType string
type targetAction string

const (
	SnapshotKey = snapshotKeyType("experimentAbstract")

	targetActionDetected = targetAction("detected")
	targetActionDeleted  = targetAction("deleted")
)

type ExperimentInterface interface {
	MarkTargetFound(name string, found bool)
}

type Snapshot interface {
	Refresh() bool
	Resume() bool
}

var _ ExperimentInterface = &Experiment{}
var _ Snapshot = &Experiment{}

// Experiment includes abstract info for one Experiment
type Experiment struct {
	ServiceKeys    []string
	DeploymentKeys []string
	targetAction   targetAction
}

func NewExperiment(services, deployments []string) *Experiment {
	return &Experiment{
		ServiceKeys:    services,
		DeploymentKeys: deployments,
	}
}

func (e *Experiment) Refresh() bool {
	return e.targetAction == targetActionDeleted
}

func (e *Experiment) Resume() bool {
	return e.targetAction == targetActionDetected
}

func (e *Experiment) clear() {
	e.targetAction = ""
}

func (e *Experiment) MarkTargetFound(name string, found bool) {
	if found == false {
		e.targetAction = targetActionDetected
	} else {
		e.targetAction = targetActionDeleted
	}
}

func (e *Experiment) GetSnapshot() Snapshot {
	out := &Experiment{}
	*out = *e
	e.clear()
	return out
}
