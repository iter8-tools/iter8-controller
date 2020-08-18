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

package notifier

import (
	"fmt"
	"strings"

	"github.com/fatih/camelcase"
	iter8v1alpha2 "github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
)

const (
	NotifierNameSlack = "slack"

	SectionBlockType = "section"

	MarkdownTextType = "mrkdwn"
)

var _ Notifier = (*SlackWebhook)(nil)

type SlackWebhook struct{}

func NewSlackWebhook() *SlackWebhook {
	return &SlackWebhook{}
}

type MarkdownText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type SectionBlock struct {
	Type string       `json:"type"`
	Text MarkdownText `json:"text"`
}

type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

type Attachment struct {
	Color  string  `json:"color"`
	Author string  `json:"author_name"`
	Fields []Field `json:"fields"`
	Text   string  `json:"text"`
	Ts     int64   `json:"ts"`
}

type SlackRequest struct {
	Text        string         `json:"text"`
	Blocks      []SectionBlock `json:"blocks"`
	Attachments []Attachment   `json:"attachments"`
}

// MakeRequest implements Notifier MakeRequest function
func (s *SlackWebhook) MakeRequest(instance *iter8v1alpha2.Experiment, reason string, messageFormat string, messageA ...interface{}) interface{} {
	splittedReason := splitString(reason)
	expName := instance.GetName() + "." + instance.GetNamespace()

	// set notification abstract
	sr := &SlackRequest{
		Text: expName + ": " + splittedReason,
	}

	sr.Blocks = append(sr.Blocks, SectionBlock{
		Type: SectionBlockType,
		Text: MarkdownText{
			Type: MarkdownTextType,
			Text: "*" + splittedReason + "*\n" + expName,
		},
	})

	details := fmt.Sprintf(messageFormat, messageA...)
	if len(details) > 0 {
		sr.Blocks = append(sr.Blocks, SectionBlock{
			Type: SectionBlockType,
			Text: MarkdownText{
				Type: MarkdownTextType,
				Text: "_" + details + "_",
			},
		})
	}

	// color := "good"

	// if reasonSeverity(reason) >= 3 {
	// 	switch reason {
	// 	case iter8v1alpha2.ReasonExperimentCompleted:
	// 		//do nothing
	// 	default:
	// 		color = "danger"
	// 	}
	// }

	// progress := ""
	// if *instance.Status.CurrentIteration > instance.Spec.GetMaxIterations() ||
	// 	reason == iter8v1alpha2.ReasonExperimentCompleted {
	// 	progress = "Experiment Completed"
	// } else {
	// 	progress = "Current Iteration: " + strconv.Itoa(int(*instance.Status.CurrentIteration)) + "/" + strconv.Itoa(int(instance.Spec.GetMaxIterations()))
	// }

	// sr.Attachments = []Attachment{{
	// 	Fields: []Field{
	// 		{
	// 			Title: "Traffic",
	// 			Value: "Baseline: " + strconv.Itoa(instance.Status.TrafficSplit.Baseline) +
	// 				"  Candidate: " + strconv.Itoa(instance.Status.TrafficSplit.Candidate),
	// 			Short: false,
	// 		},
	// 		{
	// 			Title: "Progress",
	// 			Value: progress,
	// 			Short: true,
	// 		},
	// 		{
	// 			Title: "Analytics Assessment Summary",
	// 			Value: instance.Status.AssessmentSummary.Assessment2String(),
	// 			Short: false,
	// 		},
	// 	},
	// 	Color: color,
	// 	Ts:    time.Now().Unix(),
	// }}

	return sr
}

func splitString(s string) string {
	out := ""
	for _, w := range camelcase.Split(s) {
		out += w + " "
	}

	return strings.TrimSpace(out)
}
