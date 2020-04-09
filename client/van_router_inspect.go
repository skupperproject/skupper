package client

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

// VanRouterInspect VAN deployment
func (cli *VanClient) VanRouterInspect(ctx context.Context) (*types.VanRouterInspectResponse, error) {
	vir := &types.VanRouterInspectResponse{}

	current, err := cli.KubeClient.AppsV1().Deployments(cli.Namespace).Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err == nil {
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

		vir.TransportVersion = kube.GetComponentVersion(cli.Namespace, cli.KubeClient, types.TransportContainerName)
		vir.ControllerVersion = kube.GetComponentVersion(cli.Namespace, cli.KubeClient, types.ControllerContainerName)
		vsis, err := cli.VanServiceInterfaceList(context.Background())
		if err != nil {
			vir.ExposedServices = 0
		} else {
			vir.ExposedServices = len(vsis)
		}
	}
	return vir, err

}
