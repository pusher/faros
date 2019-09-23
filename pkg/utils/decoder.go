package utils

import (
	"bytes"
	"fmt"

	goyaml "gopkg.in/yaml.v2"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	yamlSeparator = "\n---\n"
)

// list represents a Kubernetes List object which can contain multiple other
// resources in an Items slice
type list struct {
	// ApiVersion represent the lists ApiVersion
	// This should always be "v1"
	APIVersion string `yaml:"apiVersion"`

	// Kind represents the lists Kind
	// This should always be "List"
	Kind string `yaml:"kind"`

	// Items represents the list of items that the List holds
	// Convert all items to `map[string]interface{}` for correct yaml marshalling
	Items []map[string]interface{} `yaml:"items"`
}

// yamlToJSON converts a byte slice containing yaml into a byte slice containing
// json
func yamlToJSON(in []byte) ([]byte, error) {
	return yaml.ToJSON(in)
}

// JSONToUnstructured converts a raw json document into an Unstructured object
func JSONToUnstructured(in []byte) (unstructured.Unstructured, error) {
	obj := unstructured.Unstructured{}
	err := obj.UnmarshalJSON(in)
	if err != nil {
		return unstructured.Unstructured{}, fmt.Errorf("unable to unmarshal JSON: %v", err)
	}
	return obj, nil
}

// YAMLToUnstructured converts a raw yaml document into an Unstructured object
func YAMLToUnstructured(in []byte) (u unstructured.Unstructured, err error) {
	var data []byte
	yamlList := splitYAML(in)
	if len(yamlList) != 1 {
		data, err = toList(yamlList)
		if err != nil {
			return u, fmt.Errorf("unable to split file: %v", err)
		}
	} else {
		data = yamlList[0]
	}
	json, err := yamlToJSON(data)
	if err != nil {
		return u, fmt.Errorf("unable to convert YAML to JSON: %v", err)
	}
	return JSONToUnstructured(json)
}

// YAMLToUnstructuredSlice converts a raw yaml document into a slice of pointers to Unstructured objects
func YAMLToUnstructuredSlice(in []byte) ([]*unstructured.Unstructured, error) {
	var us []*unstructured.Unstructured
	for _, yaml := range splitYAML(in) {
		u, err := YAMLToUnstructured(yaml)
		if err != nil {
			// unable to parse properly, bail
			return us, err
		}
		if u.IsList() {
			err = u.EachListItem(func(obj runtime.Object) error {
				o, ok := obj.(*unstructured.Unstructured)
				if !ok {
					kind := obj.GetObjectKind().GroupVersionKind().Kind
					return fmt.Errorf("invalid resource of Kind %s", kind)
				}
				us = append(us, o)
				return nil
			})
			if err != nil {
				return us, err
			}
		} else {
			us = append(us, &u)
		}
	}
	return us, nil
}

// splitYAML will take raw yaml from a file and split yaml documents on the
// yaml separator `---`, returning a list of documents in the original input
func splitYAML(in []byte) (out [][]byte) {
	split := bytes.Split(in, []byte(yamlSeparator))
	for _, data := range split {
		if len(data) > 0 {
			out = append(out, data)
		}
	}
	return
}

// toList converts a slice of yaml documents into a Kubernetes List kind
func toList(in [][]byte) ([]byte, error) {
	items := []map[string]interface{}{}
	for _, item := range in {
		data := make(map[string]interface{})
		err := goyaml.Unmarshal(item, data)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshal input: %v", err)
		}
		items = append(items, data)
	}
	return goyaml.Marshal(&list{
		APIVersion: "v1",
		Kind:       "List",
		Items:      items,
	})
}
