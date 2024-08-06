package site

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/skupperproject/skupper/pkg/qdr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

const (
	// core options
	SiteConfigNameKey                string = "name"
	SiteConfigRouterModeKey          string = "router-mode"
	SiteConfigIngressKey             string = "ingress"
	SiteConfigIngressAnnotationsKey  string = "ingress-annotations"
	SiteConfigIngressHostKey         string = "ingress-host"
	SiteConfigCreateNetworkPolicyKey string = "create-network-policy"
	SiteConfigRoutersKey             string = "routers"
	SiteConfigRunAsUserKey           string = "run-as-user"
	SiteConfigRunAsGroupKey          string = "run-as-group"
	SiteConfigClusterPermissionsKey  string = "cluster-permissions"

	// console options
	SiteConfigConsoleKey               string = "console"
	SiteConfigConsoleAuthenticationKey string = "console-authentication"
	SiteConfigConsoleUserKey           string = "console-user"
	SiteConfigConsolePasswordKey       string = "console-password"
	SiteConfigConsoleIngressKey        string = "console-ingress"
	SiteConfigRestAPIKey               string = "rest-api"

	// flow collector options
	SiteConfigFlowCollectorKey            string = "flow-collector"
	SiteConfigFlowCollectorRecordTtlKey   string = "flow-collector-record-ttl"
	SiteConfigFlowCollectorCpuKey         string = "flow-collector-cpu"
	SiteConfigFlowCollectorMemoryKey      string = "flow-collector-memory"
	SiteConfigFlowCollectorCpuLimitKey    string = "flow-collector-cpu-limit"
	SiteConfigFlowCollectorMemoryLimitKey string = "flow-collector-memory-limit"

	// prometheus server options
	SiteConfigPrometheusExternalServerKey       string = "prometheus-external-server"
	SiteConfigPrometheusServerAuthenticationKey string = "prometheus-server-authentication"
	SiteConfigPrometheusServerUserKey           string = "prometheus-server-user"
	SiteConfigPrometheusServerPasswordKey       string = "prometheus-server-password"
	SiteConfigPrometheusServerCpuKey            string = "prometheus-server-cpu"
	SiteConfigPrometheusServerMemoryKey         string = "prometheus-server-memory"
	SiteConfigPrometheusServerCpuLimitKey       string = "prometheus-server-cpu-limit"
	SiteConfigPrometheusServerMemoryLimitKey    string = "prometheus-server-memory-limit"
	SiteConfigPrometheusServerPodAnnotationsKey string = "prometheus-server-pod-annotations"

	// router options
	SiteConfigRouterConsoleKey             string = "router-console"
	SiteConfigRouterLoggingKey             string = "router-logging"
	SiteConfigRouterCpuKey                 string = "router-cpu"
	SiteConfigRouterMemoryKey              string = "router-memory"
	SiteConfigRouterCpuLimitKey            string = "router-cpu-limit"
	SiteConfigRouterMemoryLimitKey         string = "router-memory-limit"
	SiteConfigRouterAffinityKey            string = "router-pod-affinity"
	SiteConfigRouterAntiAffinityKey        string = "router-pod-antiaffinity"
	SiteConfigRouterNodeSelectorKey        string = "router-node-selector"
	SiteConfigRouterMaxFrameSizeKey        string = "xp-router-max-frame-size"
	SiteConfigRouterMaxSessionFramesKey    string = "xp-router-max-session-frames"
	SiteConfigRouterDataConnectionCountKey string = "router-data-connection-count"
	SiteConfigRouterIngressHostKey         string = "router-ingress-host"
	SiteConfigRouterServiceAnnotationsKey  string = "router-service-annotations"
	SiteConfigRouterPodAnnotationsKey      string = "router-pod-annotations"
	SiteConfigRouterLoadBalancerIp         string = "router-load-balancer-ip"
	SiteConfigRouterDisableMutualTLS       string = "router-disable-mutual-tls"
	SiteConfigRouterDropTcpConnections     string = "router-drop-tcp-connections"

	// controller options
	SiteConfigServiceControllerKey            string = "service-controller"
	SiteConfigServiceSyncKey                  string = "service-sync"
	SiteConfigServiceSyncSiteTtlKey           string = "service-sync-site-ttl"
	SiteConfigControllerCpuKey                string = "controller-cpu"
	SiteConfigControllerMemoryKey             string = "controller-memory"
	SiteConfigControllerCpuLimitKey           string = "controller-cpu-limit"
	SiteConfigControllerMemoryLimitKey        string = "controller-memory-limit"
	SiteConfigControllerAffinityKey           string = "controller-pod-affinity"
	SiteConfigControllerAntiAffinityKey       string = "controller-pod-antiaffinity"
	SiteConfigControllerNodeSelectorKey       string = "controller-node-selector"
	SiteConfigControllerIngressHostKey        string = "controller-ingress-host"
	SiteConfigControllerServiceAnnotationsKey string = "controller-service-annotations"
	SiteConfigControllerPodAnnotationsKey     string = "controller-pod-annotations"
	SiteConfigControllerLoadBalancerIp        string = "controller-load-balancer-ip"

	// config-sync options
	SiteConfigConfigSyncCpuKey         string = "config-sync-cpu"
	SiteConfigConfigSyncMemoryKey      string = "config-sync-memory"
	SiteConfigConfigSyncCpuLimitKey    string = "config-sync-cpu-limit"
	SiteConfigConfigSyncMemoryLimitKey string = "config-sync-memory-limit"

	SiteConfigEnableSkupperEventsKey string = "enable-skupper-events"

	//labels:
	ValidRfc1123Label                = `^(` + ValidRfc1123LabelKey + `)+=(` + ValidRfc1123LabelValue + `)+(,(` + ValidRfc1123LabelKey + `)+=(` + ValidRfc1123LabelValue + `)+)*$`
	ValidRfc1123LabelKey             = "[a-z0-9]([-._a-z0-9]*[a-z0-9])*"
	ValidRfc1123LabelValue           = "[a-zA-Z0-9]([-._a-zA-Z0-9]*[a-zA-Z0-9])*"
	DefaultSkupperExtraLabels string = ""
)

