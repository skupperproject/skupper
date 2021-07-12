package client

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

const (
	//core options
	SiteConfigNameKey       string = "name"
	SiteConfigRouterModeKey string = "router-mode"
	SiteConfigIngressKey    string = "ingress"

	//console options
	SiteConfigConsoleKey               string = "console"
	SiteConfigConsoleAuthenticationKey string = "console-authentication"
	SiteConfigConsoleUserKey           string = "console-user"
	SiteConfigConsolePasswordKey       string = "console-password"
	SiteConfigConsoleIngressKey        string = "console-ingress"

	//router options
	SiteConfigRouterConsoleKey          string = "router-console"
	SiteConfigRouterLoggingKey          string = "router-logging"
	SiteConfigRouterDebugModeKey        string = "router-debug-mode"
	SiteConfigRouterCpuKey              string = "router-cpu"
	SiteConfigRouterMemoryKey           string = "router-memory"
	SiteConfigRouterAffinityKey         string = "router-pod-affinity"
	SiteConfigRouterAntiAffinityKey     string = "router-pod-antiaffinity"
	SiteConfigRouterNodeSelectorKey     string = "router-node-selector"
	SiteConfigRouterMaxFrameSizeKey     string = "xp-router-max-frame-size"
	SiteConfigRouterMaxSessionFramesKey string = "xp-router-max-session-frames"
	SiteConfigRouterIngressHostKey      string = "router-ingress-host"

	//controller options
	SiteConfigServiceControllerKey      string = "service-controller"
	SiteConfigServiceSyncKey            string = "service-sync"
	SiteConfigControllerCpuKey          string = "controller-cpu"
	SiteConfigControllerMemoryKey       string = "controller-memory"
	SiteConfigControllerAffinityKey     string = "controller-pod-affinity"
	SiteConfigControllerAntiAffinityKey string = "controller-pod-antiaffinity"
	SiteConfigControllerNodeSelectorKey string = "controller-node-selector"
	SiteConfigControllerIngressHostKey  string = "controller-ingress-host"
)

