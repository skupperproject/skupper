package client

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func (cli *VanClient) ConnectorRemove(ctx context.Context, options types.ConnectorRemoveOptions) error {
	secret, err := cli.KubeClient.CoreV1().Secrets(options.SkupperNamespace).Get(options.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return fmt.Errorf("No such link %q", options.Name)
	} else if err != nil {
		return err
	}
	if secret.ObjectMeta.Labels[types.SkupperTypeQualifier] == types.TypeClaimRequest {
		kube.DeleteSecret(options.Name, options.SkupperNamespace, cli.KubeClient)
		return nil
	} else if secret.ObjectMeta.Labels[types.SkupperTypeQualifier] == types.TypeToken {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			deployment, err := kube.GetDeployment(types.TransportDeploymentName, options.SkupperNamespace, cli.KubeClient)
			if err != nil {
				return err
			}
			configmap, err := kube.GetConfigMap(types.TransportConfigMapName, options.SkupperNamespace, cli.KubeClient)
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
				_, err = cli.KubeClient.AppsV1().Deployments(options.SkupperNamespace).Update(deployment)
				if err != nil {
					return err
				}
				//delete secret last, so that if there is a conflict updating deployment, it
				//will still exist and retry will work
				return kube.DeleteSecret(options.Name, options.SkupperNamespace, cli.KubeClient)
			}
			return nil
		})
	} else {
		return fmt.Errorf("No such link %q", options.Name)
	}
}
