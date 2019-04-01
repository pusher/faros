/*
Copyright 2018 Pusher Ltd.

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

package utils

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteAll lists and deletes all resources
func DeleteAll(cfg *rest.Config, timeout time.Duration, objLists ...runtime.Object) {
	c, err := client.New(rest.CopyConfig(cfg), client.Options{})
	g.Expect(err).ToNot(g.HaveOccurred())
	for _, objList := range objLists {
		g.Eventually(func() error {
			return c.List(context.TODO(), objList)
		}, timeout).Should(g.Succeed())
		objs, err := apimeta.ExtractList(objList)
		g.Expect(err).ToNot(g.HaveOccurred())
		errs := make(chan error, len(objs))
		for _, obj := range objs {
			go func(o runtime.Object) {
				errs <- deleteObj(c, timeout, o)
			}(obj)
		}
		for range objs {
			g.Expect(<-errs).ToNot(g.HaveOccurred())
		}
	}
}

func deleteObj(c client.Client, timeout time.Duration, obj runtime.Object) error {
	metaAccessor, err := apimeta.Accessor(obj)
	if err != nil {
		return err
	}

	err = c.Delete(context.TODO(), obj)
	if err != nil {
		return err
	}

	checkDeleted := func() error {
		key := types.NamespacedName{Namespace: metaAccessor.GetNamespace(), Name: metaAccessor.GetName()}

		err := c.Get(context.TODO(), key, obj)
		if err != nil && errors.IsNotFound(err) {
			// Object has been deleted
			return nil
		} else if err != nil {
			return err
		}

		if metaAccessor.GetDeletionTimestamp() == nil {
			return fmt.Errorf("Object has not been deleted")
		}
		if len(metaAccessor.GetFinalizers()) > 0 {
			return fmt.Errorf("Object has remaining Finalizers: %v", metaAccessor.GetFinalizers())
		}
		// If the object has deletion timestamp and no finalizers,
		// it shouldn't exist and we shouldn't get here
		panic(fmt.Sprintf("Unexpected Object state: %+v", obj))
	}

	g.Eventually(checkDeleted, timeout).Should(g.Succeed())
	return nil
}
