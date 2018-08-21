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
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// LastAppliedAnnotation is the annotation name used by faros for the last
// applied config
const LastAppliedAnnotation = "faros.pusher.com/last-applied-configuration"

// SetLastAppliedAnnotation sets the last applied configuration annotation on
// the found object
func SetLastAppliedAnnotation(found, child *unstructured.Unstructured) error {
	config, err := child.MarshalJSON()
	if err != nil {
		return fmt.Errorf("unable to marshal JSON: %v", err)
	}

	annotations := found.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[LastAppliedAnnotation] = string(config)
	found.SetAnnotations(annotations)
	return nil
}

// getLastAppliedObject creates an unstructured from the last applied annotation
func getLastAppliedObject(found *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	data := getLastAppliedAnnotation(found)
	if data == nil {
		return &unstructured.Unstructured{}, nil
	}

	obj, err := JSONToUnstructured(data)
	if err != nil {
		return nil, fmt.Errorf("unable to convert data to unstructured: %v", err)
	}
	return &obj, nil
}

// getLastAppliedAnnotation reads the last applied annotation data
func getLastAppliedAnnotation(obj *unstructured.Unstructured) []byte {
	annotations := obj.GetAnnotations()
	if data, ok := annotations[LastAppliedAnnotation]; ok {
		return []byte(data)
	}
	return nil
}
