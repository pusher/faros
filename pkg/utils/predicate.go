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

	"sigs.k8s.io/controller-runtime/pkg/event"
)

// NamespacedPredicate filters events in the given namespace
type NamespacedPredicate struct {
	Namespace string
}

// Create returns true if the event is in the same namespace
func (p NamespacedPredicate) Create(e event.CreateEvent) bool {
	return p.inNamespace(e.Meta.GetNamespace())
}

// Update returns true if the event is in the same namespace
func (p NamespacedPredicate) Update(e event.UpdateEvent) bool {
	return p.inNamespace(e.MetaNew.GetNamespace())
}

// Delete returns true if the event is in the same namespace
func (p NamespacedPredicate) Delete(e event.DeleteEvent) bool {
	return p.inNamespace(e.Meta.GetNamespace())
}

// Generic returns true if the event is in the same namespace
func (p NamespacedPredicate) Generic(e event.GenericEvent) bool {
	return p.inNamespace(e.Meta.GetNamespace())
}

// inNamespace returns whether the given namespace is the same as
// the one the Predicate was configured with
func (p NamespacedPredicate) inNamespace(ns string) bool {
	log.Printf("ns1=%v, ns2=%v", p.Namespace, ns)
	if p.Namespace != "" && ns != "" {
		return p.Namespace == ns
	}
	return true
}

// Match returns whether the given namespace is the same as
// the one the Predicate was configured with
func (p NamespacedPredicate) Match(ns string) bool {
	return p.inNamespace(ns)
}
