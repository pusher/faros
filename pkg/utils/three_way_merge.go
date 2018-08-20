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
	"encoding/json"
	"fmt"

	merge "github.com/evanphx/json-patch"
	"github.com/mattbaird/jsonpatch"
)

func createThreeWayJSONMergePatch(original, modified, current []byte) ([]byte, error) {
	if len(original) == 0 {
		original = []byte(`{}`)
	}
	if len(modified) == 0 {
		modified = []byte(`{}`)
	}
	if len(current) == 0 {
		current = []byte(`{}`)
	}

	addAndChange, err := jsonpatch.CreatePatch(current, modified)
	if err != nil {
		return nil, fmt.Errorf("error comparing current and desired state: %v", err)
	}
	// Only keep addition and changes
	addAndChange = keepOrDeleteRemoveInPatch(addAndChange, false)

	del, err := jsonpatch.CreatePatch(original, modified)
	if err != nil {
		return nil, fmt.Errorf("error comparing last applied and desired state: %v", err)
	}
	// Only keep deletion
	del = keepOrDeleteRemoveInPatch(del, true)

	addAndChangeJSON, err := json.Marshal(addAndChange)
	if err != nil {
		return nil, fmt.Errorf("error marshalling JSON: %v", err)
	}
	delJSON, err := json.Marshal(del)
	if err != nil {
		return nil, fmt.Errorf("error marshalling JSON: %v", err)
	}

	// TODO: handle conflicts
	// hasConflicts, err := mergepatch.HasConflicts(addAndChange, del)
	// if err != nil {
	// 	return nil, fmt.Errorf("error checking conflicts: %v", err)
	// }
	// if hasConflicts {
	// 	return nil, mergepatch.NewErrConflict(mergepatch.ToYAMLOrError(addAndChange), mergepatch.ToYAMLOrError(del))
	// }

	patch, err := merge.MergePatch(delJSON, addAndChangeJSON)
	if err != nil {
		return nil, fmt.Errorf("error merging patches: %v", err)
	}

	return patch, nil
}

// keepOrDeleteRemoveInPatch will keep only the null value and delete all the others,
// if keepNull is true. Otherwise, it will delete all the null value and keep the others.
func keepOrDeleteRemoveInPatch(patch []jsonpatch.JsonPatchOperation, keepRemove bool) []jsonpatch.JsonPatchOperation {
	filteredPatch := []jsonpatch.JsonPatchOperation{}
	for _, op := range patch {
		if keepRemove {
			if op.Operation == "remove" {
				filteredPatch = append(filteredPatch, op)
			}
		} else {
			if op.Operation != "remove" {
				filteredPatch = append(filteredPatch, op)
			}
		}
	}

	return filteredPatch
}
