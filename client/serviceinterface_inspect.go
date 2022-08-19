package client

import (
	"context"
	jsonencoding "encoding/json"
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"k8s.io/apimachinery/pkg/api/errors"
)

func (cli *VanClient) ServiceInterfaceInspect(ctx context.Context, address string) (*types.ServiceInterface, error) {
	current, _, err := cli.ConfigMapManager(cli.Namespace).GetConfigMap(types.ServiceInterfaceConfigMap)
	if err == nil {
		jsonDef := current.Data[address]
		if jsonDef == "" {
			return nil, nil
		} else {
			service := types.ServiceInterface{}
			err = jsonencoding.Unmarshal([]byte(jsonDef), &service)
			if err != nil {
				return nil, fmt.Errorf("Failed to read json for service definition %s: %s", address, err)
			} else {
				return &service, nil
			}
		}
	} else if errors.IsNotFound(err) {
		return nil, nil
	} else {
		return nil, fmt.Errorf("Could not retrieve service interface definition: %s", err)
	}
}
