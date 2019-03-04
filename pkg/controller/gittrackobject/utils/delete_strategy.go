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

const deleteStrategyAnnotation = "faros.pusher.com/delete-strategy"

const (
	// DefaultDeleteStrategy represents the default delete strategy where a
	// resource will not be deleted
	DefaultDeleteStrategy DeleteStrategy = "ignore"
	// DeleteResourceStrategy represents the delete strategy where a resource should
	// be deleted
	DeleteResourceStrategy DeleteStrategy = "delete"
)

// DeleteStrategy represents a valid delete strategy
type DeleteStrategy string

// GetDeleteStrategy returns the value of the `faros.pusher.com/delete-strategy`
// annotation, or the default value if one doesn't exist
func GetDeleteStrategy(obj *unstructured.Unstructured) (DeleteStrategy, error) {
	annotations := obj.GetAnnotations()
	if data, ok := annotations[deleteStrategyAnnotation]; ok {
		return validDeleteStrategy(DeleteStrategy(data))
	}
	return DefaultDeleteStrategy, nil
}

// validDeleteStrategy returns whether a given delete strategy is valid or not
func validDeleteStrategy(s DeleteStrategy) (DeleteStrategy, error) {
	switch s {
	case DefaultDeleteStrategy, DeleteResourceStrategy:
		return s, nil
	default:
		return s, fmt.Errorf("invalid delete strategy: %s", s)
	}
}
