package client

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

func (cli *VanClient) SiteConfigUpdate(ctx context.Context, config types.SiteConfigSpec) (bool, error) {
	configmap, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get("skupper-site", metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	//For now, only update logging (TODO: update of other options)
	latest := RouterLogConfigToString(config.RouterLogging)
	if configmap.Data["router-logging"] == latest {
		return false, nil
	}
	configmap.Data["router-logging"] = latest
	configmap, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Update(configmap)
	if err != nil {
		return false, err
	}
	updated, err := cli.RouterUpdateLogging(ctx, configmap, true)
	if errors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return updated, nil
}