func WriteSiteConfig(spec types.SiteConfigSpec, namespace string) (*corev1.ConfigMap, error) {
	var errs []string
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
			SiteConfigNameKey:                  namespace,
			SiteConfigRouterModeKey:            string(types.TransportModeInterior),
			SiteConfigServiceControllerKey:     "true",
			SiteConfigServiceSyncKey:           "true",
			SiteConfigConsoleKey:               "false",
			SiteConfigFlowCollectorKey:         "false",
			SiteConfigClusterPermissionsKey:    "false",
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
	if spec.SiteTtl != 0 {
		siteConfig.Data[SiteConfigServiceSyncSiteTtlKey] = spec.SiteTtl.String()
	}
	if spec.EnableConsole {
		siteConfig.Data[SiteConfigConsoleKey] = "true"
	}
	if spec.EnableRestAPI {
		siteConfig.Data[SiteConfigRestAPIKey] = "true"
	}
	if spec.EnableFlowCollector {
		siteConfig.Data[SiteConfigFlowCollectorKey] = "true"
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
	if spec.RunAsUser != 0 {
		siteConfig.Data[SiteConfigRunAsUserKey] = strconv.FormatInt(spec.RunAsUser, 10)
	}
	if spec.RunAsGroup != 0 {
		siteConfig.Data[SiteConfigRunAsGroupKey] = strconv.FormatInt(spec.RunAsGroup, 10)
	}
	if spec.EnableClusterPermissions {
		siteConfig.Data[SiteConfigClusterPermissionsKey] = "true"
	}
	if spec.Router.Logging != nil {
		siteConfig.Data[SiteConfigRouterLoggingKey] = qdr.RouterLogConfigToString(spec.Router.Logging)
	}
	if spec.Router.Cpu != "" {
		if _, err := resource.ParseQuantity(spec.Router.Cpu); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigRouterCpuKey, spec.Router.Cpu, err))
		} else {
			siteConfig.Data[SiteConfigRouterCpuKey] = spec.Router.Cpu
		}
	}
	if spec.Router.Memory != "" {
		if _, err := resource.ParseQuantity(spec.Router.Memory); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigRouterMemoryKey, spec.Router.Memory, err))
		} else {
			siteConfig.Data[SiteConfigRouterMemoryKey] = spec.Router.Memory
		}
	}
	if spec.Router.CpuLimit != "" {
		if _, err := resource.ParseQuantity(spec.Router.CpuLimit); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigRouterCpuLimitKey, spec.Router.CpuLimit, err))
		} else {
			siteConfig.Data[SiteConfigRouterCpuLimitKey] = spec.Router.CpuLimit
		}
	}
	if spec.Router.MemoryLimit != "" {
		if _, err := resource.ParseQuantity(spec.Router.MemoryLimit); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigRouterMemoryLimitKey, spec.Router.MemoryLimit, err))
		} else {
			siteConfig.Data[SiteConfigRouterMemoryLimitKey] = spec.Router.MemoryLimit
		}
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
	if spec.Router.DataConnectionCount != "" {
		siteConfig.Data[SiteConfigRouterDataConnectionCountKey] = spec.Router.DataConnectionCount
	}
	if len(spec.Router.ServiceAnnotations) > 0 {
		var annotations []string
		for key, value := range spec.Router.ServiceAnnotations {
			annotations = append(annotations, key+"="+value)
		}
		siteConfig.Data[SiteConfigRouterServiceAnnotationsKey] = strings.Join(annotations, ",")
	}
	if len(spec.Router.PodAnnotations) > 0 {
		var annotations []string
		for key, value := range spec.Router.PodAnnotations {
			annotations = append(annotations, key+"="+value)
		}
		siteConfig.Data[SiteConfigRouterPodAnnotationsKey] = strings.Join(annotations, ",")
	}
	if spec.Router.LoadBalancerIp != "" {
		siteConfig.Data[SiteConfigRouterLoadBalancerIp] = spec.Router.LoadBalancerIp
	}
	if spec.Router.DisableMutualTLS {
		siteConfig.Data[SiteConfigRouterDisableMutualTLS] = "true"
	}
	if spec.Router.DropTcpConnections {
		siteConfig.Data[SiteConfigRouterDropTcpConnections] = "true"
	}
	if spec.Controller.Cpu != "" {
		if _, err := resource.ParseQuantity(spec.Controller.Cpu); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigControllerCpuKey, spec.Controller.Cpu, err))
		} else {
			siteConfig.Data[SiteConfigControllerCpuKey] = spec.Controller.Cpu
		}
	}
	if spec.Controller.Memory != "" {
		if _, err := resource.ParseQuantity(spec.Controller.Memory); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigControllerMemoryKey, spec.Controller.Memory, err))
		} else {
			siteConfig.Data[SiteConfigControllerMemoryKey] = spec.Controller.Memory
		}
	}
	if spec.Controller.CpuLimit != "" {
		if _, err := resource.ParseQuantity(spec.Controller.CpuLimit); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigControllerCpuLimitKey, spec.Controller.CpuLimit, err))
		} else {
			siteConfig.Data[SiteConfigControllerCpuLimitKey] = spec.Controller.CpuLimit
		}
	}
	if spec.Controller.MemoryLimit != "" {
		if _, err := resource.ParseQuantity(spec.Controller.MemoryLimit); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigControllerMemoryLimitKey, spec.Controller.MemoryLimit, err))
		} else {
			siteConfig.Data[SiteConfigControllerMemoryLimitKey] = spec.Controller.MemoryLimit
		}
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
	if len(spec.Controller.PodAnnotations) > 0 {
		var annotations []string
		for key, value := range spec.Controller.PodAnnotations {
			annotations = append(annotations, key+"="+value)
		}
		siteConfig.Data[SiteConfigControllerPodAnnotationsKey] = strings.Join(annotations, ",")
	}
	if spec.Controller.LoadBalancerIp != "" {
		siteConfig.Data[SiteConfigControllerLoadBalancerIp] = spec.Controller.LoadBalancerIp
	}

	if spec.ConfigSync.Cpu != "" {
		if _, err := resource.ParseQuantity(spec.ConfigSync.Cpu); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigConfigSyncCpuKey, spec.ConfigSync.Cpu, err))
		} else {
			siteConfig.Data[SiteConfigConfigSyncCpuKey] = spec.ConfigSync.Cpu
		}
	}
	if spec.ConfigSync.Memory != "" {
		if _, err := resource.ParseQuantity(spec.ConfigSync.Memory); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigConfigSyncMemoryKey, spec.ConfigSync.Memory, err))
		} else {
			siteConfig.Data[SiteConfigConfigSyncMemoryKey] = spec.ConfigSync.Memory
		}
	}
	if spec.ConfigSync.CpuLimit != "" {
		if _, err := resource.ParseQuantity(spec.ConfigSync.CpuLimit); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigConfigSyncCpuLimitKey, spec.ConfigSync.CpuLimit, err))
		} else {
			siteConfig.Data[SiteConfigConfigSyncCpuLimitKey] = spec.ConfigSync.CpuLimit
		}
	}
	if spec.ConfigSync.MemoryLimit != "" {
		if _, err := resource.ParseQuantity(spec.ConfigSync.MemoryLimit); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigConfigSyncMemoryLimitKey, spec.ConfigSync.MemoryLimit, err))
		} else {
			siteConfig.Data[SiteConfigConfigSyncMemoryLimitKey] = spec.ConfigSync.MemoryLimit
		}
	}

	if spec.FlowCollector.FlowRecordTtl != 0 {
		siteConfig.Data[SiteConfigFlowCollectorRecordTtlKey] = spec.FlowCollector.FlowRecordTtl.String()
	}
	if spec.FlowCollector.Cpu != "" {
		if _, err := resource.ParseQuantity(spec.FlowCollector.Cpu); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigFlowCollectorCpuKey, spec.FlowCollector.Cpu, err))
		} else {
			siteConfig.Data[SiteConfigFlowCollectorCpuKey] = spec.FlowCollector.Cpu
		}
	}
	if spec.FlowCollector.Memory != "" {
		if _, err := resource.ParseQuantity(spec.FlowCollector.Memory); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigFlowCollectorMemoryKey, spec.FlowCollector.Memory, err))
		} else {
			siteConfig.Data[SiteConfigFlowCollectorMemoryKey] = spec.FlowCollector.Memory
		}
	}
	if spec.FlowCollector.CpuLimit != "" {
		if _, err := resource.ParseQuantity(spec.FlowCollector.CpuLimit); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigFlowCollectorCpuLimitKey, spec.FlowCollector.CpuLimit, err))
		} else {
			siteConfig.Data[SiteConfigFlowCollectorCpuLimitKey] = spec.FlowCollector.CpuLimit
		}
	}
	if spec.FlowCollector.MemoryLimit != "" {
		if _, err := resource.ParseQuantity(spec.FlowCollector.MemoryLimit); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigFlowCollectorMemoryLimitKey, spec.FlowCollector.MemoryLimit, err))
		} else {
			siteConfig.Data[SiteConfigFlowCollectorMemoryLimitKey] = spec.FlowCollector.MemoryLimit
		}
	}

	if spec.EnableSkupperEvents {
		siteConfig.Data[SiteConfigEnableSkupperEventsKey] = "true"
	} else {
		siteConfig.Data[SiteConfigEnableSkupperEventsKey] = "false"
	}

	if spec.PrometheusServer.ExternalServer != "" {
		siteConfig.Data[SiteConfigPrometheusExternalServerKey] = spec.PrometheusServer.ExternalServer
	}
	if spec.PrometheusServer.AuthMode != "" {
		siteConfig.Data[SiteConfigPrometheusServerAuthenticationKey] = spec.PrometheusServer.AuthMode
	}
	if spec.PrometheusServer.User != "" {
		siteConfig.Data[SiteConfigPrometheusServerUserKey] = spec.PrometheusServer.User
	}
	if spec.PrometheusServer.Password != "" {
		siteConfig.Data[SiteConfigPrometheusServerPasswordKey] = spec.PrometheusServer.Password
	}
	if spec.PrometheusServer.Cpu != "" {
		if _, err := resource.ParseQuantity(spec.PrometheusServer.Cpu); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigPrometheusServerCpuKey, spec.PrometheusServer.Cpu, err))
		} else {
			siteConfig.Data[SiteConfigPrometheusServerCpuKey] = spec.PrometheusServer.Cpu
		}
	}
	if spec.PrometheusServer.Memory != "" {
		if _, err := resource.ParseQuantity(spec.PrometheusServer.Memory); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigPrometheusServerMemoryKey, spec.PrometheusServer.Memory, err))
		} else {
			siteConfig.Data[SiteConfigPrometheusServerMemoryKey] = spec.PrometheusServer.Memory
		}
	}
	if spec.PrometheusServer.CpuLimit != "" {
		if _, err := resource.ParseQuantity(spec.PrometheusServer.CpuLimit); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigPrometheusServerCpuLimitKey, spec.PrometheusServer.CpuLimit, err))
		} else {
			siteConfig.Data[SiteConfigPrometheusServerCpuLimitKey] = spec.PrometheusServer.CpuLimit
		}
	}
	if spec.PrometheusServer.MemoryLimit != "" {
		if _, err := resource.ParseQuantity(spec.PrometheusServer.MemoryLimit); err != nil {
			errs = append(errs, fmt.Sprintf("Invalid value for %s %q: %s", SiteConfigPrometheusServerMemoryLimitKey, spec.PrometheusServer.MemoryLimit, err))
		} else {
			siteConfig.Data[SiteConfigPrometheusServerMemoryLimitKey] = spec.PrometheusServer.MemoryLimit
		}
	}
	if len(spec.PrometheusServer.PodAnnotations) > 0 {
		var annotations []string
		for key, value := range spec.PrometheusServer.PodAnnotations {
			annotations = append(annotations, key+"="+value)
		}
		siteConfig.Data[SiteConfigPrometheusServerPodAnnotationsKey] = strings.Join(annotations, ",")
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
	if len(errs) > 0 {
		return siteConfig, fmt.Errorf(strings.Join(errs, ", "))
	}
	return siteConfig, nil
}

