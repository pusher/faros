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

	"github.com/mattbaird/jsonpatch"
)

var blacklistedPaths = []string{
	"/metadata/creationTimestamp",
}

// createThreeWayJSONMergePatch takes three JSON documents and compares them.
// It retures a JSON patch document which will merge changes to the modified
// document into the current document.
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
	addAndChange = filterBlacklistedPaths(addAndChange)

	del, err := jsonpatch.CreatePatch(original, modified)
	if err != nil {
		return nil, fmt.Errorf("error comparing last applied and desired state: %v", err)
	}
	// Only keep deletion
	del = keepOrDeleteRemoveInPatch(del, true)

	patch, err := mergePatchToJSON(del, addAndChange)
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

// filterBlacklistedPaths filters blacklisted paths from the given slice of operations
func filterBlacklistedPaths(patch []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	allowed := []jsonpatch.JsonPatchOperation{}
	for _, po := range patch {
		if y := blacklistedPath(po.Path); !y {
			allowed = append(allowed, po)
		}
	}
	return allowed
}

// blacklistedPath returns whether a path is blacklisted or not
func blacklistedPath(p string) bool {
	for _, bp := range blacklistedPaths {
		if p == bp {
			return true
		}
	}
	return false
}

// mergePatchToJSON adds creates a JSON patch document with all paths from
// both a and b, b taking precedent over a
func mergePatchToJSON(a, b []jsonpatch.JsonPatchOperation) ([]byte, error) {
	paths := make(map[string]struct{})
	for _, p := range b {
		paths[p.Path] = struct{}{}
	}
	for _, p := range a {
		if _, ok := paths[p.Path]; !ok {
			b = append(b, p)
		}
	}
	return json.Marshal(b)
}
