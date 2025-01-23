package qdr

import (
	"encoding/json"
	"fmt"
)

func skmanageCommand(operation, entityType, name string, router string, edge bool, entity interface{}) []string {
	var cmd []string
	properties := map[string]interface{}{}
	cmd = append(cmd, "skmanage", operation, "--type", entityType)
	if name != "" {
		cmd = append(cmd, "--name", name)
	}
	if router != "" {
		flag := "--router"
		if edge {
			flag = "--edge-router"
		}
		cmd = append(cmd, flag, router)
	}
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
	return skmanageCommand("create", entityType, name, "", false, entity)
}

func SkmanageDeleteCommand(entityType, name string) []string {
	return skmanageCommand("delete", entityType, name, "", false, nil)
}

func SkmanageQueryCommand(entityType, routerId string, edge bool, name string) []string {
	return skmanageCommand("query", entityType, name, routerId, edge, nil)
}
