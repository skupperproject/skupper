package client

import (
	"context"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
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
				return "http://" + host + ":8080", nil
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
				if config.Spec.Controller.IngressHost == "" || port == "" {
					return "", nil
				}
				return "http://" + config.Spec.Controller.IngressHost + ":" + port, nil
			} else {
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
		siteConfig, err := cli.SiteConfigInspect(ctx, nil)
		if err == nil && siteConfig != nil {
			vir.Status.SiteName = siteConfig.Spec.SkupperName
		}
		vir.Status.Mode = string(routerConfig.Metadata.Mode)
		vir.Status.TransportReadyReplicas = current.Status.ReadyReplicas
		connected, err := qdr.GetConnectedSites(vir.Status.Mode == string(types.TransportModeEdge), namespace, cli.KubeClient, cli.RestConfig)
		for i := 0; i < 5 && err != nil; i++ {
			time.Sleep(500 * time.Millisecond)
			connected, err = qdr.GetConnectedSites(vir.Status.Mode == string(types.TransportModeEdge), namespace, cli.KubeClient, cli.RestConfig)
		}

		if err == nil {
			vir.Status.ConnectedSites = connected
		}

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
