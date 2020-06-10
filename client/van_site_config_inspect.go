package client

import (
	"context"
	"strconv"

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
	if skupperName, ok := siteConfig.Data["name"]; ok {
		result.Spec.SkupperName = skupperName
	} else {
		result.Spec.SkupperName = cli.Namespace
	}
	if isEdge, ok := siteConfig.Data["edge"]; ok {
		result.Spec.IsEdge, _ = strconv.ParseBool(isEdge)
	} else {
		result.Spec.IsEdge = false
	}
	if enableController, ok := siteConfig.Data["service-controller"]; ok {
		result.Spec.EnableController, _ = strconv.ParseBool(enableController)
	} else {
		result.Spec.EnableController = true
	}
	if enableServiceSync, ok := siteConfig.Data["service-sync"]; ok {
		result.Spec.EnableServiceSync, _ = strconv.ParseBool(enableServiceSync)
	} else {
		result.Spec.EnableServiceSync = true
	}
	if enableConsole, ok := siteConfig.Data["console"]; ok {
		result.Spec.EnableConsole, _ = strconv.ParseBool(enableConsole)
	} else {
		result.Spec.EnableConsole = true
	}
	if enableRouterConsole, ok := siteConfig.Data["router-console"]; ok {
		result.Spec.EnableRouterConsole, _ = strconv.ParseBool(enableRouterConsole)
	} else {
		result.Spec.EnableRouterConsole = false
	}
	if authMode, ok := siteConfig.Data["console-authentication"]; ok {
		result.Spec.AuthMode = authMode
	} else {
		result.Spec.AuthMode = "internal"
	}
	if user, ok := siteConfig.Data["console-user"]; ok {
		result.Spec.User = user
	} else {
		result.Spec.User = ""
	}
	if password, ok := siteConfig.Data["console-password"]; ok {
		result.Spec.Password = password
	} else {
		result.Spec.Password = ""
	}
	if clusterLocal, ok := siteConfig.Data["cluster-local"]; ok {
		result.Spec.ClusterLocal, _ = strconv.ParseBool(clusterLocal)
	} else {
		result.Spec.ClusterLocal = false
	}
	// TODO: allow Replicas to be set through skupper-site configmap?
	if siteConfig.ObjectMeta.Labels == nil {
		result.Spec.SiteControlled = true
	} else if ignore, ok := siteConfig.ObjectMeta.Labels["internal.skupper.io/site-controller-ignore"]; ok {
		siteIgnore, _ := strconv.ParseBool(ignore)
		result.Spec.SiteControlled = !siteIgnore
	} else {
		result.Spec.SiteControlled = true
	}
	result.Reference.UID = string(siteConfig.ObjectMeta.UID)
	result.Reference.Name = siteConfig.ObjectMeta.Name
	result.Reference.Kind = siteConfig.TypeMeta.Kind
	result.Reference.APIVersion = siteConfig.TypeMeta.APIVersion
	return &result, nil
}
