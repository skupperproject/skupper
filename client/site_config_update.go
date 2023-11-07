package client

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/site"
)

func (cli *VanClient) SiteConfigUpdate(ctx context.Context, config types.SiteConfigSpec) ([]string, error) {
	configmap, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get(ctx, types.SiteConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	// For now, only update router-logging (TODO: update of other options)
	updateLogging := site.UpdateLogging(config, configmap)
	if updateLogging {
		configmap, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Update(ctx, configmap, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
	}
	updates := []string{}
	if updateLogging {
		updated, err := cli.RouterUpdateLogging(ctx, configmap, true)
		if errors.IsNotFound(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if !updated {
			return nil, nil
		}
		updates = append(updates, "router logging")
	}
	return updates, nil

}
