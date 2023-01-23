package client

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube/qdr"
)

// ConnectorInspect VAN connector instance
func (cli *VanClient) ConnectorInspect(ctx context.Context, name string) (*types.LinkStatus, error) {
	current, err := cli.getRouterConfig(ctx, "")
	if err != nil {
		return nil, err
	}
	secret, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	connections, _ := qdr.GetConnections(cli.Namespace, cli.KubeClient, cli.RestConfig)
	link := getLinkStatus(secret, current.IsEdge(), connections)
	return &link, nil
}