func ReadSiteConfig(siteConfig *corev1.ConfigMap, namespace string, defaultIngress string) (*types.SiteConfig, error) {
	var errs []string
	var result types.SiteConfig
	result.Spec.SkupperNamespace = siteConfig.Namespace
	// TODO: what should the defaults be for name, namespace
	if skupperName, ok := siteConfig.Data[SiteConfigNameKey]; ok {
		result.Spec.SkupperName = skupperName
	} else {
		result.Spec.SkupperName = namespace
	}
	if routerMode, ok := siteConfig.Data[SiteConfigRouterModeKey]; ok {
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
	if routers, ok := siteConfig.Data[SiteConfigRoutersKey]; ok {
		result.Spec.Routers, _ = strconv.Atoi(routers)
	}
	if enableController, ok := siteConfig.Data[SiteConfigServiceControllerKey]; ok {
		result.Spec.EnableController, _ = strconv.ParseBool(enableController)
	} else {
		result.Spec.EnableController = true
	}
	if enableServiceSync, ok := siteConfig.Data[SiteConfigServiceSyncKey]; ok {
		result.Spec.EnableServiceSync, _ = strconv.ParseBool(enableServiceSync)
	} else {
		result.Spec.EnableServiceSync = true
	}
	if value, ok := siteConfig.Data[SiteConfigServiceSyncSiteTtlKey]; ok {
		ttl, err := time.ParseDuration(value)
		if err == nil {
			result.Spec.SiteTtl = ttl
		}
	}
	if enableConsole, ok := siteConfig.Data[SiteConfigConsoleKey]; ok {
		result.Spec.EnableConsole, _ = strconv.ParseBool(enableConsole)
	} else {
		result.Spec.EnableConsole = false
	}
	if enableFlowCollector, ok := siteConfig.Data[SiteConfigFlowCollectorKey]; ok {
		result.Spec.EnableFlowCollector, _ = strconv.ParseBool(enableFlowCollector)
	} else {
		result.Spec.EnableFlowCollector = false
	}
	if value, ok := siteConfig.Data[SiteConfigFlowCollectorRecordTtlKey]; ok {
		ttl, err := time.ParseDuration(value)
		if err == nil {
			result.Spec.FlowCollector.FlowRecordTtl = ttl
		}
	} else {
		result.Spec.FlowCollector.FlowRecordTtl = types.DefaultFlowTimeoutDuration
	}
	if value, ok := siteConfig.Data[SiteConfigRestAPIKey]; ok {
		result.Spec.EnableRestAPI, _ = strconv.ParseBool(value)
	} else {
		result.Spec.EnableRestAPI = result.Spec.EnableConsole
	}
	if enableClusterPermissions, ok := siteConfig.Data[SiteConfigClusterPermissionsKey]; ok {
		result.Spec.EnableClusterPermissions, _ = strconv.ParseBool(enableClusterPermissions)
	} else {
		result.Spec.EnableClusterPermissions = false
	}
	if createNetworkPolicy, ok := siteConfig.Data[SiteConfigCreateNetworkPolicyKey]; ok {
		result.Spec.CreateNetworkPolicy, _ = strconv.ParseBool(createNetworkPolicy)
	} else {
		result.Spec.CreateNetworkPolicy = false
	}
	if authMode, ok := siteConfig.Data[SiteConfigConsoleAuthenticationKey]; ok {
		result.Spec.AuthMode = authMode
	} else {
		result.Spec.AuthMode = types.ConsoleAuthModeInternal
	}
	if user, ok := siteConfig.Data[SiteConfigConsoleUserKey]; ok {
		result.Spec.User = user
	} else {
		result.Spec.User = ""
	}
	if password, ok := siteConfig.Data[SiteConfigConsolePasswordKey]; ok {
		result.Spec.Password = password
	} else {
		result.Spec.Password = ""
	}
	if ingress, ok := siteConfig.Data[SiteConfigIngressKey]; ok {
		result.Spec.Ingress = ingress
	} else {
		// check for deprecated key
		if clusterLocal, ok := siteConfig.Data["cluster-local"]; ok && clusterLocal == "true" {
			result.Spec.Ingress = types.IngressNoneString
		} else {
			result.Spec.Ingress = defaultIngress
		}
	}
	if ingressAnnotations, ok := siteConfig.Data[SiteConfigIngressAnnotationsKey]; ok {
		result.Spec.IngressAnnotations = asMap(splitWithEscaping(ingressAnnotations, ',', '\\'))
	}
	if consoleIngress, ok := siteConfig.Data[SiteConfigConsoleIngressKey]; ok {
		result.Spec.ConsoleIngress = consoleIngress
	}
	if ingressHost, ok := siteConfig.Data[SiteConfigIngressHostKey]; ok {
		result.Spec.IngressHost = ingressHost
	}
	if runAsUser, ok := siteConfig.Data[SiteConfigRunAsUserKey]; ok {
		result.Spec.RunAsUser, _ = strconv.ParseInt(runAsUser, 10, 64)
	}
	if runAsGroup, ok := siteConfig.Data[SiteConfigRunAsGroupKey]; ok {
		result.Spec.RunAsGroup, _ = strconv.ParseInt(runAsGroup, 10, 64)
	}
	// TODO: allow Replicas to be set through skupper-site configmap?
	if siteConfig.ObjectMeta.Labels == nil {
		result.Spec.SiteControlled = true
	} else if ignore, ok := siteConfig.ObjectMeta.Labels[types.SiteControllerIgnore]; ok {
		siteIgnore, _ := strconv.ParseBool(ignore)
		result.Spec.SiteControlled = !siteIgnore
	} else {
		result.Spec.SiteControlled = true
	}
	result.Reference.UID = string(siteConfig.ObjectMeta.UID)
	result.Reference.Name = siteConfig.ObjectMeta.Name
	result.Reference.Kind = siteConfig.TypeMeta.Kind
	result.Reference.APIVersion = siteConfig.TypeMeta.APIVersion
	if routerLogging, ok := siteConfig.Data[SiteConfigRouterLoggingKey]; ok && routerLogging != "" {
		logConf, err := qdr.ParseRouterLogConfig(routerLogging)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			result.Spec.Router.Logging = logConf
		}
	} else {
		logConf, err := qdr.ParseRouterLogConfig("ROUTER_CORE:error+")
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			result.Spec.Router.Logging = logConf
		}
	}
	if routerCpu, ok := siteConfig.Data[SiteConfigRouterCpuKey]; ok && routerCpu != "" {
		result.Spec.Router.Cpu = routerCpu
	}
	if routerMemory, ok := siteConfig.Data[SiteConfigRouterMemoryKey]; ok && routerMemory != "" {
		result.Spec.Router.Memory = routerMemory
	}
	if routerCpuLimit, ok := siteConfig.Data[SiteConfigRouterCpuLimitKey]; ok && routerCpuLimit != "" {
		result.Spec.Router.CpuLimit = routerCpuLimit
	}
	if routerMemoryLimit, ok := siteConfig.Data[SiteConfigRouterMemoryLimitKey]; ok && routerMemoryLimit != "" {
		result.Spec.Router.MemoryLimit = routerMemoryLimit
	}
	if routerNodeSelector, ok := siteConfig.Data[SiteConfigRouterNodeSelectorKey]; ok && routerNodeSelector != "" {
		result.Spec.Router.NodeSelector = routerNodeSelector
	}
	if routerAffinity, ok := siteConfig.Data[SiteConfigRouterAffinityKey]; ok && routerAffinity != "" {
		result.Spec.Router.Affinity = routerAffinity
	}
	if routerAntiAffinity, ok := siteConfig.Data[SiteConfigRouterAntiAffinityKey]; ok && routerAntiAffinity != "" {
		result.Spec.Router.AntiAffinity = routerAntiAffinity
	}
	if routerIngressHost, ok := siteConfig.Data[SiteConfigRouterIngressHostKey]; ok && routerIngressHost != "" {
		result.Spec.Router.IngressHost = routerIngressHost
	}

	if routerMaxFrameSize, ok := siteConfig.Data[SiteConfigRouterMaxFrameSizeKey]; ok && routerMaxFrameSize != "" {
		val, err := strconv.Atoi(routerMaxFrameSize)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			result.Spec.Router.MaxFrameSize = val
		}
	} else {
		result.Spec.Router.MaxFrameSize = types.RouterMaxFrameSizeDefault
	}
	if routerMaxSessionFrames, ok := siteConfig.Data[SiteConfigRouterMaxSessionFramesKey]; ok && routerMaxSessionFrames != "" {
		val, err := strconv.Atoi(routerMaxSessionFrames)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			result.Spec.Router.MaxSessionFrames = val
		}
	} else {
		result.Spec.Router.MaxSessionFrames = types.RouterMaxSessionFramesDefault
	}
	if routerDataConnectionCount, ok := siteConfig.Data[SiteConfigRouterDataConnectionCountKey]; ok && routerDataConnectionCount != "" {
		result.Spec.Router.DataConnectionCount = routerDataConnectionCount
	}

	if routerServiceAnnotations, ok := siteConfig.Data[SiteConfigRouterServiceAnnotationsKey]; ok {
		result.Spec.Router.ServiceAnnotations = asMap(splitWithEscaping(routerServiceAnnotations, ',', '\\'))
	}
	if routerPodAnnotations, ok := siteConfig.Data[SiteConfigRouterPodAnnotationsKey]; ok {
		result.Spec.Router.PodAnnotations = asMap(splitWithEscaping(routerPodAnnotations, ',', '\\'))
	}
	if routerServiceLoadBalancerIp, ok := siteConfig.Data[SiteConfigRouterLoadBalancerIp]; ok {
		result.Spec.Router.LoadBalancerIp = routerServiceLoadBalancerIp
	}
	if value, ok := siteConfig.Data[SiteConfigRouterDisableMutualTLS]; ok {
		result.Spec.Router.DisableMutualTLS, _ = strconv.ParseBool(value)
	}
	if value, ok := siteConfig.Data[SiteConfigRouterDropTcpConnections]; ok {
		result.Spec.Router.DropTcpConnections, _ = strconv.ParseBool(value)
	}

	if controllerCpu, ok := siteConfig.Data[SiteConfigControllerCpuKey]; ok && controllerCpu != "" {
		result.Spec.Controller.Cpu = controllerCpu
	}
	if controllerMemory, ok := siteConfig.Data[SiteConfigControllerMemoryKey]; ok && controllerMemory != "" {
		result.Spec.Controller.Memory = controllerMemory
	}
	if controllerCpuLimit, ok := siteConfig.Data[SiteConfigControllerCpuLimitKey]; ok && controllerCpuLimit != "" {
		result.Spec.Controller.CpuLimit = controllerCpuLimit
	}
	if controllerMemoryLimit, ok := siteConfig.Data[SiteConfigControllerMemoryLimitKey]; ok && controllerMemoryLimit != "" {
		result.Spec.Controller.MemoryLimit = controllerMemoryLimit
	}
	if controllerNodeSelector, ok := siteConfig.Data[SiteConfigControllerNodeSelectorKey]; ok && controllerNodeSelector != "" {
		result.Spec.Controller.NodeSelector = controllerNodeSelector
	}
	if controllerAffinity, ok := siteConfig.Data[SiteConfigControllerAffinityKey]; ok && controllerAffinity != "" {
		result.Spec.Controller.Affinity = controllerAffinity
	}
	if controllerAntiAffinity, ok := siteConfig.Data[SiteConfigControllerAntiAffinityKey]; ok && controllerAntiAffinity != "" {
		result.Spec.Controller.AntiAffinity = controllerAntiAffinity
	}
	if controllerIngressHost, ok := siteConfig.Data[SiteConfigControllerIngressHostKey]; ok && controllerIngressHost != "" {
		result.Spec.Controller.IngressHost = controllerIngressHost
	}
	if controllerServiceAnnotations, ok := siteConfig.Data[SiteConfigControllerServiceAnnotationsKey]; ok {
		result.Spec.Controller.ServiceAnnotations = asMap(splitWithEscaping(controllerServiceAnnotations, ',', '\\'))
	}
	if controllerPodAnnotations, ok := siteConfig.Data[SiteConfigControllerPodAnnotationsKey]; ok {
		result.Spec.Controller.PodAnnotations = asMap(splitWithEscaping(controllerPodAnnotations, ',', '\\'))
	}
	if controllerServiceLoadBalancerIp, ok := siteConfig.Data[SiteConfigControllerLoadBalancerIp]; ok {
		result.Spec.Controller.LoadBalancerIp = controllerServiceLoadBalancerIp
	}

	if configSyncCpu, ok := siteConfig.Data[SiteConfigConfigSyncCpuKey]; ok && configSyncCpu != "" {
		result.Spec.ConfigSync.Cpu = configSyncCpu
	}
	if configSyncMemory, ok := siteConfig.Data[SiteConfigConfigSyncMemoryKey]; ok && configSyncMemory != "" {
		result.Spec.ConfigSync.Memory = configSyncMemory
	}
	if configSyncCpuLimit, ok := siteConfig.Data[SiteConfigConfigSyncCpuLimitKey]; ok && configSyncCpuLimit != "" {
		result.Spec.ConfigSync.CpuLimit = configSyncCpuLimit
	}
	if configSyncMemoryLimit, ok := siteConfig.Data[SiteConfigConfigSyncMemoryLimitKey]; ok && configSyncMemoryLimit != "" {
		result.Spec.ConfigSync.MemoryLimit = configSyncMemoryLimit
	}

	if value, ok := siteConfig.Data[SiteConfigEnableSkupperEventsKey]; ok {
		result.Spec.EnableSkupperEvents, _ = strconv.ParseBool(value)
	}

	if flowCollectorCpu, ok := siteConfig.Data[SiteConfigFlowCollectorCpuKey]; ok && flowCollectorCpu != "" {
		result.Spec.FlowCollector.Cpu = flowCollectorCpu
	}
	if flowCollectorMemory, ok := siteConfig.Data[SiteConfigFlowCollectorMemoryKey]; ok && flowCollectorMemory != "" {
		result.Spec.FlowCollector.Memory = flowCollectorMemory
	}
	if flowCollectorCpuLimit, ok := siteConfig.Data[SiteConfigFlowCollectorCpuLimitKey]; ok && flowCollectorCpuLimit != "" {
		result.Spec.FlowCollector.CpuLimit = flowCollectorCpuLimit
	}
	if flowCollectorMemoryLimit, ok := siteConfig.Data[SiteConfigFlowCollectorMemoryLimitKey]; ok && flowCollectorMemoryLimit != "" {
		result.Spec.FlowCollector.MemoryLimit = flowCollectorMemoryLimit
	}

	if externalServer, ok := siteConfig.Data[SiteConfigPrometheusExternalServerKey]; ok {
		result.Spec.PrometheusServer.ExternalServer = externalServer
	} else {
		result.Spec.PrometheusServer.ExternalServer = ""
	}
	if authMode, ok := siteConfig.Data[SiteConfigPrometheusServerAuthenticationKey]; ok {
		result.Spec.PrometheusServer.AuthMode = authMode
	} else {
		result.Spec.PrometheusServer.AuthMode = string(types.PrometheusAuthModeTls)
	}
	if user, ok := siteConfig.Data[SiteConfigPrometheusServerUserKey]; ok {
		result.Spec.PrometheusServer.User = user
	} else {
		result.Spec.PrometheusServer.User = ""
	}
	if password, ok := siteConfig.Data[SiteConfigPrometheusServerPasswordKey]; ok {
		result.Spec.PrometheusServer.Password = password
	} else {
		result.Spec.PrometheusServer.Password = ""
	}
	if prometheusCpu, ok := siteConfig.Data[SiteConfigPrometheusServerCpuKey]; ok && prometheusCpu != "" {
		result.Spec.PrometheusServer.Cpu = prometheusCpu
	}
	if prometheusMemory, ok := siteConfig.Data[SiteConfigPrometheusServerMemoryKey]; ok && prometheusMemory != "" {
		result.Spec.PrometheusServer.Memory = prometheusMemory
	}
	if prometheusCpuLimit, ok := siteConfig.Data[SiteConfigPrometheusServerCpuLimitKey]; ok && prometheusCpuLimit != "" {
		result.Spec.PrometheusServer.CpuLimit = prometheusCpuLimit
	}
	if prometheusMemoryLimit, ok := siteConfig.Data[SiteConfigPrometheusServerMemoryLimitKey]; ok && prometheusMemoryLimit != "" {
		result.Spec.PrometheusServer.MemoryLimit = prometheusMemoryLimit
	}
	if prometheusPodAnnotations, ok := siteConfig.Data[SiteConfigPrometheusServerPodAnnotationsKey]; ok {
		result.Spec.PrometheusServer.PodAnnotations = asMap(splitWithEscaping(prometheusPodAnnotations, ',', '\\'))
	}

	annotations, labels := GetSiteAnnotationsAndLabels(siteConfig)
	result.Spec.Annotations = annotations
	result.Spec.Labels = labels
	if len(errs) > 0 {
		return &result, fmt.Errorf(strings.Join(errs, ", "))
	}
	return &result, nil
}

