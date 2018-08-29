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

const updateStrategyAnnotation = "faros.pusher.com/update-strategy"

const (
	// DefaultUpdateStrategy represents the default update strategy where a
	// resource should be updated in-place
	DefaultUpdateStrategy UpdateStrategy = "update"
	// NeverUpdateStrategy represents the update strategy where a resource should
	// never be updated
	NeverUpdateStrategy UpdateStrategy = "never"
	// RecreateUpdateStrategy represents the update strategy where a resource should
	// first be deleted and then created again, rather than updated in-place
	RecreateUpdateStrategy UpdateStrategy = "recreate"
)

// UpdateStrategy represents a valid update strategy
type UpdateStrategy string

// GetUpdateStrategy returns the value of the `faros.pusher.com/update-strategy`
// annotation, or the default value if one doesn't exist
func GetUpdateStrategy(obj *unstructured.Unstructured) (UpdateStrategy, error) {
	annotations := obj.GetAnnotations()
	if data, ok := annotations[updateStrategyAnnotation]; ok {
		return validUpdateStrategy(UpdateStrategy(data))
	}
	return DefaultUpdateStrategy, nil
}

// validUpdateStrategy returns whether a given update strategy is valid or not
func validUpdateStrategy(s UpdateStrategy) (UpdateStrategy, error) {
	switch s {
	case DefaultUpdateStrategy, NeverUpdateStrategy, RecreateUpdateStrategy:
		return s, nil
	default:
		return s, fmt.Errorf("invalid update strategy: %s", s)
	}
}
