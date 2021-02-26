package client

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

func (cli *VanClient) SiteConfigInspect(ctx context.Context, input *corev1.ConfigMap) (*types.SiteConfig, error) {
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

	var result types.SiteConfig
	result.Spec.SkupperNamespace = siteConfig.Namespace
	// TODO: what should the defaults be for name, namespace
	if skupperName, ok := siteConfig.Data["name"]; ok {
		result.Spec.SkupperName = skupperName
	} else {
		result.Spec.SkupperName = cli.Namespace
	}
	if routerMode, ok := siteConfig.Data["router-mode"]; ok {
		result.Spec.RouterMode = routerMode
	} else {
		// check for deprecated key
		if isEdge, ok := siteConfig.Data["edge"]; ok {
			if isEdge == "true" {
				result.Spec.RouterMode = string(types.TransportModeEdge)
			} else {
				result.Spec.RouterMode = string(types.TransportModeInterior)
			}
		} else {
			result.Spec.RouterMode = string(types.TransportModeInterior)
		}
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
	if ingress, ok := siteConfig.Data["ingress"]; ok {
		result.Spec.Ingress = ingress
	} else {
		// check for deprecated key
		if clusterLocal, ok := siteConfig.Data["cluster-local"]; ok {
			if clusterLocal == "true" {
				result.Spec.Ingress = types.IngressNoneString
			} else {
				result.Spec.Ingress = types.IngressLoadBalancerString
			}
		} else {
			result.Spec.Ingress = types.IngressLoadBalancerString
		}
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
	if routerDebugMode, ok := siteConfig.Data["router-debug-mode"]; ok && routerDebugMode != "" {
		result.Spec.RouterDebugMode = routerDebugMode
	}
	if routerLogging, ok := siteConfig.Data["router-logging"]; ok && routerLogging != "" {
		logConf, err := ParseRouterLogConfig(routerLogging)
		if err != nil {
			return &result, err
		}
		result.Spec.RouterLogging = logConf
	}
	return &result, nil
}
