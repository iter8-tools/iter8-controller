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

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"

	iter8v1alpha2 "github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
)

// Interface defines the interface for iter8cache
type Interface interface {
	// Given name and namespace of the target deployment, return the experiment key
	DeploymentToExperiment(name, namespace string) (experiment, experimentNamespace string, exist bool)
	// Given name and namespace of the target service, return the experiment key
	ServiceToExperiment(name, namespace string) (experiment, experimentNamespace string, exist bool)
	RegisterExperiment(context context.Context, instance *iter8v1alpha2.Experiment) (context.Context, error)
	RemoveExperiment(instance *iter8v1alpha2.Experiment)

	MarkDeploymentDetected(name, namespace string) bool
	MarkServiceDetected(name, namespace string) bool

	MarkDeploymentDeleted(name, namespace string) bool
	MarkServiceDeleted(name, namespace string) bool

	Inspect()
}

var _ Interface = &Impl{}

// Impl is the implementation of Iter8Cache
type Impl struct {
	logger logr.Logger

	// the mutext to protect the maps
	m sync.RWMutex
	// an ExperimentAbstract store with experimentName.experimentNamespace as key for access
	experimentAbstractStore map[string]*experiment

	// a lookup map from target to experiment
	// targetName.targetNamespace -> experimentName.experimentNamespace
	deployment2Experiment map[string]string

	// a lookup map from target service to experiment
	service2Experiment map[string]string
}

// New returns a new iter8cache implementation
func New(logger logr.Logger) Interface {
	return &Impl{
		experimentAbstractStore: make(map[string]*experiment),
		deployment2Experiment:   make(map[string]string),
		service2Experiment:      make(map[string]string),
		logger:                  logger,
	}
}

// RegisterExperiment creates new abstracts into the cache and snapshot the abstract into context
func (c *Impl) RegisterExperiment(ctx context.Context, instance *iter8v1alpha2.Experiment) (context.Context, error) {
	c.m.Lock()
	defer c.m.Unlock()

	eakey := experimentKey(instance)
	if _, ok := c.experimentAbstractStore[eakey]; !ok {
		serviceKeys, err := c.checkAndGetServices(instance)
		if err != nil {
			return ctx, err
		}

		deploymentKeys, err := c.checkAndGetDeployments(instance)
		if err != nil {
			return ctx, err
		}

		c.experimentAbstractStore[eakey] = newExperiment(serviceKeys, deploymentKeys)

		for _, svc := range serviceKeys {
			c.service2Experiment[svc] = eakey
		}

		for _, dep := range deploymentKeys {
			c.deployment2Experiment[dep] = eakey
		}
	}

	ea := c.experimentAbstractStore[eakey]
	ctx = context.WithValue(ctx, ActionKey, ea.GetAction())

	c.logger.Info("ExperimentRegistered", "experiemnt", ea)
	c.Inspect()
	return ctx, nil
}

// Inspect prints details of adapter into log
func (c *Impl) Inspect() {
	c.logger.Info("iter8Adapter", "deployment2Experiment", c.deployment2Experiment)
	c.logger.Info("iter8Adapter", "service2Experiment", c.service2Experiment)
}

// DeploymentToExperiment returns the experiment key given name and namespace of target deployment
func (c *Impl) DeploymentToExperiment(targetName, targetNamespace string) (string, string, bool) {
	c.m.Lock()
	defer c.m.Unlock()

	tKey := targetKey(targetName, targetNamespace)
	if _, ok := c.deployment2Experiment[tKey]; !ok {
		return "", "", false
	}
	namespace, name := resolveExperimentKey(c.deployment2Experiment[tKey])

	return name, namespace, true
}

// MarkDeploymentDetected marks the event that a target deployment is detected
func (c *Impl) MarkDeploymentDetected(targetName, targetNamespace string) bool {
	c.m.Lock()
	defer c.m.Unlock()

	tKey := targetKey(targetName, targetNamespace)
	eaKey, ok := c.deployment2Experiment[tKey]
	if !ok {
		return false
	}

	c.experimentAbstractStore[eaKey].MarkTargetDetected(targetName, "Deployment")

	return true
}

