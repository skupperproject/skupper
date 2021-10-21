package client

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"k8s.io/apimachinery/pkg/api/errors"
)

func (cli *VanClient) ServiceInterfaceCreate(ctx context.Context, service *types.ServiceInterface) error {
	owner, err := getRootObject(cli)
	if err == nil {
		err = validateServiceInterface(service)
		if err != nil {
			return err
		}

		if len(service.TlsCredentials) > 0 {
			serviceSecret, err := cli.CreateSecretForService(service.Address, service.Address, service.TlsCredentials)
			if err != nil {
				return err
			}

			err = cli.AppendSecretToRouter(serviceSecret.Name)
			if err != nil {
				return err
			}

		}

		return updateServiceInterface(service, false, owner, cli)
	} else if errors.IsNotFound(err) {
		return fmt.Errorf("Skupper not initialised in %s", cli.Namespace)
	} else {
		return err
	}
}
