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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/go-logr/logr"
	iter8v1alpha2 "github.com/iter8-tools/iter8-istio/pkg/apis/iter8/v1alpha2"
)

const (
	NotifierLevelVerbose = "verbose"
	NotifierLevelNormal  = "normal"
	NotifierLevelWarning = "warning"
	NotifierLevelError   = "error"
)

// NotificationCenter is designed to send notifications to registered notifiers
type NotificationCenter struct {
	m         sync.RWMutex
	logger    logr.Logger
	Notifiers map[string]*ConfiguredNotifier
}

// ConfiguredNotifier is the wrapper of the a notifier implementation and its configuration
type ConfiguredNotifier struct {
	config *Config
	impl   Notifier
}

// Notifier is the interface for notifier implementations
type Notifier interface {
	// MakeRequest returns the platform-specific request instance
	MakeRequest(instance *iter8v1alpha2.Experiment, reason string, messageFormat string, messageA ...interface{}) interface{}
}

// NewNotificationCenter returns a new NotificationCenter
func NewNotificationCenter(logger logr.Logger) *NotificationCenter {
	return &NotificationCenter{
		logger:    logger,
		Notifiers: make(map[string]*ConfiguredNotifier),
	}
}

// UpdateNotifier will update the notifier stored inside the center
func (nc *NotificationCenter) updateNotifier(name string, cfg *Config) {
	var impl Notifier
	switch cfg.Notifier {
	case NotifierNameSlack:
		impl = NewSlackWebhook()
	}

	nc.Notifiers[name] = &ConfiguredNotifier{
		config: cfg,
		impl:   impl,
	}
	nc.logger.Info("notifier channel updated", "name", name, "level", cfg.Level)
}

// RemoveNotifier will remove the notifier stored inside the center
func (nc *NotificationCenter) removeNotifier(name string) {
	delete(nc.Notifiers, name)
	nc.logger.Info("notifier channel removed", "name", name)
}

// Config defines the configuration used for a notifier channel
type Config struct {
	// Namespace speficies the namespace where experiments should be monitored
	Namespace string `yaml:"namespace,omitempty"`

	// URL specifies the endpoint to send notification
	URL string `yaml:"url"`

	// NotifierName is the name of notification receiver
	Notifier string `yaml:"notifier"`

	// Level specifies the informative level
	Level string `yaml:"level"`

	// Actions lists the actions that user may want to take during the experiment
	Actions []string `yaml:"actions,omitempty"`

	// Labels are used to filter out the experiments for report
	Labels map[string]string `yaml:"labels,omitempty"`
}

func (c *Config) validateAndSetDefault() error {
	switch c.Notifier {
	case NotifierNameSlack:
		//valid
	default:
		return fmt.Errorf("Unsupported notifier: %s", c.Notifier)
	}

	switch c.Level {
	case NotifierLevelError, NotifierLevelWarning, NotifierLevelNormal, NotifierLevelVerbose:
		//valid
	case "":
		// default option
		c.Level = NotifierLevelNormal
	default:
		return fmt.Errorf("Fail to recognize level: %s", c.Level)
	}

	return nil
}

func matchLabels(nLabels, eLabels map[string]string) bool {
	if len(nLabels) > 0 {
		if len(eLabels) == 0 {
			return false
		}

		for key, val := range nLabels {
			if eVal, ok := eLabels[key]; !ok || eVal != val {
				return false
			}
		}
	}
	return true
}

// Notify will generate notifications to all the matched notifier specified in the configs
// Errors occured will only be logged
func (nc *NotificationCenter) Notify(instance *iter8v1alpha2.Experiment, reason string, messageFormat string, messageA ...interface{}) {
	if len(nc.Notifiers) == 0 {
		return
	}

	for name, ntf := range nc.Notifiers {
		// Only send out notification if reason severity is not less than notifier level
		if reasonSeverity(reason) < level2Int(ntf.config.Level) {
			continue
		}
		// match namespace
		if len(ntf.config.Namespace) > 0 && ntf.config.Namespace != instance.GetNamespace() {
			continue
		}

		// match labels
		if !matchLabels(ntf.config.Labels, instance.GetLabels()) {
			continue
		}

		payload := ntf.impl.MakeRequest(instance, reason, messageFormat, messageA...)
		if err := post(ntf.config.URL, payload); err != nil {
			nc.logger.Error(err, "Fail to post notification", "channel", name)
		}
	}
}

// post sends notification to destination
// Only reads response status code for now
func post(url string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	raw, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	defer raw.Body.Close()
	body, err := ioutil.ReadAll(raw.Body)

	if raw.StatusCode >= 400 {
		return fmt.Errorf("%v, request: %v", string(body), string(data))
	}

	return nil
}

// returns hardcoded notifier level value
func level2Int(level string) int {
	switch level {
	case NotifierLevelError:
		return 4
	case NotifierLevelWarning:
		return 3
	case NotifierLevelNormal:
		return 2
	case NotifierLevelVerbose:
		return 1
	}
	return -1
}

// returns hardcoded severity value
func reasonSeverity(r string) int {
	switch r {
	case iter8v1alpha2.ReasonExperimentCompleted:
		return 5
	case iter8v1alpha2.ReasonTargetsError,
		iter8v1alpha2.ReasonSyncMetricsError,
		iter8v1alpha2.ReasonRoutingRulesError,
		iter8v1alpha2.ReasonAnalyticsServiceError,
		iter8v1alpha2.ReasonActionPause:
		return 4

	case iter8v1alpha2.ReasonTargetsFound,
		iter8v1alpha2.ReasonAnalyticsServiceRunning,
		iter8v1alpha2.ReasonIterationUpdate,
		iter8v1alpha2.ReasonSyncMetricsSucceeded,
		iter8v1alpha2.ReasonRoutingRulesReady:
		return 1
	}

	return 0
}
