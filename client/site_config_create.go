package client

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

func (cli *VanClient) SiteConfigCreate(ctx context.Context, spec types.SiteConfigSpec) (*types.SiteConfig, error) {
	siteConfig := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "skupper-site",
		},
		Data: map[string]string{
			"name":                   cli.Namespace,
			"router-mode":            string(types.TransportModeInterior),
			"service-controller":     "true",
			"service-sync":           "true",
			"console":                "true",
			"router-console":         "false",
			"router-logging":         "",
			"console-authentication": "internal",
			"console-user":           "",
			"console-password":       "",
			"ingress":                types.IngressLoadBalancerString,
		},
	}
	if spec.SkupperName != "" {
		siteConfig.Data["name"] = spec.SkupperName
	}
	if spec.RouterMode != "" {
		siteConfig.Data["router-mode"] = spec.RouterMode
	}
	if !spec.EnableController {
		siteConfig.Data["service-controller"] = "false"
	}
	if !spec.EnableServiceSync {
		siteConfig.Data["service-sync"] = "false"
	}
	if !spec.EnableConsole {
		siteConfig.Data["console"] = "false"
	}
	if spec.EnableRouterConsole {
		siteConfig.Data["router-console"] = "true"
	}
	if spec.AuthMode != "" {
		siteConfig.Data["console-authentication"] = spec.AuthMode
	}
	if spec.User != "" {
		siteConfig.Data["console-user"] = spec.User
	}
	if spec.Password != "" {
		siteConfig.Data["console-password"] = spec.Password
	}
	if spec.Ingress != "" {
		siteConfig.Data["ingress"] = spec.Ingress
	}
	if spec.RouterLogging != nil {
		siteConfig.Data["router-logging"] = RouterLogConfigToString(spec.RouterLogging)
	}
	if spec.RouterDebugMode != "" {
		siteConfig.Data["router-debug-mode"] = spec.RouterDebugMode
	}
	// TODO: allow Replicas to be set through skupper-site configmap?
	if !spec.SiteControlled {
		siteConfig.ObjectMeta.Labels = map[string]string{
			"internal.skupper.io/site-controller-ignore": "true",
		}
	}

	if spec.IsIngressRoute() && cli.RouteClient == nil {
		return nil, fmt.Errorf("OpenShift cluster not detected for --ingress type route")
	}

	actual, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Create(siteConfig)
	if err != nil {
		return nil, err
	}
	if actual.TypeMeta.Kind == "" || actual.TypeMeta.APIVersion == "" { //why??
		actual.TypeMeta = siteConfig.TypeMeta
	}
	return cli.SiteConfigInspect(ctx, actual)
}
