package utils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	yaml "k8s.io/apimachinery/pkg/util/yaml"
)

func YAMLToJSON(in []byte) ([]byte, error) {
	return yaml.ToJSON(in)
}

func JSONToUnstructured(in []byte) (unstructured.Unstructured, error) {
	obj := unstructured.Unstructured{}
	err := obj.UnmarshalJSON(in)
	if err != nil {
		return unstructured.Unstructured{}, fmt.Errorf("unable to unmarshal JSON: %v", err)
	}
	return obj, nil
}

func YAMLToUnstructured(in []byte) (unstructured.Unstructured, error) {
	json, err := YAMLToJSON(in)
	if err != nil {
		return unstructured.Unstructured{}, fmt.Errorf("unable to convert YAML to JSON: %v", err)
	}
	return JSONToUnstructured(json)
}
