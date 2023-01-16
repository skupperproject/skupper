package client

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	kubeqdr "github.com/skupperproject/skupper/pkg/kube/qdr"
	"k8s.io/apimachinery/pkg/api/errors"
)

func (cli *VanClient) ServiceInterfaceCreate(ctx context.Context, service *types.ServiceInterface) error {
	policy := NewPolicyValidatorAPI(cli)
	res, err := policy.Service(service.Address)
	if err != nil {
		return err
	}
	if !res.Allowed {
		return res.Err()
	}
	owner, err := getRootObject(cli)
	if err == nil {
		err = validateServiceInterface(service, cli)
		if err != nil {
			return err
		}

		tlsSupport := kubeqdr.TlsServiceSupport{Address: service.Address, Credentials: service.TlsCredentials}
		tlsManager := &kubeqdr.TlsManager{KubeClient: cli.KubeClient, Namespace: cli.Namespace}
		err = tlsManager.EnableTlsSupport(tlsSupport)
		if err != nil {
			return err
		}

		return updateServiceInterface(service, false, owner, cli)
	} else if errors.IsNotFound(err) {
		return fmt.Errorf("Skupper is not enabled in namespace '%s'", cli.Namespace)
	} else {
		return err
	}
}
