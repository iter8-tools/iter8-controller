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

	"github.com/iter8-tools/iter8-istio/pkg/analytics"
	analyticsv1alpha2 "github.com/iter8-tools/iter8-istio/pkg/analytics/api/v1alpha2"
	iter8v1alpha2 "github.com/iter8-tools/iter8-istio/pkg/apis/iter8/v1alpha2"
)

// AnalyticsService with mock response
type AnalyticsService struct {
	// The underlying server
	server *httptest.Server

	// Mock maps request to response. The key maps to request.name
	Mock map[string]analyticsv1alpha2.Response
}

// StartAnalytics starts fake analytics service
func StartAnalytics() *AnalyticsService {
	service := &AnalyticsService{
		Mock: make(map[string]analyticsv1alpha2.Response),
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

	var request analyticsv1alpha2.Request
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

// AddMock adds response for testid
func (s *AnalyticsService) AddMock(name string, response analyticsv1alpha2.Response) {
	s.Mock[name] = response
}

func GetRollToWinnerMockResponse(instance *iter8v1alpha2.Experiment, winIdx int) analyticsv1alpha2.Response {
	candidates := instance.Spec.Candidates
	cas := make([]analyticsv1alpha2.CandidateAssessment, len(candidates))

	for i := range cas {
		ca := analyticsv1alpha2.CandidateAssessment{
			VersionAssessment: analyticsv1alpha2.VersionAssessment{
				ID:             analytics.GetCandidateID(i),
				WinProbability: 0,
				RequestCount:   10,
			},
			Rollback: false,
		}
		if i == winIdx {
			ca.VersionAssessment.WinProbability = 100
		}
		cas[i] = ca
	}

	strategy := instance.Spec.GetStrategy()
	tsr := map[string]map[string]int32{
		strategy: map[string]int32{
			analytics.GetBaselineID(): 20,
		},
	}

	for i := range candidates {
		if i == winIdx {
			tsr[strategy][analytics.GetCandidateID(i)] = 80
		} else {
			tsr[strategy][analytics.GetCandidateID(i)] = 0
		}
	}

	res := analyticsv1alpha2.Response{
		BaselineAssessment: analyticsv1alpha2.VersionAssessment{
			ID:             analytics.GetBaselineID(),
			WinProbability: 0,
			RequestCount:   10,
		},
		CandidateAssessments: cas,
		WinnerAssessment: analyticsv1alpha2.WinnerAssessment{
			WinnerFound: true,
			Winner:      analytics.GetCandidateID(winIdx),
		},
		TrafficSplitRecommendation: tsr,
	}
	return res
}

func GetAbortExperimentResponse(instance *iter8v1alpha2.Experiment) analyticsv1alpha2.Response {
	candidates := instance.Spec.Candidates
	cas := make([]analyticsv1alpha2.CandidateAssessment, len(candidates))

	for i := range cas {
		ca := analyticsv1alpha2.CandidateAssessment{
			VersionAssessment: analyticsv1alpha2.VersionAssessment{
				ID:             analytics.GetCandidateID(i),
				WinProbability: 0,
				RequestCount:   10,
			},
			Rollback: true,
		}
		cas[i] = ca
	}

	strategy := instance.Spec.GetStrategy()
	tsr := map[string]map[string]int32{
		strategy: map[string]int32{
			analytics.GetBaselineID(): 100,
		},
	}

	for _, name := range candidates {
		tsr[strategy][name] = 0
	}

	res := analyticsv1alpha2.Response{
		BaselineAssessment: analyticsv1alpha2.VersionAssessment{
			ID:             analytics.GetBaselineID(),
			WinProbability: 100,
			RequestCount:   10,
		},
		CandidateAssessments:       cas,
		TrafficSplitRecommendation: tsr,
	}
	return res
}

func GetRollbackMockResponse(instance *iter8v1alpha2.Experiment) analyticsv1alpha2.Response {
	candidates := instance.Spec.Candidates
	cas := make([]analyticsv1alpha2.CandidateAssessment, len(candidates))

	for i := range cas {
		ca := analyticsv1alpha2.CandidateAssessment{
			VersionAssessment: analyticsv1alpha2.VersionAssessment{
				ID:             analytics.GetCandidateID(i),
				WinProbability: 0,
				RequestCount:   10,
			},
			Rollback: false,
		}
		cas[i] = ca
	}

	strategy := instance.Spec.GetStrategy()
	tsr := map[string]map[string]int32{
		strategy: map[string]int32{
			analytics.GetBaselineID(): 100,
		},
	}

	for i := range candidates {
		tsr[strategy][analytics.GetCandidateID(i)] = 0
	}

	res := analyticsv1alpha2.Response{
		BaselineAssessment: analyticsv1alpha2.VersionAssessment{
			ID:             analytics.GetBaselineID(),
			WinProbability: 100,
			RequestCount:   10,
		},
		CandidateAssessments:       cas,
		TrafficSplitRecommendation: tsr,
	}
	return res
}
