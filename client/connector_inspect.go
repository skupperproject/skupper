package client

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

// ConnectorInspect VAN connector instance
func (cli *VanClient) ConnectorInspect(ctx context.Context, name string) (*types.ConnectorInspectResponse, error) {
	vci := &types.ConnectorInspectResponse{}

	configmap, err := kube.GetConfigMap("skupper-internal", cli.Namespace, cli.KubeClient)
	if err != nil {
		return nil, err
	}
	current, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return nil, err
	}
	secret, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	var role types.ConnectorRole
	var hostKey string
	var portKey string
	if current.IsEdge() {
		role = types.ConnectorRoleEdge
		hostKey = "edge-host"
		portKey = "edge-port"
	} else {
		role = types.ConnectorRoleInterRouter
		hostKey = "inter-router-host"
		portKey = "inter-router-port"
	}
	vci.Connector = &types.Connector{
		Name: secret.ObjectMeta.Name,
		Host: secret.ObjectMeta.Annotations[hostKey],
		Port: secret.ObjectMeta.Annotations[portKey],
		Role: string(role),
	}

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
}
