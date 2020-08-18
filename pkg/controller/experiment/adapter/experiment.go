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

package adapter

type actionKeyType string
type targetAction string

const (
	ActionKey = actionKeyType("experimentAction")

	targetActionDetected = targetAction("detected")
	targetActionDeleted  = targetAction("deleted")
)

// Catcher defines functions can be invoked by Adapter
// It catches creation/deletion of experiment targets
type Catcher interface {
	MarkTargetDetected(name string, kind string)
	MarkTargetDeleted(name string, kind string)
}

// Action specifies desired actions to be performed by controller to the experiment
type Action interface {
	Refresh() bool
	Resume() bool
}

var _ Catcher = &experiment{}
var _ Action = &experiment{}

// Experiment includes abstract info for one Experiment
type experiment struct {
	serviceKeys    []string
	deploymentKeys []string
	targetAction   targetAction
}

// NewExperiment returns an Experiment instance used in controlelr adapter
func newExperiment(services, deployments []string) *experiment {
	return &experiment{
		serviceKeys:    services,
		deploymentKeys: deployments,
	}
}

// Refresh indicates whether the controller should allow refresh workflows on the experiment
func (e *experiment) Refresh() bool {
	return e.targetAction == targetActionDeleted
}

// Resume indicates whether the controller should try to resume the experiment or not
func (e *experiment) Resume() bool {
	return e.targetAction == targetActionDetected
}

func (e *experiment) clearAction() {
	e.targetAction = ""
}

// MarkTargetDetected captures a detection of a target
func (e *experiment) MarkTargetDetected(name string, kind string) {
	e.targetAction = targetActionDetected
}

// MarkTargetDeleted captures a deletion of a target
func (e *experiment) MarkTargetDeleted(name string, kind string) {
	e.targetAction = targetActionDeleted
}

// GetAction returns the action indicator of the experiment
func (e *experiment) GetAction() Action {
	out := &experiment{}
	*out = *e
	e.clearAction()
	return out
}
