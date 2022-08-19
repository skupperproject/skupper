package client

import (
	"context"

	"github.com/skupperproject/skupper/api/types"
	"k8s.io/apimachinery/pkg/api/errors"
)

func (cli *VanClient) SiteConfigUpdate(ctx context.Context, config types.SiteConfigSpec) ([]string, error) {
	configmap, _, err := cli.ConfigMapManager(cli.Namespace).GetConfigMap(types.SiteConfigMapName)
	if err != nil {
		return nil, err
	}
	// For now, only update router-logging and/or router-debug-mode (TODO: update of other options)
	latestLogging := RouterLogConfigToString(config.Router.Logging)
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
		configmap, err = cli.ConfigMapManager(cli.Namespace).UpdateConfigMap(configmap)
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
