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

package gittrackobject

import (
	"fmt"
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// watch sets up an informer for the object kind and sends events to the
// ReconcileGitTrackObject's eventStream.
func (r *ReconcileGitTrackObject) watch(obj unstructured.Unstructured) error {
	if _, ok := r.informers[informerKey(obj)]; ok {
		// Informer already set up
		return nil
	}

	// Create new informer
	log.Printf("Creating new informer for kind %s", obj.GetObjectKind().GroupVersionKind().Kind)
	informer, err := r.newInformerFromObject(obj)
	if err != nil {
		msg := fmt.Sprintf("error creating informer: %v", err)
		log.Printf(msg)
		return fmt.Errorf(msg)
	}

	// Add event handlers
	informer.AddEventHandler(&eventToChannelHandler{
		eventsChan: r.eventStream,
	})

	// Store and run informer
	r.informers[informerKey(obj)] = informer
	go informer.Run(r.stop)

	return nil
}

// newInformerFromObject takes an Unstructured object and builds a
// SharedIndexInformer for the object's kind.
func (r *ReconcileGitTrackObject) newInformerFromObject(obj unstructured.Unstructured) (cache.SharedIndexInformer, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()

	// TODO restmapping stuff

	// Construct a resource client from the resource
	config := configForGVK(r.config, gvk)
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("unable to create dynamic client: %v", err)
	}
	// TODO get this GVR dynamically
	resourceClient := client.Resource(schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}).Namespace(obj.GetNamespace())

	// Set up empty tweak options
	tweakOptions := func(opts *metav1.ListOptions) {}

	return newSharedIndexInformer(
		resourceClient,
		2*time.Minute,
		&unstructured.Unstructured{},
		cache.Indexers{
			cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
		},
		tweakOptions,
	), nil
}

// newSharedIndexInformer constructs a new SharedIndexInformer for the object.
func newSharedIndexInformer(client dynamic.ResourceInterface, resyncPeriod time.Duration, objType runtime.Object, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.Watch(options)
			},
		},
		objType,
		resyncPeriod,
		indexers,
	)
}

// informerKey creates a unique identifier containing the object's namespace,
// group, version and kind.
//
// This can be used to uniquely identify informers.
func informerKey(obj unstructured.Unstructured) string {
	return fmt.Sprintf("%s:%s", obj.GetNamespace(), obj.GroupVersionKind().String())
}

// configForGVK constructs a rest configuration suitable for constructing a
// dynamic client.
func configForGVK(cfg *rest.Config, gvk schema.GroupVersionKind) *rest.Config {
	c := rest.CopyConfig(cfg)
	c.GroupVersion = &schema.GroupVersion{
		Group:   gvk.Group,
		Version: gvk.Version,
	}
	// Non core group requires API path override
	if gvk.Group != "" {
		c.APIPath = "/apis"
	}
	return c
}
