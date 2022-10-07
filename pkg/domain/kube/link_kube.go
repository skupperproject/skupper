package kube

import (
	"context"
	"fmt"
	"strconv"

	"github.com/skupperproject/skupper/api/types"
	kubeqdr "github.com/skupperproject/skupper/pkg/kube/qdr"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/server"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type LinkHandlerKube struct {
	namespace    string
	site         *types.SiteConfig
	routerConfig *qdr.RouterConfig
	cli          kubernetes.Interface
	restConfig   *rest.Config
}

func NewLinkHandlerKube(namespace string, site *types.SiteConfig, routerConfig *qdr.RouterConfig, cli kubernetes.Interface, restConfig *rest.Config) *LinkHandlerKube {
	return &LinkHandlerKube{
		namespace:    namespace,
		site:         site,
		cli:          cli,
		restConfig:   restConfig,
		routerConfig: routerConfig,
	}
}

func (l *LinkHandlerKube) Create(secret *corev1.Secret, name string, cost int) error {
	// TODO implement me
	panic("implement me")
}

func (l *LinkHandlerKube) Delete(name string) error {
	// TODO implement me
	panic("implement me")
}

func (l *LinkHandlerKube) List() ([]*corev1.Secret, error) {
	currentSecrets, err := l.cli.CoreV1().Secrets(l.namespace).List(metav1.ListOptions{LabelSelector: "skupper.io/type=connection-token"})
	if err != nil {
		return nil, fmt.Errorf("Could not retrieve secrets: %w", err)
	}
	var secrets []*corev1.Secret
	for _, s := range currentSecrets.Items {
		secrets = append(secrets, &s)
	}
	return secrets, nil
}

func (l *LinkHandlerKube) StatusAll() ([]types.LinkStatus, error) {
	var ls []types.LinkStatus
	secrets, err := l.cli.CoreV1().Secrets(l.namespace).List(metav1.ListOptions{LabelSelector: "skupper.io/type in (connection-token, token-claim)"})
	if err != nil {
		return ls, err
	}
	connections, _ := kubeqdr.GetConnections(l.namespace, l.cli, l.restConfig)
	for _, secret := range secrets.Items {
		ls = append(ls, qdr.GetLinkStatus(&secret, l.routerConfig.IsEdge(), connections))
	}
	return ls, nil
}

func (l *LinkHandlerKube) Status(name string) (types.LinkStatus, error) {
	var ls types.LinkStatus
	secret, err := l.cli.CoreV1().Secrets(l.namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return ls, err
	}
	connections, _ := kubeqdr.GetConnections(l.namespace, l.cli, l.restConfig)
	link := qdr.GetLinkStatus(secret, l.routerConfig.IsEdge(), connections)
	return link, nil
}

func (l *LinkHandlerKube) Detail(link types.LinkStatus) (map[string]string, error) {
	status := "Active"

	if !link.Connected {
		status = "Not active"

		if len(link.Description) > 0 {
			status = fmt.Sprintf("%s (%s)", status, link.Description)
		}
	}

	return map[string]string{
		"Name:":      link.Name,
		"Status:":    status,
		"Namespace:": l.site.Spec.SkupperNamespace,
		"Site:":      l.site.Spec.SkupperName + "-" + l.site.Reference.UID,
		"Cost:":      strconv.Itoa(link.Cost),
		"Created:":   link.Created,
	}, nil
}

func (l *LinkHandlerKube) RemoteLinks(ctx context.Context) ([]*types.RemoteLinkInfo, error) {
	// Checking if the router has been deployed
	_, err := l.cli.AppsV1().Deployments(l.namespace).Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("skupper is not installed: %s", err)
	}

	currentSiteId := l.site.Reference.UID

	sites, err := server.GetSiteInfo(ctx, l.namespace, l.cli, l.restConfig)

	if err != nil {
		return nil, err
	}

	var remoteLinks []*types.RemoteLinkInfo

	for _, site := range *sites {

		if site.SiteId == currentSiteId {
			continue
		}

		for _, link := range site.Links {
			if link == currentSiteId {
				newRemoteLink := types.RemoteLinkInfo{SiteName: site.Name, Namespace: site.Namespace, SiteId: site.SiteId}
				remoteLinks = append(remoteLinks, &newRemoteLink)
			}
		}
	}
	return remoteLinks, nil
}
