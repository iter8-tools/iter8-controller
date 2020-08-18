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

package grafana

import (
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iter8v1alpha2 "github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
)

const (
	defaultHost     = "localhost"
	defaultPort     = "3000"
	defaultBoardUID = "eXPEaNnZz"
)

// Interface specifies functions to be called
type Interface interface {
	UpdateGrafanaURL(instance *iter8v1alpha2.Experiment)
}

var _ Interface = &ConfigStore{}

// NewConfigStore returns the implentation of Interface
func NewConfigStore(logger logr.Logger, client client.Client) *ConfigStore {
	return &ConfigStore{
		logger: logger,
		client: client,
	}
}

// ConfigStore contains
type ConfigStore struct {
	host *string
	port *string
	uid  *string

	logger logr.Logger
	client client.Client
}

// GetEndpoint returns the endpoint to grafana dashboard
func (c *ConfigStore) getEndpoint() string {
	out := ""
	if c.host != nil {
		out += *c.host
	} else {
		out += defaultHost
	}

	if c.port != nil {
		out += ":" + *c.port
	} else {
		out += ":" + defaultPort
	}

	return out
}

// UpdateGrafanaURL will update grafana url in the status of the experiment instance with latest setting
func (c *ConfigStore) UpdateGrafanaURL(instance *iter8v1alpha2.Experiment) {
	if instance.Status.StartTimestamp == nil {
		// StartTimestamp value is required
		return
	}
	candidates := ""
	for _, candidate := range instance.Spec.Service.Candidates {
		candidates += "," + candidate
	}
	endTsStr := "now"
	if instance.Status.EndTimestamp != nil {
		endTsStr = strconv.FormatInt(instance.Status.EndTimestamp.UTC().UnixNano()/int64(time.Millisecond), 10)
	}
	grafanaEndpoint := c.getEndpoint() +
		"/d/" + defaultBoardUID + "/iter8-application-metrics?" +
		"var-namespace=" + instance.ServiceNamespace() +
		"&var-service=" + instance.Spec.Service.Name +
		"&var-baseline=" + instance.Spec.Service.Baseline +
		"&var-candidate=" + candidates[1:] +
		"&from=" + strconv.FormatInt(instance.Status.StartTimestamp.UTC().UnixNano()/int64(time.Millisecond), 10) +
		"&to=" + endTsStr
	instance.Status.GrafanaURL = &grafanaEndpoint
}
