package client

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

const (
	//core options
	SiteConfigNameKey                string = "name"
	SiteConfigRouterModeKey          string = "router-mode"
	SiteConfigIngressKey             string = "ingress"
	SiteConfigIngressAnnotationsKey  string = "ingress-annotations"
	SiteConfigIngressHostKey         string = "ingress-host"
	SiteConfigCreateNetworkPolicyKey string = "create-network-policy"
	SiteConfigRoutersKey             string = "routers"

	//console options
	SiteConfigConsoleKey               string = "console"
	SiteConfigConsoleAuthenticationKey string = "console-authentication"
	SiteConfigConsoleUserKey           string = "console-user"
	SiteConfigConsolePasswordKey       string = "console-password"
	SiteConfigConsoleIngressKey        string = "console-ingress"

	//router options
	SiteConfigRouterConsoleKey            string = "router-console"
	SiteConfigRouterLoggingKey            string = "router-logging"
	SiteConfigRouterDebugModeKey          string = "router-debug-mode"
	SiteConfigRouterCpuKey                string = "router-cpu"
	SiteConfigRouterMemoryKey             string = "router-memory"
	SiteConfigRouterCpuLimitKey           string = "router-cpu-limit"
	SiteConfigRouterMemoryLimitKey        string = "router-memory-limit"
	SiteConfigRouterAffinityKey           string = "router-pod-affinity"
	SiteConfigRouterAntiAffinityKey       string = "router-pod-antiaffinity"
	SiteConfigRouterNodeSelectorKey       string = "router-node-selector"
	SiteConfigRouterMaxFrameSizeKey       string = "xp-router-max-frame-size"
	SiteConfigRouterMaxSessionFramesKey   string = "xp-router-max-session-frames"
	SiteConfigRouterIngressHostKey        string = "router-ingress-host"
	SiteConfigRouterServiceAnnotationsKey string = "router-service-annotations"
	SiteConfigRouterLoadBalancerIp        string = "router-load-balancer-ip"

	//controller options
	SiteConfigServiceControllerKey            string = "service-controller"
	SiteConfigServiceSyncKey                  string = "service-sync"
	SiteConfigControllerCpuKey                string = "controller-cpu"
	SiteConfigControllerMemoryKey             string = "controller-memory"
	SiteConfigControllerCpuLimitKey           string = "controller-cpu-limit"
	SiteConfigControllerMemoryLimitKey        string = "controller-memory-limit"
	SiteConfigControllerAffinityKey           string = "controller-pod-affinity"
	SiteConfigControllerAntiAffinityKey       string = "controller-pod-antiaffinity"
	SiteConfigControllerNodeSelectorKey       string = "controller-node-selector"
	SiteConfigControllerIngressHostKey        string = "controller-ingress-host"
	SiteConfigControllerServiceAnnotationsKey string = "controller-service-annotations"
	SiteConfigControllerLoadBalancerIp        string = "controller-load-balancer-ip"

	//config-sync options
	SiteConfigConfigSyncCpuKey         string = "config-sync-cpu"
	SiteConfigConfigSyncMemoryKey      string = "config-sync-memory"
	SiteConfigConfigSyncCpuLimitKey    string = "config-sync-cpu-limit"
	SiteConfigConfigSyncMemoryLimitKey string = "config-sync-memory-limit"
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
	if spec.Routers != 0 {
		siteConfig.Data[SiteConfigRoutersKey] = strconv.Itoa(spec.Routers)
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
	if len(spec.IngressAnnotations) > 0 {
		var annotations []string
		for key, value := range spec.IngressAnnotations {
			annotations = append(annotations, key+"="+value)
		}
		siteConfig.Data[SiteConfigIngressAnnotationsKey] = strings.Join(annotations, ",")
	}
	if spec.ConsoleIngress != "" {
		siteConfig.Data[SiteConfigConsoleIngressKey] = spec.ConsoleIngress
	}
	if spec.IngressHost != "" {
		siteConfig.Data[SiteConfigIngressHostKey] = spec.IngressHost
	}
	if spec.CreateNetworkPolicy {
		siteConfig.Data[SiteConfigCreateNetworkPolicyKey] = "true"
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
	if spec.Router.CpuLimit != "" {
		if _, err := resource.ParseQuantity(spec.Router.CpuLimit); err != nil {
			return nil, fmt.Errorf("Invalid value for %s %q: %s", SiteConfigRouterCpuLimitKey, spec.Router.CpuLimit, err)
		}
		siteConfig.Data[SiteConfigRouterCpuLimitKey] = spec.Router.CpuLimit
	}
	if spec.Router.MemoryLimit != "" {
		if _, err := resource.ParseQuantity(spec.Router.MemoryLimit); err != nil {
			return nil, fmt.Errorf("Invalid value for %s %q: %s", SiteConfigRouterMemoryLimitKey, spec.Router.MemoryLimit, err)
		}
		siteConfig.Data[SiteConfigRouterMemoryLimitKey] = spec.Router.MemoryLimit
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
	if len(spec.Router.ServiceAnnotations) > 0 {
		var annotations []string
		for key, value := range spec.Router.ServiceAnnotations {
			annotations = append(annotations, key+"="+value)
		}
		siteConfig.Data[SiteConfigRouterServiceAnnotationsKey] = strings.Join(annotations, ",")
	}
	if spec.Router.LoadBalancerIp != "" {
		siteConfig.Data[SiteConfigRouterLoadBalancerIp] = spec.Router.LoadBalancerIp
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
	if spec.Controller.CpuLimit != "" {
		if _, err := resource.ParseQuantity(spec.Controller.CpuLimit); err != nil {
			return nil, fmt.Errorf("Invalid value for %s %q: %s", SiteConfigControllerCpuLimitKey, spec.Controller.CpuLimit, err)
		}
		siteConfig.Data[SiteConfigControllerCpuLimitKey] = spec.Controller.CpuLimit
	}
	if spec.Controller.MemoryLimit != "" {
		if _, err := resource.ParseQuantity(spec.Controller.MemoryLimit); err != nil {
			return nil, fmt.Errorf("Invalid value for %s %q: %s", SiteConfigControllerMemoryLimitKey, spec.Controller.MemoryLimit, err)
		}
		siteConfig.Data[SiteConfigControllerMemoryLimitKey] = spec.Controller.MemoryLimit
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
	if len(spec.Controller.ServiceAnnotations) > 0 {
		var annotations []string
		for key, value := range spec.Controller.ServiceAnnotations {
			annotations = append(annotations, key+"="+value)
		}
		siteConfig.Data[SiteConfigControllerServiceAnnotationsKey] = strings.Join(annotations, ",")
	}
	if spec.Controller.LoadBalancerIp != "" {
		siteConfig.Data[SiteConfigControllerLoadBalancerIp] = spec.Controller.LoadBalancerIp
	}

	if spec.ConfigSync.Cpu != "" {
		if _, err := resource.ParseQuantity(spec.ConfigSync.Cpu); err != nil {
			return nil, fmt.Errorf("Invalid value for %s %q: %s", SiteConfigConfigSyncCpuKey, spec.ConfigSync.Cpu, err)
		}
		siteConfig.Data[SiteConfigConfigSyncCpuKey] = spec.ConfigSync.Cpu
	}
	if spec.ConfigSync.Memory != "" {
		if _, err := resource.ParseQuantity(spec.ConfigSync.Memory); err != nil {
			return nil, fmt.Errorf("Invalid value for %s %q: %s", SiteConfigConfigSyncMemoryKey, spec.ConfigSync.Memory, err)
		}
		siteConfig.Data[SiteConfigConfigSyncMemoryKey] = spec.ConfigSync.Memory
	}
	if spec.ConfigSync.CpuLimit != "" {
		if _, err := resource.ParseQuantity(spec.ConfigSync.CpuLimit); err != nil {
			return nil, fmt.Errorf("Invalid value for %s %q: %s", SiteConfigConfigSyncCpuLimitKey, spec.ConfigSync.CpuLimit, err)
		}
		siteConfig.Data[SiteConfigConfigSyncCpuLimitKey] = spec.ConfigSync.CpuLimit
	}
	if spec.ConfigSync.MemoryLimit != "" {
		if _, err := resource.ParseQuantity(spec.ConfigSync.MemoryLimit); err != nil {
			return nil, fmt.Errorf("Invalid value for %s %q: %s", SiteConfigConfigSyncMemoryLimitKey, spec.ConfigSync.MemoryLimit, err)
		}
		siteConfig.Data[SiteConfigConfigSyncMemoryLimitKey] = spec.ConfigSync.MemoryLimit
	}

	// TODO: allow Replicas to be set through skupper-site configmap?
	if !spec.SiteControlled {
		if siteConfig.ObjectMeta.Labels == nil {
			siteConfig.ObjectMeta.Labels = map[string]string{}
		}
		siteConfig.ObjectMeta.Labels[types.SiteControllerIgnore] = "true"
	}
	if DefaultSkupperExtraLabels != "" {
		labelRegex := regexp.MustCompile(ValidRfc1123Label)
		if labelRegex.MatchString(DefaultSkupperExtraLabels) {
			s := strings.Split(DefaultSkupperExtraLabels, ",")
			for _, kv := range s {
				parts := strings.Split(kv, "=")
				if len(parts) > 1 {
					siteConfig.ObjectMeta.Labels[parts[0]] = parts[1]
				}
			}
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
