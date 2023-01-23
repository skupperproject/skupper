package client

import (
	"context"
	"fmt"

	"github.com/skupperproject/skupper/pkg/qdr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
)

func isToken(secret *corev1.Secret) bool {
	typename, ok := secret.ObjectMeta.Labels[types.SkupperTypeQualifier]
	return ok && (typename == types.TypeClaimRequest || typename == types.TypeToken)
}

func (cli *VanClient) ConnectorRemove(ctx context.Context, options types.ConnectorRemoveOptions) error {
	secret, err := cli.KubeClient.CoreV1().Secrets(options.SkupperNamespace).Get(ctx, options.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) || (err == nil && !isToken(secret)) {
		return fmt.Errorf("No such link %q", options.Name)
	} else if err != nil {
		return err
	}

	err = cli.removeConnectorRouterConfig(options)
	if err != nil {
		return err
	}

	return kube.DeleteSecret(options.Name, options.SkupperNamespace, cli.KubeClient)

}

func (cli *VanClient) removeConnectorRouterConfig(options types.ConnectorRemoveOptions) error {
	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, options.SkupperNamespace, cli.KubeClient)
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

	_, err = cli.KubeClient.CoreV1().ConfigMaps(options.SkupperNamespace).Update(context.TODO(), configmap, metav1.UpdateOptions{})
	return err
}