func (cli *VanClient) SiteConfigCreate(ctx context.Context, spec types.SiteConfigSpec) (*types.SiteConfig, error) {
	siteConfig := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        types.SiteConfigMapName,
			Annotations: spec.Annotations,
			Labels:      spec.Labels,
		},
		Data: map[string]string{
			SiteConfigNameKey:                  cli.Namespace,
			SiteConfigRouterModeKey:            string(types.TransportModeInterior),
			SiteConfigServiceControllerKey:     "true",
			SiteConfigServiceSyncKey:           "true",
			SiteConfigConsoleKey:               "true",
			SiteConfigRouterConsoleKey:         "false",
			SiteConfigRouterLoggingKey:         "",
			SiteConfigConsoleAuthenticationKey: types.ConsoleAuthModeInternal,
			SiteConfigConsoleUserKey:           "",
			SiteConfigConsolePasswordKey:       "",
			SiteConfigIngressKey:               types.IngressLoadBalancerString,
		},
	}
	if spec.SkupperName != "" {
		siteConfig.Data[SiteConfigNameKey] = spec.SkupperName
	}
	if spec.RouterMode != "" {
		siteConfig.Data[SiteConfigRouterModeKey] = spec.RouterMode
	}
	if !spec.EnableController {
		siteConfig.Data[SiteConfigServiceControllerKey] = "false"
	}
	if !spec.EnableServiceSync {
		siteConfig.Data[SiteConfigServiceSyncKey] = "false"
	}
	if !spec.EnableConsole {
		siteConfig.Data[SiteConfigConsoleKey] = "false"
	}
	if spec.EnableRouterConsole {
		siteConfig.Data[SiteConfigRouterConsoleKey] = "true"
	}
	if spec.AuthMode != "" {
		siteConfig.Data[SiteConfigConsoleAuthenticationKey] = spec.AuthMode
	}
	if spec.User != "" {
		siteConfig.Data[SiteConfigConsoleUserKey] = spec.User
	}
	if spec.Password != "" {
		siteConfig.Data[SiteConfigConsolePasswordKey] = spec.Password
	}
	if spec.Ingress != "" {
		siteConfig.Data[SiteConfigIngressKey] = spec.Ingress
	}
	if spec.ConsoleIngress != "" {
		siteConfig.Data[SiteConfigConsoleIngressKey] = spec.ConsoleIngress
	}
	if spec.Router.Logging != nil {
		siteConfig.Data[SiteConfigRouterLoggingKey] = RouterLogConfigToString(spec.Router.Logging)
	}
	if spec.Router.DebugMode != "" {
		siteConfig.Data[SiteConfigRouterDebugModeKey] = spec.Router.DebugMode
	}
	if spec.Router.Cpu != "" {
		if _, err := resource.ParseQuantity(spec.Router.Cpu); err != nil {
			return nil, fmt.Errorf("Invalid value for %s %q: %s", SiteConfigRouterCpuKey, spec.Router.Cpu, err)
		}
		siteConfig.Data[SiteConfigRouterCpuKey] = spec.Router.Cpu
	}
	if spec.Router.Memory != "" {
		if _, err := resource.ParseQuantity(spec.Router.Memory); err != nil {
			return nil, fmt.Errorf("Invalid value for %s %q: %s", SiteConfigRouterMemoryKey, spec.Router.Memory, err)
		}
		siteConfig.Data[SiteConfigRouterMemoryKey] = spec.Router.Memory
	}
	if spec.Router.Affinity != "" {
		siteConfig.Data[SiteConfigRouterAffinityKey] = spec.Router.Affinity
	}
	if spec.Router.AntiAffinity != "" {
		siteConfig.Data[SiteConfigRouterAntiAffinityKey] = spec.Router.AntiAffinity
	}
	if spec.Router.NodeSelector != "" {
		siteConfig.Data[SiteConfigRouterNodeSelectorKey] = spec.Router.NodeSelector
	}
	if spec.Router.IngressHost != "" {
		siteConfig.Data[SiteConfigRouterIngressHostKey] = spec.Router.IngressHost
	}
	if spec.Router.MaxFrameSize != types.RouterMaxFrameSizeDefault {
		siteConfig.Data[SiteConfigRouterMaxFrameSizeKey] = strconv.Itoa(spec.Router.MaxFrameSize)
	}
	if spec.Router.MaxSessionFrames != types.RouterMaxSessionFramesDefault {
		siteConfig.Data[SiteConfigRouterMaxSessionFramesKey] = strconv.Itoa(spec.Router.MaxSessionFrames)
	}
	if spec.Controller.Cpu != "" {
		if _, err := resource.ParseQuantity(spec.Controller.Cpu); err != nil {
			return nil, fmt.Errorf("Invalid value for %s %q: %s", SiteConfigControllerCpuKey, spec.Controller.Cpu, err)
		}
		siteConfig.Data[SiteConfigControllerCpuKey] = spec.Controller.Cpu
	}
	if spec.Controller.Memory != "" {
		if _, err := resource.ParseQuantity(spec.Controller.Memory); err != nil {
			return nil, fmt.Errorf("Invalid value for %s %q: %s", SiteConfigControllerMemoryKey, spec.Controller.Memory, err)
		}
		siteConfig.Data[SiteConfigControllerMemoryKey] = spec.Controller.Memory
	}
	if spec.Controller.Affinity != "" {
		siteConfig.Data[SiteConfigControllerAffinityKey] = spec.Controller.Affinity
	}
	if spec.Controller.AntiAffinity != "" {
		siteConfig.Data[SiteConfigControllerAntiAffinityKey] = spec.Controller.AntiAffinity
	}
	if spec.Controller.NodeSelector != "" {
		siteConfig.Data[SiteConfigControllerNodeSelectorKey] = spec.Controller.NodeSelector
	}
	if spec.Controller.IngressHost != "" {
		siteConfig.Data[SiteConfigControllerIngressHostKey] = spec.Controller.IngressHost
	}
	// TODO: allow Replicas to be set through skupper-site configmap?
	if !spec.SiteControlled {
		if siteConfig.ObjectMeta.Labels == nil {
			siteConfig.ObjectMeta.Labels = map[string]string{}
		}
		siteConfig.ObjectMeta.Labels[types.SiteControllerIgnore] = "true"
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