// MarkDeploymentDeleted marks the event that a target deployment is deleted
func (c *Impl) MarkDeploymentDeleted(targetName, targetNamespace string) bool {
	c.m.Lock()
	defer c.m.Unlock()

	tKey := targetKey(targetName, targetNamespace)
	eaKey, ok := c.deployment2Experiment[tKey]
	if !ok {
		return false
	}

	c.experimentAbstractStore[eaKey].MarkTargetDeleted(targetName, "Deployment")

	return true
}

// ServiceToExperiment returns the experiment key given name and namespace of target service
func (c *Impl) ServiceToExperiment(targetName, targetNamespace string) (string, string, bool) {
	c.m.Lock()
	defer c.m.Unlock()

	tKey := targetKey(targetName, targetNamespace)
	if _, ok := c.service2Experiment[tKey]; !ok {
		return "", "", false
	}

	namespace, name := resolveExperimentKey(c.service2Experiment[tKey])

	return name, namespace, true
}

// MarkServiceDetected marks the event that a target service is detected
func (c *Impl) MarkServiceDetected(targetName, targetNamespace string) bool {
	c.m.Lock()
	defer c.m.Unlock()

	tKey := targetKey(targetName, targetNamespace)
	eaKey, ok := c.service2Experiment[tKey]
	if !ok {
		return false
	}

	c.experimentAbstractStore[eaKey].MarkTargetDetected(targetName, "Service")

	return true
}

// MarkServiceDeleted marks the event that a target service is deleted
func (c *Impl) MarkServiceDeleted(targetName, targetNamespace string) bool {
	c.m.Lock()
	defer c.m.Unlock()

	tKey := targetKey(targetName, targetNamespace)
	eaKey, ok := c.service2Experiment[tKey]
	if !ok {
		return false
	}

	c.experimentAbstractStore[eaKey].MarkTargetDeleted(targetName, "Service")

	return true
}

// RemoveExperiment removes the experiment abstract from the cache
func (c *Impl) RemoveExperiment(instance *iter8v1alpha2.Experiment) {
	c.m.Lock()
	defer c.m.Unlock()

	eakey := experimentKey(instance)
	ea, ok := c.experimentAbstractStore[eakey]
	if !ok {
		return
	}

	for _, key := range ea.serviceKeys {
		delete(c.service2Experiment, key)
	}

	for _, key := range ea.deploymentKeys {
		delete(c.deployment2Experiment, key)
	}
	delete(c.experimentAbstractStore, eakey)
}

func (c *Impl) checkAndGetServices(instance *iter8v1alpha2.Experiment) ([]string, error) {
	out := []string{}
	service := instance.Spec.Service
	ns := instance.ServiceNamespace()

	if service.Name != "" {
		key := targetKey(service.Name, ns)
		if _, ok := c.service2Experiment[key]; ok {
			return nil, fmt.Errorf("Service %s is being involved in other experiment", key)
		}
		out = append(out, key)
	}

	if service.Kind == "Service" {
		baselineKey := targetKey(service.Baseline, ns)
		if _, ok := c.service2Experiment[baselineKey]; ok {
			return nil, fmt.Errorf("Baseline %s is being involved in other experiment", baselineKey)
		}
		out = append(out, baselineKey)

		for _, candidate := range service.Candidates {
			candidateKey := targetKey(candidate, ns)
			if _, ok := c.service2Experiment[candidateKey]; ok {
				return nil, fmt.Errorf("Candidate %s is being involved in other experiment", candidateKey)
			}
			out = append(out, candidateKey)
		}
	}

	return out, nil
}

func (c *Impl) checkAndGetDeployments(instance *iter8v1alpha2.Experiment) ([]string, error) {
	service := instance.Spec.Service
	if service.Kind == "Deployment" || service.Kind == "" {
		out := []string{}
		ns := instance.ServiceNamespace()

		baselineKey := targetKey(service.Baseline, ns)
		if _, ok := c.deployment2Experiment[baselineKey]; ok {
			return nil, fmt.Errorf("Baseline %s is being involved in other experiment", baselineKey)
		}
		out = append(out, baselineKey)

		for _, candidate := range service.Candidates {
			candidateKey := targetKey(candidate, ns)
			if _, ok := c.service2Experiment[candidateKey]; ok {
				return nil, fmt.Errorf("Candidate %s is being involved in other experiment", candidateKey)
			}
			out = append(out, candidateKey)
		}
		return out, nil
	}
	return nil, nil
}
