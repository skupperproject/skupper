package qdr

import (
	"encoding/json"
	"fmt"
)

func skmanageCommand(operation, entityType, name string, entity interface{}) []string {
	var cmd []string
	properties := map[string]interface{}{}
	cmd = append(cmd, "skmanage", operation, "--type", entityType, "--name", name)
	if entity != nil {
		entityOut, _ := json.Marshal(entity)
		_ = json.Unmarshal(entityOut, &properties)
		for k, v := range properties {
			if k == "name" {
				continue
			}
			cmd = append(cmd, fmt.Sprintf("%s=%v", k, v))
		}
	}
	return cmd
}

func SkmanageCreateCommand(entityType, name string, entity interface{}) []string {
	return skmanageCommand("create", entityType, name, entity)
}

func SkmanageDeleteCommand(entityType, name string) []string {
	return skmanageCommand("delete", entityType, name, nil)
}
