package client

import (
	"context"
	"fmt"

	"github.com/skupperproject/skupper/pkg/qdr"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

func isToken(secret *corev1.Secret) bool {
	typename, ok := secret.ObjectMeta.Labels[types.SkupperTypeQualifier]
	return ok && (typename == types.TypeClaimRequest || typename == types.TypeToken)
}

func (cli *VanClient) ConnectorRemove(ctx context.Context, options types.ConnectorRemoveOptions) error {
	secret, _, err := cli.SecretManager(options.SkupperNamespace).GetSecret(options.Name)
	if errors.IsNotFound(err) || (err == nil && !isToken(secret)) {
		return fmt.Errorf("No such link %q", options.Name)
	} else if err != nil {
		return err
	}

	err = cli.removeConnectorRouterConfig(options)
	if err != nil {
		return err
	}

	return kube.DeleteSecret(options.Name, cli.SecretManager(options.SkupperNamespace))

}

func (cli *VanClient) removeConnectorRouterConfig(options types.ConnectorRemoveOptions) error {
	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, cli.ConfigMapManager(options.SkupperNamespace))
	if err != nil {
		return err
	}
	current, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return err
	}
	current.RemoveConnector(options.Name)
	current.RemoveSslProfile(options.Name + "-profile")

	_, err = current.UpdateConfigMap(configmap)
	if err != nil {
		return err
	}

	_, err = cli.ConfigMapManager(options.SkupperNamespace).UpdateConfigMap(configmap)
	return err
}
