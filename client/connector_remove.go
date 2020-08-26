package client

import (
	"context"
	"fmt"

	"k8s.io/client-go/util/retry"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func (cli *VanClient) ConnectorRemove(ctx context.Context, options types.ConnectorRemoveOptions) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		deployment, err := kube.GetDeployment(types.TransportDeploymentName, options.SkupperNamespace, cli.KubeClient)
		if err != nil {
			return err
		}
		configmap, err := kube.GetConfigMap("skupper-internal", options.SkupperNamespace, cli.KubeClient)
		if err != nil {
			return err
		}
		current, err := qdr.GetRouterConfigFromConfigMap(configmap)
		if err != nil {
			return err
		}

		found, connector := current.RemoveConnector(options.Name)
		if found || options.ForceCurrent {
			if connector.SslProfile != "" {
				current.RemoveSslProfile(connector.SslProfile)
			}
			_, err := current.UpdateConfigMap(configmap)
			if err != nil {
				return err
			}
			_, err = cli.KubeClient.CoreV1().ConfigMaps(options.SkupperNamespace).Update(configmap)
			if err != nil {
				return err
			}
			kube.RemoveSecretVolumeForDeployment(options.Name, deployment, 0)
			kube.DeleteSecret(options.Name, options.SkupperNamespace, cli.KubeClient)
			_, err = cli.KubeClient.AppsV1().Deployments(options.SkupperNamespace).Update(deployment)
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("Failed to update skupper-router deployment: %w", err)
	}
	return nil
}
