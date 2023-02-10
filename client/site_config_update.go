package client

import (
	"context"

	"github.com/skupperproject/skupper/pkg/qdr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

func (cli *VanClient) SiteConfigUpdate(ctx context.Context, config types.SiteConfigSpec) ([]string, error) {
	configmap, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get(types.SiteConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	// For now, only update router-logging and/or router-debug-mode (TODO: update of other options)
	latestLogging := qdr.RouterLogConfigToString(config.Router.Logging)
	updateLogging := false
	if configmap.Data[SiteConfigRouterLoggingKey] != latestLogging {
		configmap.Data[SiteConfigRouterLoggingKey] = latestLogging
		updateLogging = true
	}
	updateDebugMode := false
	if configmap.Data[SiteConfigRouterDebugModeKey] != config.Router.DebugMode {
		configmap.Data[SiteConfigRouterDebugModeKey] = config.Router.DebugMode
		updateDebugMode = true
	}
	if updateLogging || updateDebugMode {
		configmap, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Update(configmap)
		if err != nil {
			return nil, err
		}
	}
	updates := []string{}
	if updateLogging {
		updated, err := cli.RouterUpdateLogging(ctx, configmap, !updateDebugMode)
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
	if updateDebugMode {
		updated, err := cli.RouterUpdateDebugMode(ctx, configmap)
		if errors.IsNotFound(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if !updated {
			return nil, nil
		}
		updates = append(updates, "router debug mode")
	}
	return updates, nil
}
