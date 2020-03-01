package client

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ajssmith/skupper/api/types"
	"github.com/ajssmith/skupper/pkg/qdr"
)

// VanConnectorInspect VAN connector instance
func (cli *VanClient) VanConnectorInspect(ctx context.Context, name string) (*types.VanConnectorInspectResponse, error) {
	vci := &types.VanConnectorInspectResponse{}

	current, err := cli.KubeClient.AppsV1().Deployments(cli.Namespace).Get(types.QdrDeploymentName, metav1.GetOptions{})
	if err == nil {
		mode := qdr.GetQdrMode(current)
		secret, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(name, metav1.GetOptions{})
		if err == nil {
			var role types.ConnectorRole
			var hostKey string
			var portKey string
			if mode == types.QdrModeEdge {
				role = types.ConnectorRoleEdge
				hostKey = "edge-host"
				portKey = "edge-port"
			} else {
				role = types.ConnectorRoleInterRouter
				hostKey = "inter-router-host"
				portKey = "inter-router-port"
			}
			vci.Connector.Name = secret.ObjectMeta.Name
			vci.Connector.Host = secret.ObjectMeta.Annotations[hostKey]
			vci.Connector.Port = secret.ObjectMeta.Annotations[portKey]
			vci.Connector.Role = string(role)

			connections, err := qdr.GetConnections(cli.Namespace, cli.KubeClient, cli.RestConfig)
			if err == nil {
				connection := qdr.GetInterRouterOrEdgeConnection(vci.Connector.Host+":"+vci.Connector.Port, connections)
				if connection == nil || !connection.Active {
					vci.Connected = false
				} else {
					vci.Connected = true
				}
			}
			return vci, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}
