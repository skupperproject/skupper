package client

import (
	"context"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

func (cli *VanClient) SiteConfigInspect(ctx context.Context, input *corev1.ConfigMap) (*types.SiteConfig, error) {
	var namespace string
	if input == nil {
		namespace = cli.Namespace
	} else {
		namespace = input.ObjectMeta.Namespace
	}
	return cli.SiteConfigInspectInNamespace(ctx, input, namespace)
}

func (cli *VanClient) SiteConfigInspectInNamespace(ctx context.Context, input *corev1.ConfigMap, namespace string) (*types.SiteConfig, error) {
	var siteConfig *corev1.ConfigMap
	if input == nil {
		cm, err := cli.KubeClient.CoreV1().ConfigMaps(namespace).Get(types.SiteConfigMapName, metav1.GetOptions{})
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
	if enableConsole, ok := siteConfig.Data[SiteConfigConsoleKey]; ok {
		result.Spec.EnableConsole, _ = strconv.ParseBool(enableConsole)
	} else {
		result.Spec.EnableConsole = true
	}
	if enableRouterConsole, ok := siteConfig.Data[SiteConfigRouterConsoleKey]; ok {
		result.Spec.EnableRouterConsole, _ = strconv.ParseBool(enableRouterConsole)
	} else {
		result.Spec.EnableRouterConsole = false
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
		if clusterLocal, ok := siteConfig.Data["cluster-local"]; ok {
			if clusterLocal == "true" {
				result.Spec.Ingress = types.IngressNoneString
			} else {
				result.Spec.Ingress = types.IngressLoadBalancerString
			}
		} else {
			result.Spec.Ingress = cli.GetIngressDefault()
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
	if routerDebugMode, ok := siteConfig.Data[SiteConfigRouterDebugModeKey]; ok && routerDebugMode != "" {
		result.Spec.Router.DebugMode = routerDebugMode
	}
	if routerLogging, ok := siteConfig.Data[SiteConfigRouterLoggingKey]; ok && routerLogging != "" {
		logConf, err := ParseRouterLogConfig(routerLogging)
		if err != nil {
			return &result, err
		}
		result.Spec.Router.Logging = logConf
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
			return &result, err
		}
		result.Spec.Router.MaxFrameSize = val
	} else {
		result.Spec.Router.MaxFrameSize = types.RouterMaxFrameSizeDefault
	}
	if routerMaxSessionFrames, ok := siteConfig.Data[SiteConfigRouterMaxSessionFramesKey]; ok && routerMaxSessionFrames != "" {
		val, err := strconv.Atoi(routerMaxSessionFrames)
		if err != nil {
			return &result, err
		}
		result.Spec.Router.MaxSessionFrames = val
	} else {
		result.Spec.Router.MaxSessionFrames = types.RouterMaxSessionFramesDefault
	}

	if routerServiceAnnotations, ok := siteConfig.Data[SiteConfigRouterServiceAnnotationsKey]; ok {
		result.Spec.Router.ServiceAnnotations = asMap(splitWithEscaping(routerServiceAnnotations, ',', '\\'))
	}
	if routerServiceLoadBalancerIp, ok := siteConfig.Data[SiteConfigRouterLoadBalancerIp]; ok {
		result.Spec.Router.LoadBalancerIp = routerServiceLoadBalancerIp
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
	result.Spec.Annotations = annotations
	labels := map[string]string{}
	for key, value := range siteConfig.ObjectMeta.Labels {
		if key != types.SiteControllerIgnore {
			labels[key] = value
		}
	}
	for _, key := range labelExclusions {
		delete(labels, key)
	}
	result.Spec.Labels = labels
	return &result, nil
}
