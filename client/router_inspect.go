package client

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

// RouterInspect VAN deployment
func (cli *VanClient) RouterInspect(ctx context.Context) (*types.RouterInspectResponse, error) {
	vir := &types.RouterInspectResponse{}

	current, err := cli.KubeClient.AppsV1().Deployments(cli.Namespace).Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err == nil {
		siteConfig, err := cli.SiteConfigInspect(ctx, nil)
		if err == nil && siteConfig != nil {
			vir.Status.SiteName = siteConfig.Spec.SkupperName
		}
		vir.Status.Mode = string(qdr.GetTransportMode(current))
		vir.Status.TransportReadyReplicas = current.Status.ReadyReplicas
		connected, err := qdr.GetConnectedSites(vir.Status.Mode == types.TransportModeEdge, cli.Namespace, cli.KubeClient, cli.RestConfig)
		for i := 0; i < 5 && err != nil; i++ {
			time.Sleep(500 * time.Millisecond)
			connected, err = qdr.GetConnectedSites(vir.Status.Mode == types.TransportModeEdge, cli.Namespace, cli.KubeClient, cli.RestConfig)
		}
		if err != nil {
			return nil, err
		} else {
			vir.Status.ConnectedSites = connected
		}

		vir.TransportVersion = kube.GetComponentVersion(cli.Namespace, cli.KubeClient, types.TransportComponentName, types.TransportContainerName)
		vir.ControllerVersion = kube.GetComponentVersion(cli.Namespace, cli.KubeClient, types.ControllerComponentName, types.ControllerContainerName)
		vsis, err := cli.ServiceInterfaceList(context.Background())
		if err != nil {
			vir.ExposedServices = 0
		} else {
			vir.ExposedServices = len(vsis)
		}
	}
	return vir, err

}
