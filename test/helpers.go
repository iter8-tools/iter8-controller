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
	"context"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	StockImageName = "villardl/stock-60d7d0dbe2427b272042abacd4e1e644"
	interval       = 1 * time.Second
	timeout        = 1 * time.Minute
)

// Hook defines a function with a client.
type Hook func(ctx context.Context, cl client.Client) error

type InStateFunc func(obj runtime.Object) (bool, error)

// WaitForState polls the status of the object called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout
func WaitForState(ctx context.Context, cl client.Client, obj runtime.Object, inState InStateFunc) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	key := client.ObjectKey{Namespace: accessor.GetNamespace(), Name: accessor.GetName()}
	waitErr := wait.PollImmediate(interval, timeout, func() (bool, error) {
		var err error
		err = cl.Get(ctx, key, obj)
		if err != nil {
			return true, err
		}
		return inState(obj)
	})

	if waitErr != nil {
		return errors.Wrapf(waitErr, "object %q is not in desired state, got: %+v", accessor.GetName(), obj)
	}
	return nil
}

// WaitForDelete polls the obj existence
func WaitForDelete(ctx context.Context, cl client.Client, obj runtime.Object) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	key := client.ObjectKey{Namespace: accessor.GetNamespace(), Name: accessor.GetName()}
	waitErr := wait.PollImmediate(interval, timeout, func() (bool, error) {
		var err error
		err = cl.Get(ctx, key, obj)
		if err != nil {
			return true, nil
		}
		return false, nil
	})

	if waitErr != nil {
		return errors.Wrapf(waitErr, "object %q is not deleted, got: %+v", accessor.GetName(), obj)
	}
	return nil
}

func DeleteObject(obj runtime.Object) Hook {
	return func(ctx context.Context, cl client.Client) error {
		if err := cl.Delete(ctx, obj); err != nil {
			return err
		}
		if err := WaitForDelete(ctx, cl, obj); err != nil {
			return err
		}
		return nil
	}
}

func WantAllStates(stateFns ...InStateFunc) InStateFunc {
	return func(obj runtime.Object) (bool, error) {
		for _, fn := range stateFns {
			ok, err := fn(obj)
			if !ok || err != nil {
				return ok, err
			}
		}
		return true, nil
	}
}

func CheckObjectDeleted(objects ...runtime.Object) Hook {
	return func(ctx context.Context, cl client.Client) error {
		for _, obj := range objects {
			if err := WaitForDelete(ctx, cl, obj); err != nil {
				return err
			}
		}
		return nil
	}
}
