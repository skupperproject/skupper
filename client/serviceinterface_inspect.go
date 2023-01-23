package client

import (
	"context"
	jsonencoding "encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

func (cli *VanClient) ServiceInterfaceInspect(ctx context.Context, address string) (*types.ServiceInterface, error) {
	current, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get(ctx, types.ServiceInterfaceConfigMap, metav1.GetOptions{})
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
