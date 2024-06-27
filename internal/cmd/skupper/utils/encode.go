package utils

import (
	"encoding/json"
	"fmt"
	"sigs.k8s.io/yaml"
)

func Encode(outputType string, resource interface{}) (string, error) {

	var result []byte
	var err error

	switch outputType {
	case "json":
		result, err = json.MarshalIndent(resource, "", "  ")
	case "yaml":
		result, err = yaml.Marshal(resource)
	default:
		return "", fmt.Errorf("format %s not supported", outputType)
	}

	return string(result), err
}
