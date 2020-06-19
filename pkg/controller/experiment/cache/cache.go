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

package cache

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"

	iter8v1alpha1 "github.com/iter8-tools/iter8-controller/pkg/apis/iter8/v1alpha1"
	"github.com/iter8-tools/iter8-controller/pkg/controller/experiment/cache/abstract"
)

// Interface defines the interface for iter8cache
type Interface interface {
	// Given name and namespace of the target deployment, return the experiment key
	DeploymentToExperiment(name, namespace string) (experiment, experimentNamespace string, exist bool)
	// Given name and namespace of the target service, return the experiment key
	ServiceToExperiment(name, namespace string) (experiment, experimentNamespace string, exist bool)
	RegisterExperiment(context context.Context, instance *iter8v1alpha1.Experiment) (context.Context, error)
	RemoveExperiment(instance *iter8v1alpha1.Experiment)

	MarkTargetDeploymentFound(name, namespace string) bool
	MarkTargetServiceFound(name, namespace string) bool

	MarkTargetDeploymentMissing(name, namespace string) bool
	MarkTargetServiceMissing(name, namespace string) bool
}

var _ Interface = &Impl{}

// Impl is the implementation of Iter8Cache
type Impl struct {
	logger logr.Logger

	// the mutext to protect the maps
	m sync.RWMutex
	// an ExperimentAbstract store with experimentName.experimentNamespace as key for access
	experimentAbstractStore map[string]*abstract.Experiment

	// a lookup map from target to experiment
	deployment2Experiment map[string]string

	// a lookup map from target service to experiment
	service2Experiment map[string]string
}

// New returns a new iter8cache implementation
func New(logger logr.Logger) Interface {
	return &Impl{
		experimentAbstractStore: make(map[string]*abstract.Experiment),
		deployment2Experiment:   make(map[string]string),
		service2Experiment:      make(map[string]string),
		logger:                  logger,
	}
}

func (c *Impl) checkAndGetServices(instance *iter8v1alpha1.Experiment) ([]string, error) {
	out := []string{}
	ts := instance.Spec.TargetService
	ns := instance.ServiceNamespace()

	if ts.Name != "" {
		key := targetKey(ts.Name, ns)
		if _, ok := c.service2Experiment[key]; ok {
			return nil, fmt.Errorf("Service %s is being involved in other experiment", key)
		}
		out = append(out, key)
	}

	if ts.Kind == "Service" {
		baselineKey := targetKey(ts.Baseline, ns)
		if _, ok := c.service2Experiment[baselineKey]; ok {
			return nil, fmt.Errorf("Baseline %s is being involved in other experiment", baselineKey)
		}
		out = append(out, baselineKey)

		candidateKey := targetKey(ts.Candidate, ns)
		if _, ok := c.service2Experiment[candidateKey]; ok {
			return nil, fmt.Errorf("Candidate %s is being involved in other experiment", baselineKey)
		}
		out = append(out, candidateKey)
	}

	return out, nil
}

func (c *Impl) checkAndGetDeployments(instance *iter8v1alpha1.Experiment) ([]string, error) {
	ts := instance.Spec.TargetService
	if ts.Kind == "Deployment" {
		out := []string{}
		ts := instance.Spec.TargetService
		ns := instance.ServiceNamespace()
		baselineKey := targetKey(ts.Baseline, ns)
		if _, ok := c.deployment2Experiment[baselineKey]; ok {
			return nil, fmt.Errorf("Baseline %s is being involved in other experiment", baselineKey)
		}
		out = append(out, baselineKey)

		candidateKey := targetKey(ts.Candidate, ns)
		if _, ok := c.deployment2Experiment[candidateKey]; ok {
			return nil, fmt.Errorf("Candidate %s is being involved in other experiment", baselineKey)
		}
		out = append(out, candidateKey)
		return out, nil
	}
	return nil, nil
}

// RegisterExperiment creates new abstracts into the cache and snapshot the abstract into context
func (c *Impl) RegisterExperiment(ctx context.Context, instance *iter8v1alpha1.Experiment) (context.Context, error) {
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

		c.experimentAbstractStore[eakey] = abstract.NewExperiment(serviceKeys, deploymentKeys)
		for _, svc := range serviceKeys {
			c.service2Experiment[svc] = eakey
		}

		for _, dep := range deploymentKeys {
			c.deployment2Experiment[dep] = eakey
		}

		c.experimentAbstractStore[eakey] = abstract.NewExperiment(serviceKeys, deploymentKeys)
	}

	ea := c.experimentAbstractStore[eakey]
	eas := ea.GetSnapshot()
	ctx = context.WithValue(ctx, abstract.SnapshotKey, eas)
	c.logger.Info("ExperimentAbstract", eakey, eas)
	return ctx, nil
}

// DeploymentToExperiment returns the experiment key given name and namespace of target deployment
func (c *Impl) DeploymentToExperiment(targetName, targetNamespace string) (string, string, bool) {
	c.m.Lock()
	defer c.m.Unlock()

	tKey := targetKey(targetName, targetNamespace)
	if _, ok := c.deployment2Experiment[tKey]; !ok {
		return "", "", false
	}
	name, namespace := resolveExperimentKey(c.deployment2Experiment[tKey])

	return name, namespace, true
}

func (c *Impl) MarkTargetDeploymentFound(targetName, targetNamespace string) bool {
	c.m.Lock()
	defer c.m.Unlock()

	tKey := targetKey(targetName, targetNamespace)
	eaKey, ok := c.deployment2Experiment[tKey]
	if !ok {
		return false
	}

	c.experimentAbstractStore[eaKey].MarkTargetFound(targetName, true)

	return true
}

func (c *Impl) MarkTargetDeploymentMissing(targetName, targetNamespace string) bool {
	c.m.Lock()
	defer c.m.Unlock()

	tKey := targetKey(targetName, targetNamespace)
	eaKey, ok := c.deployment2Experiment[tKey]
	if !ok {
		return false
	}

	c.experimentAbstractStore[eaKey].MarkTargetFound(targetName, false)

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

	name, namespace := resolveExperimentKey(c.service2Experiment[tKey])

	return name, namespace, true
}

func (c *Impl) MarkTargetServiceFound(targetName, targetNamespace string) bool {
	c.m.Lock()
	defer c.m.Unlock()

	tKey := targetKey(targetName, targetNamespace)
	eaKey, ok := c.service2Experiment[tKey]
	if !ok {
		return false
	}

	c.experimentAbstractStore[eaKey].MarkTargetFound(tKey, true)

	return true
}

func (c *Impl) MarkTargetServiceMissing(targetName, targetNamespace string) bool {
	c.m.Lock()
	defer c.m.Unlock()

	tKey := targetKey(targetName, targetNamespace)
	eaKey, ok := c.service2Experiment[tKey]
	if !ok {
		return false
	}

	c.experimentAbstractStore[eaKey].MarkTargetFound(tKey, false)

	return true
}

// RemoveExperiment removes the experiment abstract from the cache
func (c *Impl) RemoveExperiment(instance *iter8v1alpha1.Experiment) {
	c.m.Lock()
	defer c.m.Unlock()

	eakey := experimentKey(instance)
	ea, ok := c.experimentAbstractStore[eakey]
	if !ok {
		return
	}

	for _, key := range ea.ServiceKeys {
		delete(c.service2Experiment, key)
	}

	for _, key := range ea.DeploymentKeys {
		delete(c.deployment2Experiment, key)
	}
	delete(c.experimentAbstractStore, eakey)
}
