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

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// OwnerInNamespacePredicate filters events to check the owner of the event
// object is in the controller's namespace
type OwnerInNamespacePredicate struct {
	client client.Client
}

// Create returns true if the event object's owner is in the same namespace
func (p OwnerInNamespacePredicate) Create(e event.CreateEvent) bool {
	return p.ownerInNamespace(e.Meta.GetOwnerReferences())
}

// Update returns true if the event object's owner is in the same namespace
func (p OwnerInNamespacePredicate) Update(e event.UpdateEvent) bool {
	return p.ownerInNamespace(e.MetaNew.GetOwnerReferences())
}

// Delete returns true if the event object's owner is in the same namespace
func (p OwnerInNamespacePredicate) Delete(e event.DeleteEvent) bool {
	return p.ownerInNamespace(e.Meta.GetOwnerReferences())
}

// Generic returns true if the event object's owner is in the same namespace
func (p OwnerInNamespacePredicate) Generic(e event.GenericEvent) bool {
	return p.ownerInNamespace(e.Meta.GetOwnerReferences())
}

// ownerInNamespace returns true if the the GitTrack owner is in the namespace
// managed by the controller
//
// This works on the premise that listing objects from the client will only
// return those in its cache.
// When it is restricted to a namespace this should only be the GitTracks
// in the namespace the controller is managing.
func (p OwnerInNamespacePredicate) ownerInNamespace(ownerRefs []metav1.OwnerReference) bool {
	gtList := &farosv1alpha1.GitTrackList{}
	err := p.client.List(context.TODO(), &client.ListOptions{}, gtList)
	if err != nil {
		// We can't list CGTOs so fail closed and ignore the requests
		return false
	}
	for _, ref := range ownerRefs {
		if ref.Kind == "GitTrack" && ref.APIVersion == "faros.pusher.com/v1alpha1" {
			for _, gt := range gtList.Items {
				if ref.UID == gt.UID {
					return true
				}
			}
		}
	}
	return false
}

// NewOwnerInNamespacePredicate constructs a new OwnerInNamespacePredicate
func NewOwnerInNamespacePredicate(client client.Client) OwnerInNamespacePredicate {
	return OwnerInNamespacePredicate{
		client: client,
	}
}
