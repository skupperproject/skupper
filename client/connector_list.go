package client

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func (cli *VanClient) ConnectorList(ctx context.Context) ([]*types.Connector, error) {
	var connectors []*types.Connector
	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, cli.Namespace, cli.KubeClient)
	if err != nil {
		return connectors, err
	}
	current, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return connectors, err
	}
	secrets, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).List(metav1.ListOptions{LabelSelector: "skupper.io/type=connection-token"})
	if err != nil {
		return connectors, err
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
	for _, s := range secrets.Items {
		connectors = append(connectors, &types.Connector{
			Name: s.ObjectMeta.Name,
			Host: s.ObjectMeta.Annotations[hostKey],
			Port: s.ObjectMeta.Annotations[portKey],
			Role: string(role),
		})
	}
	return connectors, nil
}
