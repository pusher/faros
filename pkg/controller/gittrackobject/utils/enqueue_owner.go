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
	"log"

	"github.com/pusher/faros/pkg/utils"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

var _ handler.EventHandler = &EnqueueRequestForOwner{}

// EnqueueRequestForOwner enqueues Requests for the Owners of an object.  E.g. the object that created
// the object that was the source of the Event.
//
// This implementation handles both namespaced and non-namespaced resources
type EnqueueRequestForOwner struct {
	NamespacedEnqueueRequestForOwner    *handler.EnqueueRequestForOwner
	NonNamespacedEnqueueRequestForOwner *handler.EnqueueRequestForOwner
	restMapper                          meta.RESTMapper
}

// Create implements EventHandler
func (e *EnqueueRequestForOwner) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	_, namespaced, err := utils.GetAPIResource(e.restMapper, evt.Object.GetObjectKind().GroupVersionKind())
	if err != nil {
		log.Printf("unable to get API resource: %v", err)
	}
	if namespaced {
		e.NamespacedEnqueueRequestForOwner.Create(evt, q)
	} else {
		e.NonNamespacedEnqueueRequestForOwner.Create(evt, q)
	}
}

// Update implements EventHandler
func (e *EnqueueRequestForOwner) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	_, namespaced, err := utils.GetAPIResource(e.restMapper, evt.ObjectNew.GetObjectKind().GroupVersionKind())
	if err != nil {
		log.Printf("unable to get API resource: %v", err)
	}
	if namespaced {
		e.NamespacedEnqueueRequestForOwner.Update(evt, q)
	} else {
		e.NonNamespacedEnqueueRequestForOwner.Update(evt, q)
	}
}

// Delete implements EventHandler
func (e *EnqueueRequestForOwner) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	_, namespaced, err := utils.GetAPIResource(e.restMapper, evt.Object.GetObjectKind().GroupVersionKind())
	if err != nil {
		log.Printf("unable to get API resource: %v", err)
	}
	if namespaced {
		e.NamespacedEnqueueRequestForOwner.Delete(evt, q)
	} else {
		e.NonNamespacedEnqueueRequestForOwner.Delete(evt, q)
	}
}

// Generic implements EventHandler
func (e *EnqueueRequestForOwner) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	_, namespaced, err := utils.GetAPIResource(e.restMapper, evt.Object.GetObjectKind().GroupVersionKind())
	if err != nil {
		log.Printf("unable to get API resource: %v", err)
	}
	if namespaced {
		e.NamespacedEnqueueRequestForOwner.Generic(evt, q)
	} else {
		e.NonNamespacedEnqueueRequestForOwner.Generic(evt, q)
	}
}

var _ inject.Scheme = &EnqueueRequestForOwner{}

// InjectScheme is called by the Controller to provide a singleton scheme to the EnqueueRequestForOwner.
func (e *EnqueueRequestForOwner) InjectScheme(s *runtime.Scheme) error {
	err := e.NamespacedEnqueueRequestForOwner.InjectScheme(s)
	if err != nil {
		return err
	}
	return e.NonNamespacedEnqueueRequestForOwner.InjectScheme(s)
}

var _ inject.Mapper = &EnqueueRequestForOwner{}

// InjectMapper is called by the Controller to provide the rest mapper used by the manager.
func (e *EnqueueRequestForOwner) InjectMapper(m meta.RESTMapper) error {
	e.restMapper = m

	err := e.NamespacedEnqueueRequestForOwner.InjectMapper(m)
	if err != nil {
		return err
	}
	return e.NonNamespacedEnqueueRequestForOwner.InjectMapper(m)
}
