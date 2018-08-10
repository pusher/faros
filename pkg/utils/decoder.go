package utils

import (
	yaml "k8s.io/apimachinery/pkg/util/yaml"
)

func YAMLToJSON(in []byte) ([]byte, error) {
	return yaml.ToJSON(in)
}
