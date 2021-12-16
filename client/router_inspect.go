package client

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/data"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func (cli *VanClient) getConsoleUrl() (string, error) {
	if cli.RouteClient == nil {
		service, err := cli.KubeClient.CoreV1().Services(cli.Namespace).Get(types.ControllerServiceName, metav1.GetOptions{})
		if err != nil {
			return "", err
		} else {
			if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
				host := kube.GetLoadBalancerHostOrIp(service)
				return "https://" + host + ":8080", nil
			} else if service.Spec.Type == corev1.ServiceTypeNodePort {
				port := ""
				for _, p := range service.Spec.Ports {
					if p.Name == "metrics" {
						port = strconv.Itoa(int(p.NodePort))
					}
				}
				config, err := cli.SiteConfigInspect(context.Background(), nil)
				if err != nil {
					return "", err
				}
				host := config.Spec.GetControllerIngressHost()
				if host == "" || port == "" {
					return "", nil
				}
				return "https://" + host + ":" + port, nil
			} else {
				proxy, err := kube.GetContourProxy(cli.DynamicClient, cli.Namespace, "skupper-console")
				if err != nil {
					return "", err
				}
				if proxy != nil {
					return "https://" + proxy.Host, nil
				}
				routes, err := kube.GetIngressRoutes(types.IngressName, cli.Namespace, cli.KubeClient)
				if err != nil {
					return "", err
				}
				for _, route := range routes {
					if strings.HasPrefix(route.Host, "console") {
						return "https://" + route.Host, nil
					}
				}
				return "", nil
			}
		}
	} else {
		route, err := cli.RouteClient.Routes(cli.Namespace).Get(types.ConsoleRouteName, metav1.GetOptions{})
		if err != nil {
			return "", err
		} else {
			return "https://" + route.Spec.Host, nil
		}
	}
}

func (cli *VanClient) RouterInspect(ctx context.Context) (*types.RouterInspectResponse, error) {
	return cli.RouterInspectNamespace(ctx, cli.Namespace)
}

// RouterInspect VAN deployment
func (cli *VanClient) RouterInspectNamespace(ctx context.Context, namespace string) (*types.RouterInspectResponse, error) {
	vir := &types.RouterInspectResponse{}

	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, namespace, cli.KubeClient)
	if err != nil {
		return nil, err
	}
	routerConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return nil, err
	}
	current, err := cli.KubeClient.AppsV1().Deployments(namespace).Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err == nil {
		siteConfig, err := cli.SiteConfigInspectInNamespace(ctx, nil, namespace)
		if err == nil && siteConfig != nil {
			vir.Status.SiteName = siteConfig.Spec.SkupperName
			connected, err := cli.getSitesInNetwork(siteConfig.Reference.UID, namespace)
			for i := 0; i < 5 && err != nil; i++ {
				time.Sleep(500 * time.Millisecond)
				connected, err = cli.getSitesInNetwork(siteConfig.Reference.UID, namespace)
			}

			if err == nil {
				vir.Status.ConnectedSites = connected
			}
		}
		vir.Status.Mode = string(routerConfig.Metadata.Mode)
		vir.Status.TransportReadyReplicas = current.Status.ReadyReplicas

		vir.TransportVersion = kube.GetComponentVersion(namespace, cli.KubeClient, types.TransportComponentName, types.TransportContainerName)
		vir.ControllerVersion = kube.GetComponentVersion(namespace, cli.KubeClient, types.ControllerComponentName, types.ControllerContainerName)
		vsis, err := cli.ServiceInterfaceList(context.Background())
		if err != nil {
			vir.ExposedServices = 0
		} else {
			vir.ExposedServices = len(vsis)
		}
		url, err := cli.getConsoleUrl()
		if url != "" {
			vir.ConsoleUrl = url
		}
	}

	return vir, err

}

func (cli *VanClient) getSitesInNetwork(siteId string, namespace string) (types.TransportConnectedSites, error) {
	result := types.TransportConnectedSites{}
	output, err := cli.exec([]string{"get", "sites", "-o", "json"}, namespace)
	if err != nil {
		return result, err
	}
	sites := []data.Site{}
	err = json.Unmarshal(output.Bytes(), &sites)
	if err != nil {
		return result, err
	}
	self := getSelf(sites, siteId)
	for _, site := range sites {
		if site.SiteId == siteId { //skip self
			continue
		}
		if site.IsConnectedTo(siteId) || (self != nil && self.IsConnectedTo(site.SiteId)) {
			result.Direct++
		} else {
			result.Indirect++
		}
		result.Total++
	}
	return result, nil
}

func getSelf(sites []data.Site, siteId string) *data.Site {
	for _, site := range sites {
		if site.SiteId == siteId {
			return &site
		}
	}
	return nil
}

func (cli *VanClient) exec(command []string, namespace string) (*bytes.Buffer, error) {
	pod, err := kube.GetReadyPod(namespace, cli.KubeClient, "service-controller")
	if err != nil {
		return nil, err
	}
	return kube.ExecCommandInContainer(command, pod.Name, "service-controller", namespace, cli.KubeClient, cli.RestConfig)
}
