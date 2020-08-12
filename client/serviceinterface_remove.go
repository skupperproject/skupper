package client

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (cli *VanClient) ServiceInterfaceRemove(ctx context.Context, address string) error {
	current, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get("skupper-services", metav1.GetOptions{})
	if err == nil && current.Data != nil {
		jsonDef := current.Data[address]
		if jsonDef == "" {
			return fmt.Errorf("Could not find service %s", address)
		} else {
			delete(current.Data, address)
			_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Update(current)
			if err != nil {
				return fmt.Errorf("Failed to update skupper-services config map: %v", err.Error())
			} else {
				return nil
			}
		}
	} else if errors.IsNotFound(err) {
		return fmt.Errorf("No skupper services defined: %v", err.Error())
	} else if current.Data == nil {
		return fmt.Errorf("Service %s not defined", address)
	} else {
		return fmt.Errorf("Could not retrieve service definitions from configmap 'skupper-services': %s", err.Error())
	}
}
