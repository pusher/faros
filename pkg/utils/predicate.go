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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	farosGroupVersion = "faros.pusher.com/v1alpha1"
)

// AnyPredicate takes a list of predicates and returns true if any one of them
// returns true
type AnyPredicate struct {
	elems []predicate.Predicate
}

// NewAnyPredicate constructs a new AnyPredicate
func NewAnyPredicate(elems ...predicate.Predicate) AnyPredicate {
	return AnyPredicate{
		elems: elems,
	}
}

// Create returns true if any of the elements return true
func (p AnyPredicate) Create(e event.CreateEvent) bool {
	for _, el := range p.elems {
		if el.Create(e) {
			return true
		}
	}
	return false
}

// Update returns true if any of the elements return true
func (p AnyPredicate) Update(e event.UpdateEvent) bool {
	for _, el := range p.elems {
		if el.Update(e) {
			return true
		}
	}
	return false
}

// Delete returns true if any of the elements return true
func (p AnyPredicate) Delete(e event.DeleteEvent) bool {
	for _, el := range p.elems {
		if el.Delete(e) {
			return true
		}
	}
	return false
}

// Generic returns true if any of the elements return true
func (p AnyPredicate) Generic(e event.GenericEvent) bool {
	for _, el := range p.elems {
		if el.Generic(e) {
			return true
		}
	}
	return false
}

// OwnerIsClusterGitTrackPredicate filters events to check the owner of the event
// object is a ClusterGitTrack
type OwnerIsClusterGitTrackPredicate struct {
	client client.Client
}

// NewOwnerIsClusterGitTrackPredicate constructs a new OwnerIsClusterGitTrackPredicate
func NewOwnerIsClusterGitTrackPredicate(client client.Client) OwnerIsClusterGitTrackPredicate {
	return OwnerIsClusterGitTrackPredicate{
		client: client,
	}
}

// Create returns true if the event object owner is a ClusterGitTrack
func (p OwnerIsClusterGitTrackPredicate) Create(e event.CreateEvent) bool {
	return p.ownerIsClusterGitTrack(e.Meta.GetOwnerReferences())
}

// Update returns true if the event object owner is a ClusterGitTrack
func (p OwnerIsClusterGitTrackPredicate) Update(e event.UpdateEvent) bool {
	return p.ownerIsClusterGitTrack(e.MetaNew.GetOwnerReferences())
}

// Delete returns true if the event object owner is a ClusterGitTrack
func (p OwnerIsClusterGitTrackPredicate) Delete(e event.DeleteEvent) bool {
	return p.ownerIsClusterGitTrack(e.Meta.GetOwnerReferences())
}

// Generic returns true if the event object owner is a ClusterGitTrack
func (p OwnerIsClusterGitTrackPredicate) Generic(e event.GenericEvent) bool {
	return p.ownerIsClusterGitTrack(e.Meta.GetOwnerReferences())
}

func (p OwnerIsClusterGitTrackPredicate) ownerIsClusterGitTrack(ownerRefs []metav1.OwnerReference) bool {
	for _, ref := range ownerRefs {
		if ref.Kind == "ClusterGitTrack" && ref.APIVersion == farosGroupVersion {
			return true
		}
	}
	return false
}

// OwnerIsGitTrackPredicate filters events to check the owner of the event
// object is a GitTrack
type OwnerIsGitTrackPredicate struct {
	client client.Client
}

// NewOwnerIsGitTrackPredicate constructs a new OwnerIsGitTrackPredicate
func NewOwnerIsGitTrackPredicate(client client.Client) OwnerIsGitTrackPredicate {
	return OwnerIsGitTrackPredicate{
		client: client,
	}
}

// Create returns true if the event object owner is a GitTrack
func (p OwnerIsGitTrackPredicate) Create(e event.CreateEvent) bool {
	return p.ownerIsGitTrack(e.Meta.GetOwnerReferences())
}

// Update returns true if the event object owner is a GitTrack
func (p OwnerIsGitTrackPredicate) Update(e event.UpdateEvent) bool {
	return p.ownerIsGitTrack(e.MetaNew.GetOwnerReferences())
}

// Delete returns true if the event object owner is a GitTrack
func (p OwnerIsGitTrackPredicate) Delete(e event.DeleteEvent) bool {
	return p.ownerIsGitTrack(e.Meta.GetOwnerReferences())
}

// Generic returns true if the event object owner is a GitTrack
func (p OwnerIsGitTrackPredicate) Generic(e event.GenericEvent) bool {
	return p.ownerIsGitTrack(e.Meta.GetOwnerReferences())
}

func (p OwnerIsGitTrackPredicate) ownerIsGitTrack(ownerRefs []metav1.OwnerReference) bool {
	for _, ref := range ownerRefs {
		if ref.Kind == "GitTrack" && ref.APIVersion == farosGroupVersion {
			return true
		}
	}
	return false
}

// OwnersOwnerIsGitTrackPredicate filters events to check the owner's owner of the event
// object is a GitTrack
type OwnersOwnerIsGitTrackPredicate struct {
	client client.Client
}

// NewOwnersOwnerIsGitTrackPredicate constructs a new OwnerIsGitTrackPredicate
func NewOwnersOwnerIsGitTrackPredicate(client client.Client) OwnersOwnerIsGitTrackPredicate {
	return OwnersOwnerIsGitTrackPredicate{
		client: client,
	}
}

// Create returns true if the event object owner's owner is a GitTrack
func (p OwnersOwnerIsGitTrackPredicate) Create(e event.CreateEvent) bool {
	return p.ownersOwnerIsGitTrack(e.Meta.GetOwnerReferences())
}

