package client

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

func (cli *VanClient) VanSiteConfigCreate(ctx context.Context, spec types.VanSiteConfigSpec) (*types.VanSiteConfig, error) {
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
			"namespace":              "",
			"edge":                   "false",
			"service-controller":     "true",
			"service-sync":           "true",
			"console":                "true",
			"router-console":         "false",
			"console-authentication": "internal",
			"console-user":           "",
			"console-password":       "",
			"cluster-local":          "false",
		},
	}
	if spec.SkupperName != "" {
		siteConfig.Data["name"] = spec.SkupperName
	}
	if spec.SkupperNamespace != "" {
		siteConfig.Data["namespace"] = spec.SkupperNamespace
	}
	if spec.IsEdge {
		siteConfig.Data["edge"] = "true"
	}
	if !spec.EnableController {
		siteConfig.Data["service-controller"] = "false"
	}
	if !spec.EnableServiceSync {
		siteConfig.Data["service-sync"] = "false"
	}
	if spec.EnableConsole {
		siteConfig.Data["console"] = "true"
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
	if spec.ClusterLocal {
		siteConfig.Data["cluster-local"] = "true"
	}
	// TODO: allow Replicas to be set through skupper-site configmap?
	if !spec.SiteControlled {
		siteConfig.ObjectMeta.Labels = map[string]string{
			"internal.skupper.io/site-controller-ignore": "true",
		}
	}
	actual, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Create(siteConfig)
	if err != nil {
		return nil, err
	}
	if actual.TypeMeta.Kind == "" || actual.TypeMeta.APIVersion == "" { //why??
		actual.TypeMeta = siteConfig.TypeMeta
	}
	return cli.VanSiteConfigInspect(ctx, actual)
}
