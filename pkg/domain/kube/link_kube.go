package kube

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	k8s "github.com/skupperproject/skupper/pkg/kube"
	kubeqdr "github.com/skupperproject/skupper/pkg/kube/qdr"
	"github.com/skupperproject/skupper/pkg/network"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"strconv"
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
	currentSecrets, err := l.cli.CoreV1().Secrets(l.namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "skupper.io/type in (connection-token, token-claim)"})
	if err != nil {
		return nil, fmt.Errorf("Could not retrieve secrets: %w", err)
	}
	var secrets []*corev1.Secret

	for i, _ := range currentSecrets.Items {
		secrets = append(secrets, &currentSecrets.Items[i])
	}
	return secrets, nil
}

func (l *LinkHandlerKube) StatusAll() ([]types.LinkStatus, error) {
	var ls []types.LinkStatus
	secrets, err := l.cli.CoreV1().Secrets(l.namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "skupper.io/type in (connection-token, token-claim)"})
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
	secret, err := l.cli.CoreV1().Secrets(l.namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return ls, err
	}
	connections, _ := kubeqdr.GetConnections(l.namespace, l.cli, l.restConfig)
	link := qdr.GetLinkStatus(secret, l.routerConfig.IsEdge(), connections)
	return link, nil
}

func (l *LinkHandlerKube) Detail(link types.LinkStatus) (map[string]string, error) {
	status := "Connected"

	if !link.Connected {
		status = "Not connected"

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

func (l *LinkHandlerKube) RemoteLinks(ctx context.Context) ([]*network.RemoteLinkInfo, error) {
	// Checking if the router has been deployed
	_, err := k8s.GetDeployment(types.TransportDeploymentName, l.namespace, l.cli)
	if err != nil {
		return nil, fmt.Errorf("skupper is not installed: %s", err)
	}

	configSyncVersion := utils.GetVersionTag(k8s.GetComponentVersion(l.namespace, l.cli, types.TransportContainerName, types.ConfigSyncContainerName))
	if configSyncVersion != "" && !utils.IsValidFor(configSyncVersion, network.MINIMUM_VERSION) {
		return nil, fmt.Errorf(network.MINIMUM_VERSION_MESSAGE, configSyncVersion, network.MINIMUM_VERSION)
	}

	currentSiteId := l.site.Reference.UID

	configmap, err := k8s.GetConfigMap(types.NetworkStatusConfigMapName, l.namespace, l.cli)
	if err != nil {
		return nil, err
	}

	currentStatus, err := network.UnmarshalSkupperStatus(configmap.Data)
	if err != nil {
		return nil, err
	}

	var remoteLinks []*network.RemoteLinkInfo

	statusManager := network.SkupperStatus{NetworkStatus: currentStatus}

	mapRouterSite := statusManager.GetRouterSiteMap()

	var currentSite network.SiteStatusInfo
	for _, s := range currentStatus.SiteStatus {
		if s.Site.Identity == currentSiteId {
			currentSite = s
		}
	}

	if len(currentSite.Site.Identity) > 0 {
		for _, router := range currentSite.RouterStatus {
			for _, link := range router.Links {
				if link.Direction == "incoming" {
					remoteSite, ok := mapRouterSite[link.Name]
					if !ok {
						return nil, fmt.Errorf("remote site not found in config map %s", types.NetworkStatusConfigMapName)
					}

					// links between routers of the same site will not be shown
					if remoteSite.Site.Identity != currentSite.Site.Identity {
						newRemoteLink := network.RemoteLinkInfo{SiteName: remoteSite.Site.Name, Namespace: remoteSite.Site.Namespace, SiteId: remoteSite.Site.Identity, LinkName: link.Name}
						remoteLinks = append(remoteLinks, &newRemoteLink)
					}
				}
			}
		}
	}

	return remoteLinks, nil
}
