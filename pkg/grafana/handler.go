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
	"context"
	"os"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	iter8v1alpha2 "github.com/iter8-tools/iter8/pkg/apis/iter8/v1alpha2"
)

const (
	configMapName    = "iter8config"
	defaultNamespace = "iter8"
)

// RegisterHandler adds event handlers to k8s cache
func (c *ConfigStore) RegisterHandler(cache cache.Cache) error {
	typeObj, handler := getTypedHandler(c)

	informer, err := cache.GetInformer(typeObj)
	if err != nil {
		return err
	}
	informer.AddEventHandler(handler)
	return nil
}

func getTypedHandler(c *ConfigStore) (runtime.Object, toolscache.FilteringResourceEventHandler) {
	handler := toolscache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			cm := obj.(*corev1.ConfigMap)
			if cm.GetNamespace() == defaultNamespace && cm.GetName() == configMapName {
				c.logger.Info("grafana configmap detected", "name", cm.GetName())
				return true
			}

			return false
		},
		Handler: toolscache.ResourceEventHandlerFuncs{
			AddFunc: updateConfigFromConfigmap(c),
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldCm, newCm := oldObj.(*corev1.ConfigMap), newObj.(*corev1.ConfigMap)
				if !reflect.DeepEqual(oldCm.Data, newCm.Data) {
					updateConfigFromConfigmap(c)(newObj)
				}
			},
			DeleteFunc: updateConfigFromConfigmap(c),
		},
	}

	return &corev1.ConfigMap{}, handler
}

func updateConfigFromConfigmap(c *ConfigStore) func(obj interface{}) {
	return func(obj interface{}) {
		cm := obj.(*corev1.ConfigMap)
		data := cm.Data

		for _, p := range []struct {
			key   string
			field *string
		}{{
			key:   "host",
			field: c.host,
		}, {
			key:   "port",
			field: c.port,
		}, {
			key:   "board_uid",
			field: c.uid,
		}} {
			if raw, ok := data[p.key]; ok {
				*p.field = raw
			} else {
				p.field = nil
			}
		}

		// Update grafana url for all experiment objects
		experimentList := &iter8v1alpha2.ExperimentList{}
		ctx := context.Background()
		err := c.client.List(ctx, experimentList)
		if err != nil {
			c.logger.Error(err, "Fail to list experiments")
			return
		}

		for _, instance := range experimentList.Items {
			c.UpdateGrafanaURL(&instance)
			if err := c.client.Update(ctx, &instance); err != nil {
				// Just log the error for now
				// TODO: log the error in condition
				c.logger.Error(err, "Fail to update grafana url")
			}
		}

		return
	}
}

func getConfigMapNamespace() string {
	if namespace := os.Getenv("POD_NAMESPACE"); namespace != "" {
		return namespace
	}
	return defaultNamespace
}
