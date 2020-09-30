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
	"context"
	"os"
	"reflect"

	"gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

const (
	configMapName    = "iter8config-notifiers"
	defaultNamespace = "iter8"
)

// RegisterHandler adds event handlers to k8s cache
func (nc *NotificationCenter) RegisterHandler(ctx context.Context, cache cache.Cache) error {
	typeObj, handler := getTypedHandler(nc)

	informer, err := cache.GetInformer(ctx, typeObj)
	if err != nil {
		return err
	}
	informer.AddEventHandler(handler)
	return nil
}

// getTypedHandler returns the typed object and handler used for catching updates in notifier setting
func getTypedHandler(nc *NotificationCenter) (runtime.Object, toolscache.FilteringResourceEventHandler) {
	handler := toolscache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			cm := obj.(*corev1.ConfigMap)
			if cm.GetNamespace() == iter8Namespace() && cm.GetName() == configMapName {
				nc.logger.Info("notifier configmap detected", "name", cm.GetName())
				return true
			}

			return false
		},
		Handler: toolscache.ResourceEventHandlerFuncs{
			AddFunc: updateConfigFromConfigmap(nc),
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldCm, newCm := oldObj.(*corev1.ConfigMap), newObj.(*corev1.ConfigMap)
				if !reflect.DeepEqual(oldCm.Data, newCm.Data) {
					updateConfigFromConfigmap(nc)(newObj)
				}
			},
			DeleteFunc: removeNotifiers(nc),
		},
	}

	return &corev1.ConfigMap{}, handler
}

// updateConfigFromConfigmap update notification
func updateConfigFromConfigmap(nc *NotificationCenter) func(obj interface{}) {
	return func(obj interface{}) {
		nc.m.Lock()
		defer nc.m.Unlock()

		cm := obj.(*corev1.ConfigMap)
		data := cm.Data

		for name, raw := range data {
			newConfig := Config{}
			if err := yaml.Unmarshal([]byte(raw), &newConfig); err != nil {
				nc.logger.Error(err, "Fail to unmarshal cm", "name", name)
				continue
			}

			if err := newConfig.validateAndSetDefault(); err != nil {
				nc.logger.Error(err, "Invalid config for channel", "name", name)
				continue
			}

			originalConfig, ok := nc.Notifiers[name]
			if !ok || !reflect.DeepEqual(originalConfig, newConfig) {
				nc.updateNotifier(name, &newConfig)
			}
		}

		// Remove deleted channels
		for old := range nc.Notifiers {
			if _, ok := data[old]; !ok {
				nc.removeNotifier(old)
			}
		}

		return
	}
}

// removeNotifiers will remove all the notifiers away
func removeNotifiers(nc *NotificationCenter) func(obj interface{}) {
	return func(obj interface{}) {
		nc.m.Lock()
		defer nc.m.Unlock()

		for ntf := range nc.Notifiers {
			nc.removeNotifier(ntf)
		}
	}
}

func iter8Namespace() string {
	retVal := defaultNamespace
	if namespace := os.Getenv("POD_NAMESPACE"); namespace != "" {
		retVal = namespace
	}
	return retVal
}
