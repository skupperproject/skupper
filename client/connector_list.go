package client

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func getLinkStatus(s *corev1.Secret, edge bool, connections []qdr.Connection) types.LinkStatus {
	link := types.LinkStatus{
		Name: s.ObjectMeta.Name,
	}
	if s.ObjectMeta.Labels[types.SkupperTypeQualifier] == types.TypeClaimRequest {
		link.Url = s.ObjectMeta.Annotations[types.ClaimUrlAnnotationKey]
		link.Description = "Failed to redeem claim: " + s.ObjectMeta.Annotations[types.StatusAnnotationKey]
		link.Configured = false
	} else {
		if edge {
			link.Url = fmt.Sprintf("%s:%s", s.ObjectMeta.Annotations["edge-host"], s.ObjectMeta.Annotations["edge-port"])
		} else {
			link.Url = fmt.Sprintf("%s:%s", s.ObjectMeta.Annotations["inter-router-host"], s.ObjectMeta.Annotations["inter-router-port"])
		}
		link.Configured = true
		if connection := qdr.GetInterRouterOrEdgeConnection(link.Url, connections); connection != nil && connection.Active {
			link.Connected = true
		}
	}
	return link
}

func (cli *VanClient) getRouterConfig() (*qdr.RouterConfig, error) {
	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, cli.Namespace, cli.KubeClient)
	if errors.IsNotFound(err) {
		return nil, fmt.Errorf("Skupper is not installed in %s", cli.Namespace)
	} else if err != nil {
		return nil, err
	}
	current, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return nil, err
	}
	return current, nil
}

func (cli *VanClient) ConnectorList(ctx context.Context) ([]types.LinkStatus, error) {
	var links []types.LinkStatus
	secrets, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).List(metav1.ListOptions{LabelSelector: "skupper.io/type in (connection-token, token-claim)"})
	if err != nil {
		return links, err
	}
	current, err := cli.getRouterConfig()
	if err != nil {
		return links, err
	}
	edge := current.IsEdge()
	connections, _ := qdr.GetConnections(cli.Namespace, cli.KubeClient, cli.RestConfig)
	for _, s := range secrets.Items {
		links = append(links, getLinkStatus(&s, edge, connections))
	}
	return links, nil
}
