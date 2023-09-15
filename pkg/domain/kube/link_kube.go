package kube

import (
	"context"
	"fmt"
	k8s "github.com/skupperproject/skupper/pkg/kube"
	"strconv"
	"strings"

	"encoding/json"
	"github.com/skupperproject/skupper/api/types"
	kubeqdr "github.com/skupperproject/skupper/pkg/kube/qdr"
	"github.com/skupperproject/skupper/pkg/qdr"
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
	currentSecrets, err := l.cli.CoreV1().Secrets(l.namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "skupper.io/type in (connection-token, token-claim)"})
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

func (l *LinkHandlerKube) RemoteLinks(ctx context.Context) ([]*types.RemoteLinkInfo, error) {
	// Checking if the router has been deployed
	_, err := k8s.GetDeployment(types.TransportDeploymentName, l.namespace, l.cli)
	if err != nil {
		return nil, fmt.Errorf("skupper is not installed: %s", err)
	}

	currentSiteId := l.site.Reference.UID

	configmap, err := k8s.GetConfigMap(types.SiteStatusConfigMapName, l.namespace, l.cli)
	if err != nil {
		return nil, err
	}

	sites, err := UnmarshalSiteStatus(&configmap.Data)
	if err != nil {
		return nil, err
	}

	var remoteLinks []*types.RemoteLinkInfo

	currentSite := sites[currentSiteId]
	mapRouterSite := getRouterSiteMap(sites)

	if currentSite != nil {
		for _, router := range currentSite.RouterStatus {
			for _, link := range router.Links {
				if link.Direction == "incoming" {

					remoteSite := mapRouterSite[link.Name]

					if remoteSite == nil {
						return nil, fmt.Errorf("there was an issue getting information from the network: config map %s is incomplete", types.SiteStatusConfigMapName)
					}

					newRemoteLink := types.RemoteLinkInfo{SiteName: remoteSite.Site.Name, Namespace: remoteSite.Site.Namespace, SiteId: remoteSite.Site.Identity, LinkName: link.Name}
					remoteLinks = append(remoteLinks, &newRemoteLink)
				}
			}
		}
	}

	return remoteLinks, nil
}

func UnmarshalSiteStatus(data *map[string]string) (map[string]*types.SiteStatusInfo, error) {

	allSites := make(map[string]*types.SiteStatusInfo)

	for _, site := range *data {
		var siteStatus types.SiteStatusInfo
		err := json.Unmarshal([]byte(site), &siteStatus)

		if err != nil {
			return nil, err
		}

		allSites[siteStatus.Site.Identity] = &siteStatus

	}

	return allSites, nil
}

func getRouterSiteMap(sitesStatus map[string]*types.SiteStatusInfo) map[string]*types.SiteStatusInfo {
	mapRouterSite := make(map[string]*types.SiteStatusInfo)
	for _, siteStatus := range sitesStatus {
		if len(siteStatus.RouterStatus) > 0 {
			for _, routerStatus := range siteStatus.RouterStatus {

				// the name of the router has a "0/" as a prefix that it is needed to remove
				routerName := strings.Split(routerStatus.Router.Name, "/")
				mapRouterSite[routerName[1]] = siteStatus
			}
		}
	}

	return mapRouterSite
}
