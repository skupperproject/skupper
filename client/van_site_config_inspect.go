package client

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

func (cli *VanClient) VanSiteConfigInspect(ctx context.Context, input *corev1.ConfigMap) (*types.VanSiteConfig, error) {
	var siteConfig *corev1.ConfigMap
	if input == nil {
		cm, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get("skupper-site", metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
		siteConfig = cm
	} else {
		siteConfig = input
	}
	var result types.VanSiteConfig
	result.Spec.SkupperName = siteConfig.Data["name"]
	result.Spec.IsEdge = siteConfig.Data["edge"] == "true"
	result.Spec.EnableController = siteConfig.Data["service-controller"] != "false"
	result.Spec.EnableServiceSync = siteConfig.Data["service-sync"] != "false"
	result.Spec.EnableConsole = siteConfig.Data["console"] != "false"
	result.Spec.AuthMode = siteConfig.Data["console-authentication"]
	result.Spec.User = siteConfig.Data["console-user"]
	result.Spec.Password = siteConfig.Data["console-password"]
	result.Spec.ClusterLocal = siteConfig.Data["cluster-local"] == "true"
	// TODO: allow Replicas to be set through skupper-site configmap?
	result.Spec.SiteControlled = siteConfig.ObjectMeta.Labels == nil || siteConfig.ObjectMeta.Labels["internal.skupper.io/site-controller-ignore"] != ""
	result.Reference.UID = string(siteConfig.ObjectMeta.UID)
	result.Reference.Name = siteConfig.ObjectMeta.Name
	result.Reference.Kind = siteConfig.TypeMeta.Kind
	result.Reference.APIVersion = siteConfig.TypeMeta.APIVersion
	return &result, nil
}
