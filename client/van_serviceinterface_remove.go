package client

import (
	"context"
	jsonencoding "encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

func removeServiceInterfaceTarget(serviceName string, targetName string, cli *VanClient) error {
	current, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get("skupper-services", metav1.GetOptions{})
	if err == nil {
		jsonDef := current.Data[serviceName]
		if jsonDef == "" {
			return fmt.Errorf("Could not find entry for service interface %s", serviceName)
		} else {
			service := types.ServiceInterface{}
			err = jsonencoding.Unmarshal([]byte(jsonDef), &service)
			if err != nil {
				return fmt.Errorf("Failed to read json for service interface %s: %s", serviceName, err)
			} else {
				modified := false
				targets := []types.ServiceInterfaceTarget{}
				for _, t := range service.Targets {
					if t.Name == targetName || (t.Name == "" && targetName == serviceName) {
						modified = true
					} else {
						targets = append(targets, t)
					}
				}
				if !modified {
					return fmt.Errorf("Could not find target %s for service interface %s", targetName, serviceName)
				}
				if len(targets) > 0 {
					service.Targets = targets
					encoded, err := jsonencoding.Marshal(service)
					if err != nil {
						return fmt.Errorf("Failed to create json for service interface: %s", err)
					} else {
						current.Data[serviceName] = string(encoded)
					}
				} else {
					delete(current.Data, serviceName)
				}
			}
		}
		_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Update(current)
		if err != nil {
			return fmt.Errorf("Failed to update skupper-services config map: ", err.Error())
		}
	} else if errors.IsNotFound(err) {
		return fmt.Errorf("No skupper service interfaces defined: ", err.Error())
	} else {
		return fmt.Errorf("Could not retrieve service interfaces from configmap 'skupper-services'", err)
	}
	return nil
}

func (cli *VanClient) VanServiceInterfaceRemove(ctx context.Context, targetType string, targetName string, address string) error {
	if targetType == "deployment" || targetType == "statefulset" {
		if address == "" {
			err := removeServiceInterfaceTarget(targetName, targetName, cli)
			return err
		} else {
			err := removeServiceInterfaceTarget(address, targetName, cli)
			return err
		}
	} else if targetType == "pods" {
		return fmt.Errorf("Target type for service interface not yet implemented")
	} else {
		return fmt.Errorf("Unsupported target type for service itnerface", targetType)
	}
}
