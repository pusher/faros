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

	mergepatch "github.com/evanphx/json-patch"
)

// UpdateChildResource compares the found object with the child object and
// updates the found object if necessary.
func UpdateChildResource(found, child *unstructured.Unstructured) (bool, error) {
	// Create a three way merge patch
	patchBytes, err := createThreeWayMergePatch(found, child)
	if err != nil {
		return false, fmt.Errorf("error calculating patch: %v", err)
	}
	// If no patching to do return now
	if string(patchBytes) == "[]" {
		// nothing to do
		return false, nil
	}

	// Patch the unstructured object
	err = patchUnstructured(found, patchBytes)
	if err != nil {
		return false, fmt.Errorf("unable to patch unstructured object: %v", err)
	}
	return true, nil
}

func patchUnstructured(obj *unstructured.Unstructured, patchBytes []byte) error {
	objJSON, err := obj.MarshalJSON()
	if err != nil {
		return fmt.Errorf("unable to marshal JSON: %v", err)
	}

	patch, err := mergepatch.DecodePatch(patchBytes)
	if err != nil {
		return fmt.Errorf("unable to decode patch: %v", err)
	}

	updatedJSON, err := patch.Apply(objJSON)
	if err != nil {
		return fmt.Errorf("unable to apply patch: %v", err)
	}

	*obj, err = JSONToUnstructured(updatedJSON)
	if err != nil {
		return fmt.Errorf("error converting JSON to unstructured: %v", err)
	}

	return nil
}

func createThreeWayMergePatch(found, child *unstructured.Unstructured) ([]byte, error) {
	original, err := getLastAppliedObject(found)
	if err != nil {
		return nil, fmt.Errorf("unable to get last applied object: %v", err)
	}
	foundJSON, childJSON, originalJSON, err := getJSON(found, child, original)
	if err != nil {
		return nil, fmt.Errorf("error getting json: %v", err)
	}

	patch, err := createThreeWayJSONMergePatch(originalJSON, childJSON, foundJSON)
	if err != nil {
		return nil, fmt.Errorf("unable to create three way merge patch: %v", err)
	}
	return patch, nil
}

func getJSON(found, child, original *unstructured.Unstructured) ([]byte, []byte, []byte, error) {
	foundJSON, err := found.MarshalJSON()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to marshal found JSON: %v", err)
	}
	childJSON, err := child.MarshalJSON()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to marshal found JSON: %v", err)
	}
	originalJSON, err := original.MarshalJSON()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to marshal found JSON: %v", err)
	}
	return foundJSON, childJSON, originalJSON, nil
}
