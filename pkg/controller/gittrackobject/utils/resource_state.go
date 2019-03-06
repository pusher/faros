/*
+Copyright 2018 Pusher Ltd.
+
+Licensed under the Apache License, Version 2.0 (the "License");
+you may not use this file except in compliance with the License.
+You may obtain a copy of the License at
+
+    http://www.apache.org/licenses/LICENSE-2.0
+
+Unless required by applicable law or agreed to in writing, software
+distributed under the License is distributed on an "AS IS" BASIS,
+WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
+See the License for the specific language governing permissions and
+limitations under the License.
+*/

package utils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const resourceStateAnnotation = "faros.pusher.com/resource-state"

const (
	// ActiveResourceState represents the default state of a resource
	ActiveResourceState ResourceState = "Active"
	// MarkedForDeletionResourceState represents the state of a resource
	// that is marked for deletion
	MarkedForDeletionResourceState ResourceState = "MarkedForDeletion"
)

// ResourceState represents a resource state
type ResourceState string

// GetResourceState returns the value of the `faros.pusher.com/resource-state`
// annotation, or the default value if one doesn't exist
func GetResourceState(obj *unstructured.Unstructured) (ResourceState, error) {
	annotations := obj.GetAnnotations()
	if data, ok := annotations[resourceStateAnnotation]; ok {
		return validResourceState(ResourceState(data))
	}
	return ActiveResourceState, nil
}

// validResourceState returns whether a given resource state is valid or not
func validResourceState(s ResourceState) (ResourceState, error) {
	switch s {
	case ActiveResourceState, MarkedForDeletionResourceState:
		return s, nil
	default:
		return s, fmt.Errorf("invalid resource state: %s", s)
	}
}
