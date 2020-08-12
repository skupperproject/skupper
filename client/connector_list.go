package client

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func (cli *VanClient) ConnectorList(ctx context.Context) ([]*types.Connector, error) {
	var connectors []*types.Connector
	current, err := cli.KubeClient.AppsV1().Deployments(cli.Namespace).Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err == nil {
		mode := qdr.GetTransportMode(current)
		secrets, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).List(metav1.ListOptions{LabelSelector: "skupper.io/type=connection-token"})
		if err == nil {
			var role types.ConnectorRole
			var hostKey string
			var portKey string
			if mode == types.TransportModeEdge {
				role = types.ConnectorRoleEdge
				hostKey = "edge-host"
				portKey = "edge-port"
			} else {
				role = types.ConnectorRoleInterRouter
				hostKey = "inter-router-host"
				portKey = "inter-router-port"
			}
			for _, s := range secrets.Items {
				connectors = append(connectors, &types.Connector{
					Name: s.ObjectMeta.Name,
					Host: s.ObjectMeta.Annotations[hostKey],
					Port: s.ObjectMeta.Annotations[portKey],
					Role: string(role),
				})
			}
			return connectors, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}
