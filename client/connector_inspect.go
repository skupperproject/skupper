package client

import (
	"context"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube/qdr"
)

// ConnectorInspect VAN connector instance
func (cli *VanClient) ConnectorInspect(ctx context.Context, name string) (*types.LinkStatus, error) {
	current, err := cli.getRouterConfig()
	if err != nil {
		return nil, err
	}
	secret, _, err := cli.SecretManager(cli.Namespace).GetSecret(name)
	if err != nil {
		return nil, err
	}
	connections, _ := qdr.GetConnections(cli.Namespace, cli.KubeClient, cli.RestConfig)
	link := getLinkStatus(secret, current.IsEdge(), connections)
	return &link, nil
}