func GetSiteAnnotationsAndLabels(siteConfig *corev1.ConfigMap) (map[string]string, map[string]string) {
	annotationExclusions := []string{}
	labelExclusions := []string{}
	annotations := map[string]string{}
	for key, value := range siteConfig.ObjectMeta.Annotations {
		if key == types.AnnotationExcludes {
			annotationExclusions = strings.Split(value, ",")
		} else if key == types.LabelExcludes {
			labelExclusions = strings.Split(value, ",")
		} else {
			annotations[key] = value
		}
	}
	for _, key := range annotationExclusions {
		delete(annotations, key)
	}
	labels := map[string]string{}
	for key, value := range siteConfig.ObjectMeta.Labels {
		if key != types.SiteControllerIgnore {
			labels[key] = value
		}
	}
	for _, key := range labelExclusions {
		delete(labels, key)
	}
	return annotations, labels
}

func UpdateLogging(config types.SiteConfigSpec, configmap *corev1.ConfigMap) bool {
	latestLogging := qdr.RouterLogConfigToString(config.Router.Logging)
	if configmap.Data[SiteConfigRouterLoggingKey] != latestLogging {
		configmap.Data[SiteConfigRouterLoggingKey] = latestLogging
		return true
	}
	return false
}

func UpdateForCollectorEnabled(configmap *corev1.ConfigMap) {
	configmap.Data[SiteConfigConsoleKey] = "true"
	configmap.Data[SiteConfigFlowCollectorKey] = "true"
}

func splitWithEscaping(s string, separator, escape byte) []string {
	var token []byte
	var tokens []string
	for i := 0; i < len(s); i++ {
		if s[i] == separator {
			tokens = append(tokens, strings.TrimSpace(string(token)))
			token = token[:0]
		} else if s[i] == escape && i+1 < len(s) {
			i++
			token = append(token, s[i])
		} else {
			token = append(token, s[i])
		}
	}
	tokens = append(tokens, strings.TrimSpace(string(token)))
	return tokens
}

func asMap(entries []string) map[string]string {
	result := map[string]string{}
	for _, entry := range entries {
		parts := strings.Split(entry, "=")
		if len(parts) > 1 {
			result[parts[0]] = parts[1]
		} else {
			result[parts[0]] = ""
		}
	}
	return result
}
