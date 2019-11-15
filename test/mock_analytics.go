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

package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/iter8-tools/iter8-controller/pkg/analytics"
	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
)

const (
	// TestTag identifies the test id
	TestTag = "testid"
)

// AnalyticsService with mock response
type AnalyticsService struct {
	// The underlying server
	server *httptest.Server

	// Mock maps request to response. The key maps to request.name
	Mock map[string]analytics.Response
}

// StartAnalytics starts fake analytics service
func StartAnalytics() *AnalyticsService {
	service := &AnalyticsService{
		Mock: make(map[string]analytics.Response),
	}
	service.server = httptest.NewServer(service)
	return service
}

// GetURL returns the service URL
func (s *AnalyticsService) GetURL() string {
	return s.server.URL
}

// Close the service
func (s *AnalyticsService) Close() {
	s.server.Close()
}

func (s *AnalyticsService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(400)
		return
	}

	var request analytics.Request
	err = json.Unmarshal(b, &request)
	if err != nil {
		w.Write([]byte(err.Error()))
		w.WriteHeader(400)
		return
	}

	name := request.Name
	if name == "" {
		w.Write([]byte("missing request name"))
		w.WriteHeader(400)
		return
	}

	response, ok := s.Mock[name]
	if !ok {
		w.Write([]byte("missing response for test " + name))
		w.WriteHeader(400)
		return
	}

	raw, err := json.Marshal(response)
	if err != nil {
		w.Write([]byte(fmt.Sprintf("invalid response format %v", err)))
		w.WriteHeader(400)
		return
	}
	w.WriteHeader(200)
	w.Write(raw)
}

// Mock adds response for testid
func (s *AnalyticsService) AddMock(name string, response analytics.Response) {
	s.Mock[name] = response
}

func GetDefaultMockResponse() analytics.Response {
	return analytics.Response{
		Baseline: analytics.MetricsTraffic{
			TrafficPercentage: 50,
		},
		Candidate: analytics.MetricsTraffic{
			TrafficPercentage: 50,
		},
		Assessment: analytics.Assessment{
			Summary: iter8v1alpha1.Summary{
				AbortExperiment: false,
			},
		},
	}
}

func GetSuccessMockResponse() analytics.Response {
	return analytics.Response{
		Baseline: analytics.MetricsTraffic{
			TrafficPercentage: 50,
		},
		Candidate: analytics.MetricsTraffic{
			TrafficPercentage: 50,
		},
		Assessment: analytics.Assessment{
			Summary: iter8v1alpha1.Summary{
				AllSuccessCriteriaMet: true,
				AbortExperiment:       false,
			},
		},
	}
}

func GetAbortExperimentResponse() analytics.Response {
	return analytics.Response{
		Assessment: analytics.Assessment{
			Summary: iter8v1alpha1.Summary{
				AbortExperiment: true,
			},
		},
	}
}

func GetFailureMockResponse() analytics.Response {
	return analytics.Response{
		Baseline: analytics.MetricsTraffic{
			TrafficPercentage: 50,
		},
		Candidate: analytics.MetricsTraffic{
			TrafficPercentage: 50,
		},
		Assessment: analytics.Assessment{
			Summary: iter8v1alpha1.Summary{
				AllSuccessCriteriaMet: false,
			},
		},
	}
}