// Update returns true if the event object owner's owner is a GitTrack
func (p OwnersOwnerIsGitTrackPredicate) Update(e event.UpdateEvent) bool {
	return p.ownersOwnerIsGitTrack(e.MetaNew.GetOwnerReferences())
}

// Delete returns true if the event object owner's owner is a GitTrack
func (p OwnersOwnerIsGitTrackPredicate) Delete(e event.DeleteEvent) bool {
	return p.ownersOwnerIsGitTrack(e.Meta.GetOwnerReferences())
}

// Generic returns true if the event object owner's owner is a GitTrack
func (p OwnersOwnerIsGitTrackPredicate) Generic(e event.GenericEvent) bool {
	return p.ownersOwnerIsGitTrack(e.Meta.GetOwnerReferences())
}

func (p OwnersOwnerIsGitTrackPredicate) ownersOwnerIsGitTrack(ownerRefs []metav1.OwnerReference) bool {
	gtoList := &farosv1alpha1.GitTrackObjectList{}
	err := p.client.List(context.TODO(), gtoList)
	if err != nil {
		// We can't list GTOs so fail closed and ignore the requests
		return false
	}

	// build a set of uids
	gtoSet := make(map[types.UID]farosv1alpha1.GitTrackObject)
	for _, item := range gtoList.Items {
		gtoSet[item.UID] = item
	}

	for _, ref := range ownerRefs {
		if ref.APIVersion != farosGroupVersion {
			continue
		}
		if ref.Kind != "GitTrackObject" {
			continue
		}
		// the gittrackobject should owned by a gittrack or a clustergittrack.
		// fetch the gittrackobject, then run through its owners to see if there's
		// a gittrack in there. We shouldn't see any gittracks from namespaces
		// that we're not managing since cross namespace ownership is disallowed
		gto, inset := gtoSet[ref.UID]
		if !inset {
			// we have an owner reference to a gittrackobject that isn't in our namespace
			// be cautious and don't handle this object
			return false
		}
		for _, gtoOwner := range gto.GetOwnerReferences() {
			if gtoOwner.APIVersion == farosGroupVersion && gtoOwner.Kind == "GitTrack" {
				return true
			}
		}
	}
	return false
}

// OwnersOwnerIsClusterGitTrackPredicate filters events to check the owner's owner of the event
// object is a ClusterGitTrack
type OwnersOwnerIsClusterGitTrackPredicate struct {
	client            client.Client
	includeNamespaced bool
}

// NewOwnersOwnerIsClusterGitTrackPredicate constructs a new OwnerIsClusterGitTrackPredicate
func NewOwnersOwnerIsClusterGitTrackPredicate(client client.Client, includeNamespaced bool) OwnersOwnerIsClusterGitTrackPredicate {
	return OwnersOwnerIsClusterGitTrackPredicate{
		client:            client,
		includeNamespaced: includeNamespaced,
	}
}

// Create returns true if the event object owner's owner is a ClusterGitTrack
func (p OwnersOwnerIsClusterGitTrackPredicate) Create(e event.CreateEvent) bool {
	return p.ownersOwnerIsClusterGitTrack(e.Meta.GetOwnerReferences())
}

// Update returns true if the event object owner's owner is a ClusterGitTrack
func (p OwnersOwnerIsClusterGitTrackPredicate) Update(e event.UpdateEvent) bool {
	return p.ownersOwnerIsClusterGitTrack(e.MetaNew.GetOwnerReferences())
}

// Delete returns true if the event object owner's owner is a ClusterGitTrack
func (p OwnersOwnerIsClusterGitTrackPredicate) Delete(e event.DeleteEvent) bool {
	return p.ownersOwnerIsClusterGitTrack(e.Meta.GetOwnerReferences())
}

// Generic returns true if the event object owner's owner is a ClusterGitTrack
func (p OwnersOwnerIsClusterGitTrackPredicate) Generic(e event.GenericEvent) bool {
	return p.ownersOwnerIsClusterGitTrack(e.Meta.GetOwnerReferences())
}

func (p OwnersOwnerIsClusterGitTrackPredicate) ownersOwnerIsClusterGitTrack(ownerRefs []metav1.OwnerReference) bool {
	gtoList := &farosv1alpha1.GitTrackObjectList{}
	err := p.client.List(context.TODO(), gtoList)
	if err != nil {
		// We can't list GTOs so fail closed and ignore the requests
		return false
	}

	// build a set of uids
	gtoSet := make(map[types.UID]farosv1alpha1.GitTrackObject)
	for _, item := range gtoList.Items {
		gtoSet[item.UID] = item
	}

	for _, ref := range ownerRefs {
		if ref.APIVersion != farosGroupVersion {
			continue
		}
		// ClusterGitTrackObjects can only be owned by ClusterGitTracks, so we
		// if we're owned by a CGTO, then we know that our owners owner is a ClusterGitTrack
		if ref.Kind == "ClusterGitTrackObject" {
			return true
		}
		if ref.Kind != "GitTrackObject" || !p.includeNamespaced {
			continue
		}
		// the gittrackobject should owned by a gittrack or a clustergittrack.
		// fetch the gittrackobject, then run through its owners to see if there's
		// a clustergittrack in there
		gto, inset := gtoSet[ref.UID]
		if !inset {
			// we have an owner reference to a gittrackobject that isn't handled
			// by us. This should never happen, since we should only be watching on
			// these objects if we're in IncludeNamespaced mode and when that happens,
			// we should be watching everything in the cluster
			//
			// Fail closed for now
			return false
		}
		for _, gtoOwner := range gto.GetOwnerReferences() {
			if gtoOwner.APIVersion == farosGroupVersion && gtoOwner.Kind == "ClusterGitTrack" {
				return true
			}
		}
	}
	return false
}
