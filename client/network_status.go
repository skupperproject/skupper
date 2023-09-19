package client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/domain/kube"
	k8s "github.com/skupperproject/skupper/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (cli *VanClient) NetworkStatus(ctx context.Context) (*[]types.SiteStatusInfo, error) {

	//Checking if the router has been deployed
	_, err := cli.KubeClient.AppsV1().Deployments(cli.Namespace).Get(ctx, types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Skupper is not installed: %s", err)
	}

	configmap, err := k8s.GetConfigMap(types.VanStatusConfigMapName, cli.Namespace, cli.KubeClient)
	if err != nil {
		return nil, err
	}

	sites, err := GetSitesInfoFromConfigMap(configmap)
	if err != nil {
		return nil, err
	}

	return &sites, nil
}

func (cli *VanClient) GetRemoteLinks(ctx context.Context, siteConfig *types.SiteConfig) ([]*types.RemoteLinkInfo, error) {
	cfg, err := cli.getRouterConfig(ctx, cli.Namespace)
	if err != nil {
		return nil, err
	}
	linkHander := kube.NewLinkHandlerKube(cli.Namespace, siteConfig, cfg, cli.KubeClient, cli.RestConfig)
	return linkHander.RemoteLinks(ctx)
}

func GetSitesInfoFromConfigMap(configmap *corev1.ConfigMap) ([]types.SiteStatusInfo, error) {
	if configmap.Data == nil {
		return nil, nil
	} else {

		siteStatusFlowRecords, err := UnmarshalSiteStatus(configmap.Data)
		if err != nil {
			return nil, err
		}

		return siteStatusFlowRecords, nil
	}
}

func UnmarshalSiteStatus(data map[string]string) ([]types.SiteStatusInfo, error) {

	var vanStatus types.VanStatusInfo

	err := json.Unmarshal([]byte(data["VanStatus"]), &vanStatus)
	if err != nil {
		return nil, err
	}

	return vanStatus.SiteStatus, nil
}
