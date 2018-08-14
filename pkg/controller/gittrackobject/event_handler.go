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
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// eventToChannelHandler send all events onto the eventsChan for consumption
// and filtering by the controller's Watch function
type eventToChannelHandler struct {
	eventsChan chan event.GenericEvent
}

// OnAdd implements the cache.ResoureEventHandler interface
func (e *eventToChannelHandler) OnAdd(obj interface{}) {
	e.queueEventForObject(obj)
}

// OnUpdate implements the cache.ResoureEventHandler interface
func (e *eventToChannelHandler) OnUpdate(oldobj, obj interface{}) {
	e.queueEventForObject(obj)
}

// OnDelete implements the cache.ResoureEventHandler interface
func (e *eventToChannelHandler) OnDelete(obj interface{}) {
	e.queueEventForObject(obj)
}

// queueEventForObject sends the event onto the channel
func (e *eventToChannelHandler) queueEventForObject(obj interface{}) {
	var u *unstructured.Unstructured
	var ok bool
	if u, ok = obj.(*unstructured.Unstructured); !ok {
		log.Printf("unable to create unstructured object from interface")
		return
	}

	// Send an event to the events channel
	e.eventsChan <- event.GenericEvent{
		Meta: &metav1.ObjectMeta{
			Name:            u.GetName(),
			Namespace:       u.GetNamespace(),
			OwnerReferences: u.GetOwnerReferences(),
		},
		Object: u,
	}
}
