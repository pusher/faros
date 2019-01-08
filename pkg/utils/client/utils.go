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
/*
Copyright 2014 The Kubernetes Authors.

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

package client

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

var metadataAccessor = meta.NewAccessor()

// LastAppliedAnnotation is the annotation name used by faros for the last
// applied config
const LastAppliedAnnotation = "faros.pusher.com/last-applied-configuration"

func getNamespacedName(obj runtime.Object) (types.NamespacedName, error) {
	name, err := metadataAccessor.Name(obj)
	if err != nil {
		return types.NamespacedName{}, err
	}
	namespace, err := metadataAccessor.Namespace(obj)
	if err != nil {
		return types.NamespacedName{}, err
	}
	return types.NamespacedName{Namespace: namespace, Name: name}, nil
}

// createApplyAnnotation gets the modified configuration of the object,
// without embedding it again, and then sets it on the object as the annotation.
func createApplyAnnotation(obj runtime.Object, codec runtime.Encoder) error {
	modified, err := getModifiedConfiguration(obj, false, codec)
	if err != nil {
		return err
	}
	return setOriginalConfiguration(obj, modified)
}

// getOriginalConfiguration retrieves the original configuration of the object
// from the annotation, or nil if no annotation was found.
func getOriginalConfiguration(obj runtime.Object) ([]byte, error) {
	annots, err := metadataAccessor.Annotations(obj)
	if err != nil {
		return nil, err
	}

	if annots == nil {
		return nil, nil
	}

	original, ok := annots[LastAppliedAnnotation]
	if !ok {
		return nil, nil
	}

	return []byte(original), nil
}

// setOriginalConfiguration sets the original configuration of the object
// as the annotation on the object for later use in computing a three way patch.
func setOriginalConfiguration(obj runtime.Object, original []byte) error {
	if len(original) < 1 {
		return nil
	}

	annots, err := metadataAccessor.Annotations(obj)
	if err != nil {
		return err
	}

	if annots == nil {
		annots = map[string]string{}
	}

	annots[LastAppliedAnnotation] = string(original)
	return metadataAccessor.SetAnnotations(obj, annots)
}

// getModifiedConfiguration retrieves the modified configuration of the object.
// If annotate is true, it embeds the result as an annotation in the modified
// configuration. If an object was read from the command input, it will use that
// version of the object. Otherwise, it will use the version from the server.
func getModifiedConfiguration(obj runtime.Object, annotate bool, codec runtime.Encoder) ([]byte, error) {
	// First serialize the object without the annotation to prevent recursion,
	// then add that serialization to it as the annotation and serialize it again.
	var modified []byte

	// Otherwise, use the server side version of the object.
	// Get the current annotations from the object.
	annots, err := metadataAccessor.Annotations(obj)
	if err != nil {
		return nil, err
	}

	if annots == nil {
		annots = map[string]string{}
	}

	original := annots[LastAppliedAnnotation]
	delete(annots, LastAppliedAnnotation)
	err = metadataAccessor.SetAnnotations(obj, annots)
	if err != nil {
		return nil, err
	}

	modified, err = runtime.Encode(codec, obj)
	if err != nil {
		return nil, err
	}

	if annotate {
		annots[LastAppliedAnnotation] = string(modified)
		err = metadataAccessor.SetAnnotations(obj, annots)
		if err != nil {
			return nil, err
		}

		modified, err = runtime.Encode(codec, obj)
		if err != nil {
			return nil, err
		}
	}

	// Restore the object to its original condition.
	annots[LastAppliedAnnotation] = original
	if err := metadataAccessor.SetAnnotations(obj, annots); err != nil {
		return nil, err
	}

	return modified, nil
}

// addSourceToErr adds handleResourcePrefix and source string to error message.
// verb is the string like "creating", "deleting" etc.
// source is the filename or URL to the template file(*.json or *.yaml), or stdin to use to handle the resource.
func addSourceToErr(verb string, source string, err error) error {
	if source != "" {
		if statusError, ok := err.(errors.APIStatus); ok {
			status := statusError.Status()
			status.Message = fmt.Sprintf("error when %s %q: %v", verb, source, status.Message)
			return &errors.StatusError{ErrStatus: status}
		}
		return fmt.Errorf("error when %s %q: %v", verb, source, err)
	}
	return err
}

func runDelete(namespace, name string, mapping *meta.RESTMapping, c dynamic.Interface, cascade bool, gracePeriod int, serverDryRun bool) error {
	options := &metav1.DeleteOptions{}
	if gracePeriod >= 0 {
		options = metav1.NewDeleteOptions(int64(gracePeriod))
	}
	if serverDryRun {
		options.DryRun = []string{metav1.DryRunAll}
	}
	policy := metav1.DeletePropagationForeground
	if !cascade {
		policy = metav1.DeletePropagationOrphan
	}
	options.PropagationPolicy = &policy
	return c.Resource(mapping.Resource).Namespace(namespace).Delete(name, options)
}

func addResourceVersion(patch []byte, rv string) ([]byte, error) {
	var patchMap map[string]interface{}
	err := json.Unmarshal(patch, &patchMap)
	if err != nil {
		return nil, err
	}
	u := unstructured.Unstructured{Object: patchMap}
	a, err := meta.Accessor(&u)
	if err != nil {
		return nil, err
	}
	a.SetResourceVersion(rv)

	return json.Marshal(patchMap)
}
