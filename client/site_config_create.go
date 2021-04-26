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

func (cli *VanClient) SiteConfigCreate(ctx context.Context, spec types.SiteConfigSpec) (*types.SiteConfig, error) {
	siteConfig := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "skupper-site",
			Annotations: spec.Annotations,
			Labels:      spec.Labels,
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
	if spec.ConsoleIngress != "" {
		siteConfig.Data["console-ingress"] = spec.ConsoleIngress
	}
	if spec.Router.Logging != nil {
		siteConfig.Data["router-logging"] = RouterLogConfigToString(spec.Router.Logging)
	}
	if spec.Router.DebugMode != "" {
		siteConfig.Data["router-debug-mode"] = spec.Router.DebugMode
	}
	if spec.Router.Cpu != "" {
		if _, err := resource.ParseQuantity(spec.Router.Cpu); err != nil {
			return nil, fmt.Errorf("Invalid value for router-cpu %q: %s", spec.Router.Cpu, err)
		}
		siteConfig.Data["router-cpu"] = spec.Router.Cpu
	}
	if spec.Router.Memory != "" {
		if _, err := resource.ParseQuantity(spec.Router.Memory); err != nil {
			return nil, fmt.Errorf("Invalid value for router-memory %q: %s", spec.Router.Memory, err)
		}
		siteConfig.Data["router-memory"] = spec.Router.Memory
	}
	if spec.Router.Affinity != "" {
		siteConfig.Data["router-affinity"] = spec.Router.Affinity
	}
	if spec.Router.AntiAffinity != "" {
		siteConfig.Data["router-anti-affinity"] = spec.Router.AntiAffinity
	}
	if spec.Router.NodeSelector != "" {
		siteConfig.Data["router-node-selector"] = spec.Router.NodeSelector
	}
	if spec.Router.MaxFrameSize != types.RouterMaxFrameSizeDefault {
		siteConfig.Data["xp-router-max-frame-size"] = strconv.Itoa(spec.Router.MaxFrameSize)
	}
	if spec.Router.MaxSessionFrames != types.RouterMaxSessionFramesDefault {
		siteConfig.Data["xp-router-max-session-frames"] = strconv.Itoa(spec.Router.MaxSessionFrames)
	}
	if spec.Controller.Cpu != "" {
		if _, err := resource.ParseQuantity(spec.Controller.Cpu); err != nil {
			return nil, fmt.Errorf("Invalid value for controller-cpu %q: %s", spec.Controller.Cpu, err)
		}
		siteConfig.Data["controller-cpu"] = spec.Controller.Cpu
	}
	if spec.Controller.Memory != "" {
		if _, err := resource.ParseQuantity(spec.Controller.Memory); err != nil {
			return nil, fmt.Errorf("Invalid value for controller-memory %q: %s", spec.Controller.Memory, err)
		}
		siteConfig.Data["controller-memory"] = spec.Controller.Memory
	}
	if spec.Controller.Affinity != "" {
		siteConfig.Data["controller-affinity"] = spec.Controller.Affinity
	}
	if spec.Controller.AntiAffinity != "" {
		siteConfig.Data["controller-anti-affinity"] = spec.Controller.AntiAffinity
	}
	if spec.Controller.NodeSelector != "" {
		siteConfig.Data["controller-node-selector"] = spec.Controller.NodeSelector
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
