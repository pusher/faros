package client

import (
	"io"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// UnstructuredStatusAwareJSONScheme is a wrapper around UnstructuredJSONScheme that can convert a metav1.Status response
// so that Faros logs the API's status message properly.
var UnstructuredStatusAwareJSONScheme runtime.Codec = unstructuredStatusAwareJSONScheme{}

type unstructuredStatusAwareJSONScheme struct{}

func (s unstructuredStatusAwareJSONScheme) Decode(data []byte, _ *schema.GroupVersionKind, obj runtime.Object) (runtime.Object, *schema.GroupVersionKind, error) {
	respObj, gvk, err := unstructured.UnstructuredJSONScheme.Decode(data, nil, obj)

	if err != nil {
		return nil, nil, err
	}

	// If the Kind is Status, we need to decode the data into the proper struct so that the go-client returns the actual error message.
	if gvk.Kind == "Status" {
		var status metav1.Status
		_, _, _ = unstructured.UnstructuredJSONScheme.Decode(data, nil, &status)
		return &status, gvk, nil
	}

	return respObj, gvk, nil
}

func (unstructuredStatusAwareJSONScheme) Encode(obj runtime.Object, w io.Writer) error {
	return unstructured.UnstructuredJSONScheme.Encode(obj, w)
}
