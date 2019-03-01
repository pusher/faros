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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// EventToChannelHandler send all events onto the EventsChan for consumption
// and filtering by the controller's Watch function
type EventToChannelHandler struct {
	EventsChan chan event.GenericEvent
}

// OnAdd implements the cache.ResoureEventHandler interface
func (e *EventToChannelHandler) OnAdd(obj interface{}) {
	e.queueEventForObject(obj)
}

// OnUpdate implements the cache.ResoureEventHandler interface
func (e *EventToChannelHandler) OnUpdate(oldobj, obj interface{}) {
	e.queueEventForObject(obj)
}

// OnDelete implements the cache.ResoureEventHandler interface
func (e *EventToChannelHandler) OnDelete(obj interface{}) {
	e.queueEventForObject(obj)
}

// queueEventForObject sends the event onto the channel
func (e *EventToChannelHandler) queueEventForObject(obj interface{}) {
	if obj == nil {
		// Can't do anything here
		return
	}
	var u *unstructured.Unstructured
	var ok bool
	if u, ok = obj.(*unstructured.Unstructured); !ok {
		log.Printf("unable to create unstructured object from interface: %+v", obj)
		return
	}

	// Send an event to the events channel
	e.EventsChan <- event.GenericEvent{
		Meta: &metav1.ObjectMeta{
			Name:            u.GetName(),
			Namespace:       u.GetNamespace(),
			OwnerReferences: u.GetOwnerReferences(),
		},
		Object: u,
	}
}
