package utils

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sigs.k8s.io/yaml"
)

func Encode(outputType string, resource interface{}) (string, error) {

	var result []byte
	var err error

	switch outputType {
	case "json":
		{
			initialMap := make(map[string]interface{})
			jsonData, err := json.MarshalIndent(resource, "", "  ")
			if err != nil {
				return "", err
			}

			err = json.Unmarshal(jsonData, &initialMap)
			if err != nil {
				return "", err
			}

			cleanedMap := omitEmptyValues(initialMap)
			result, err = json.MarshalIndent(cleanedMap, "", "  ")
		}
	case "yaml":
		{
			initialMap := make(map[string]interface{})
			yamlData, err := yaml.Marshal(resource)
			if err != nil {
				return "", err
			}

			err = yaml.Unmarshal(yamlData, &initialMap)
			if err != nil {
				return "", err
			}

			cleanedMap := omitEmptyValues(initialMap)

			result, err = yaml.Marshal(cleanedMap)
		}

	default:
		return "", fmt.Errorf("format %s not supported", outputType)
	}

	return string(result), err
}

func omitEmptyValues(data map[string]interface{}) map[string]interface{} {
	cleaned := make(map[string]interface{})
	for k, v := range data {
		if !isZeroValue(v) {
			if reflect.TypeOf(v).Kind() == reflect.Map {
				if vm, ok := v.(map[string]interface{}); ok {
					v = omitEmptyValues(vm)
				}
			}
			cleaned[k] = v
		}
	}
	return cleaned
}

func isZeroValue(x interface{}) bool {
	if x == nil {
		return true
	}

	v := reflect.ValueOf(x)
	switch v.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		return v.Len() == 0
	case reflect.Struct:
		z := reflect.New(v.Type()).Elem().Interface()
		return reflect.DeepEqual(x, z)
	default:
		z := reflect.Zero(v.Type()).Interface()
		return reflect.DeepEqual(x, z)
	}
}
