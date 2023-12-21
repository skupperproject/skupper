package client

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/briandowns/spinner"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/images"
	"github.com/skupperproject/skupper/pkg/version"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/kube/resolver"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
)

func getServingSecretAnnotations(name string) map[string]string {
	return map[string]string{
		"service.alpha.openshift.io/serving-cert-secret-name": name,
	}
}

func OauthProxyContainer(serviceAccount string, servicePort string) *corev1.Container {
	image := images.GetOauthProxyImageDetails()
	return &corev1.Container{
		Image:           image.Name,
		ImagePullPolicy: kube.GetPullPolicy(image.PullPolicy),
		Name:            "oauth-proxy",
		Args: []string{
			"--https-address=:" + strconv.Itoa(int(types.ConsoleOpenShiftOauthServiceTargetPort)),
			"--provider=openshift",
			"--openshift-service-account=" + serviceAccount,
			"--upstream=http://localhost:" + servicePort,
			"--tls-cert=/etc/tls/proxy-certs/tls.crt",
			"--tls-key=/etc/tls/proxy-certs/tls.key",
			"--cookie-secret=SECRET",
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: types.FlowCollectorDefaultServicePort,
			},
			{
				Name:          "https",
				ContainerPort: types.ConsoleOpenShiftOauthServiceTargetPort,
			},
		},
	}
}

func ConfigSyncContainer() *corev1.Container {
	return &corev1.Container{
		Image:           images.GetConfigSyncImageName(),
		ImagePullPolicy: kube.GetPullPolicy(images.GetConfigSyncImagePullPolicy()),
		Name:            "config-sync",
	}
}

func (cli *VanClient) getControllerRules(options types.SiteConfigSpec) []rbacv1.PolicyRule {
	return cli.adjustRules(options, types.ControllerPolicyRule)
}

func (cli *VanClient) adjustRules(options types.SiteConfigSpec, original []rbacv1.PolicyRule) []rbacv1.PolicyRule {
	// remove rule for routes or DeploymentConfigs if they are not defined
	var apigroups []string
	if cli.RouteClient == nil {
		apigroups = append(apigroups, "route.openshift.io")
	} else if options.IsIngressRoute() && options.IngressHost != "" {
		original = append(original, types.ControllerRoutesCustomHostPolicyRule...)
	}
	if cli.OCAppsClient == nil {
		apigroups = append(apigroups, "apps.openshift.io")
	}
	return removeRules(original, apigroups)
}

func removeRules(original []rbacv1.PolicyRule, apigroups []string) []rbacv1.PolicyRule {
	// remove rules for particular apigroups
	var rules []rbacv1.PolicyRule
	for _, rule := range original {
		if len(rule.APIGroups) == 1 && utils.StringSliceContains(apigroups, rule.APIGroups[0]) {
			continue
		}

		rules = append(rules, rule)
	}
	return rules
}

func (cli *VanClient) GetVanPrometheusServerSpec(options types.SiteConfigSpec, van *types.RouterSpec) {
	// prometheus-server container index
	const (
		prometheusServer = iota
	)

	van.PrometheusServer.Image = images.GetPrometheusServerImageDetails()
	van.PrometheusServer.Replicas = 1
	van.PrometheusServer.LabelSelector = map[string]string{
		types.ComponentAnnotation: types.PrometheusComponentName,
	}
	van.PrometheusServer.Labels = map[string]string{
		types.AppLabel:    types.PrometheusDeploymentName,
		types.PartOfLabel: types.AppName,
	}
	for key, value := range van.PrometheusServer.LabelSelector {
		van.PrometheusServer.Labels[key] = value
	}
	van.PrometheusServer.Annotations = map[string]string{}
	for key, value := range options.Annotations {
		van.PrometheusServer.Annotations[key] = value
	}
	for key, value := range options.PrometheusServer.PodAnnotations {
		van.PrometheusServer.Annotations[key] = value
	}
	for key, value := range options.Labels {
		van.PrometheusServer.Labels[key] = value
	}

	envVars := []corev1.EnvVar{}

	sidecars := []*corev1.Container{}
	volumes := []corev1.Volume{}
	mounts := make([][]corev1.VolumeMount, 1)
	err := configureDeployment(&van.PrometheusServer, &options.PrometheusServer.Tuning)
	if err != nil {
		fmt.Println("Error configuring prometheus server deployment:", err)
	}
	kube.AppendConfigVolume(&volumes, &mounts[prometheusServer], "prometheus-config", "prometheus-server-config", "/etc/prometheus")
	kube.AppendSharedVolume(&volumes, []*[]corev1.VolumeMount{&mounts[prometheusServer]}, "prometheus-storage-volume", "/prometheus")

	van.PrometheusServer.EnvVar = envVars
	van.PrometheusServer.Volumes = volumes
	van.PrometheusServer.VolumeMounts = mounts
	van.PrometheusServer.Sidecars = sidecars

	serviceAccounts := []*corev1.ServiceAccount{}
	annotation := map[string]string{}
	serviceAccounts = append(serviceAccounts, &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        types.PrometheusServiceAccountName,
			Annotations: annotation,
			Labels:      options.Labels,
		},
	})
	van.PrometheusServer.ServiceAccounts = serviceAccounts

	van.PrometheusCredentials = []types.Credential{}

	roles := []*rbacv1.Role{}
	roles = append(roles, &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   types.PrometheusRoleName,
			Labels: options.Labels,
		},
		Rules: cli.getControllerRules(options),
	})
	van.PrometheusServer.Roles = roles

	roleBindings := []*rbacv1.RoleBinding{}
	roleBindings = append(roleBindings, &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   types.PrometheusRoleBindingName,
			Labels: options.Labels,
		},
		Subjects: []rbacv1.Subject{{
			Kind: "ServiceAccount",
			Name: types.PrometheusServiceAccountName,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: types.PrometheusRoleName,
		},
	})
	van.PrometheusServer.RoleBindings = roleBindings

	svctype := corev1.ServiceTypeClusterIP
	annotations := map[string]string{}
	prometheusPorts := []corev1.ServicePort{}
	prometheusPort := corev1.ServicePort{
		Name:       "prometheus",
		Protocol:   "TCP",
		Port:       types.PrometheusServerDefaultServicePort,
		TargetPort: intstr.FromInt(int(types.PrometheusServerDefaultServiceTargetPort)),
	}

	prometheusPorts = append(prometheusPorts, prometheusPort)
	svcs := []*corev1.Service{}
	if len(prometheusPorts) > 0 {
		svc := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        types.PrometheusServiceName,
				Annotations: annotations,
				Labels:      options.Labels,
			},
			Spec: corev1.ServiceSpec{
				Selector: van.PrometheusServer.Labels,
				Ports:    prometheusPorts,
				Type:     svctype,
			},
		}

		if options.Controller.LoadBalancerIp != "" && svctype == corev1.ServiceTypeLoadBalancer {
			svc.Spec.LoadBalancerIP = options.Controller.LoadBalancerIp
		}

		svcs = append(svcs, svc)
		for _, p := range prometheusPorts {
			van.PrometheusServer.Ports = append(van.PrometheusServer.Ports, corev1.ContainerPort{
				Name:          p.Name,
				ContainerPort: p.Port,
			})
		}
	}
	van.PrometheusServer.Services = svcs
}

func (cli *VanClient) GetVanControllerSpec(options types.SiteConfigSpec, van *types.RouterSpec, transport *appsv1.Deployment, siteId string) {
	// service-controller container index
	const (
		serviceController = iota
		flowCollector
		oauthProxy
	)

	van.Controller.Image = images.GetServiceControllerImageDetails()
	van.Controller.Replicas = 1
	van.Controller.LabelSelector = map[string]string{
		types.ComponentAnnotation: types.ControllerComponentName,
	}
	van.Controller.Labels = map[string]string{
		types.AppLabel:    types.ControllerDeploymentName,
		types.PartOfLabel: types.AppName,
	}
	for key, value := range van.Controller.LabelSelector {
		van.Controller.Labels[key] = value
	}
	van.Controller.Annotations = map[string]string{}
	for key, value := range options.Annotations {
		van.Controller.Annotations[key] = value
	}
	for key, value := range options.Controller.PodAnnotations {
		van.Controller.Annotations[key] = value
	}
	for key, value := range options.Labels {
		van.Controller.Labels[key] = value
	}
	runAsNonRoot := true
	van.Controller.SecurityContext = &corev1.SecurityContext{
		RunAsNonRoot: &runAsNonRoot,
	}
	if options.RunAsUser > 0 {
		van.Controller.SecurityContext.RunAsUser = &options.RunAsUser
	}
	if options.RunAsGroup > 0 {
		van.Controller.SecurityContext.RunAsGroup = &options.RunAsGroup
	}

	envVars := []corev1.EnvVar{}
	envVars = append(envVars, corev1.EnvVar{Name: "SKUPPER_NAMESPACE", Value: van.Namespace})
	envVars = append(envVars, corev1.EnvVar{Name: "SKUPPER_SITE_NAME", Value: van.Name})
	envVars = append(envVars, corev1.EnvVar{Name: "SKUPPER_SITE_ID", Value: siteId})
	envVars = append(envVars, corev1.EnvVar{Name: "SKUPPER_SERVICE_ACCOUNT", Value: types.TransportServiceAccountName})
	envVars = append(envVars, corev1.EnvVar{Name: "SKUPPER_ROUTER_MODE", Value: options.RouterMode})
	envVars = append(envVars, corev1.EnvVar{Name: "OWNER_NAME", Value: transport.ObjectMeta.Name})
	envVars = append(envVars, corev1.EnvVar{Name: "OWNER_UID", Value: string(transport.ObjectMeta.UID)})
	envVars = images.AddRouterImageOverrideToEnv(envVars)
	if !options.EnableServiceSync {
		envVars = append(envVars, corev1.EnvVar{Name: "SKUPPER_DISABLE_SERVICE_SYNC", Value: "true"})
	}

	sidecars := []*corev1.Container{}
	volumes := []corev1.Volume{}
	mounts := make([][]corev1.VolumeMount, 1)

	consoleUsersMounted := false
	if options.EnableFlowCollector {
		mounts = append(mounts, []corev1.VolumeMount{})
		if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
			csp := strconv.Itoa(int(types.ConsoleOpenShiftServicePort))
			envVars = append(envVars, corev1.EnvVar{Name: "FLOW_PORT", Value: csp})
			envVars = append(envVars, corev1.EnvVar{Name: "FLOW_HOST", Value: "localhost"})
			mounts = append(mounts, []corev1.VolumeMount{})
			kube.AppendSecretVolume(&volumes, &mounts[oauthProxy], types.ConsoleServerSecret, "/etc/tls/proxy-certs/")
		} else if options.AuthMode == string(types.ConsoleAuthModeInternal) {
			envVars = append(envVars, corev1.EnvVar{Name: "FLOW_USERS", Value: "/etc/console-users"})
			kube.AppendSharedSecretVolume(&volumes, []*[]corev1.VolumeMount{&mounts[serviceController], &mounts[flowCollector]}, "skupper-console-users", "/etc/console-users/")
			consoleUsersMounted = true
		}
		err := configureDeployment(&van.Collector, &options.FlowCollector.Tuning)
		if err != nil {
			fmt.Println("Error configuring flow collector sidecar:", err)
		}
		van.Collector.Image = images.GetFlowCollectorImageDetails()
		van.Collector.EnvVar = envVars
		sidecars = append(sidecars, kube.ContainerForFlowCollector(van.Collector))
		if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
			csp := strconv.Itoa(int(types.ConsoleOpenShiftServicePort))
			sidecars = append(sidecars, OauthProxyContainer(types.ControllerServiceAccountName, csp))
		}
	}
	if options.AuthMode != string(types.ConsoleAuthModeOpenshift) {
		if options.EnableFlowCollector && options.EnableRestAPI {
			kube.AppendSharedSecretVolume(&volumes, []*[]corev1.VolumeMount{&mounts[serviceController], &mounts[flowCollector]}, types.ConsoleServerSecret, "/etc/service-controller/console/")
		} else if options.EnableFlowCollector {
			kube.AppendSecretVolume(&volumes, &mounts[flowCollector], types.ConsoleServerSecret, "/etc/service-controller/console/")
		} else if options.EnableRestAPI {
			kube.AppendSecretVolume(&volumes, &mounts[serviceController], types.ConsoleServerSecret, "/etc/service-controller/console/")
		}
	}
	if options.EnableRestAPI {
		if options.AuthMode == string(types.ConsoleAuthModeInternal) {
			envVars = append(envVars, corev1.EnvVar{Name: "METRICS_USERS", Value: "/etc/console-users"})
			if !consoleUsersMounted {
				kube.AppendSecretVolume(&volumes, &mounts[serviceController], "skupper-console-users", "/etc/console-users/")
			}
		}
	}
	localClientMounts := []*[]corev1.VolumeMount{}
	localClientMounts = append(localClientMounts, &mounts[serviceController])
	if options.EnableFlowCollector {
		localClientMounts = append(localClientMounts, &mounts[flowCollector])
	}
	kube.AppendSharedSecretVolume(&volumes, localClientMounts, types.LocalClientSecret, "/etc/messaging/")
	van.Controller.EnvVar = envVars
	van.Controller.Volumes = volumes
	van.Controller.VolumeMounts = mounts
	van.Controller.Sidecars = sidecars

	serviceAccounts := []*corev1.ServiceAccount{}
	annotation := map[string]string{}
	if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
		annotation = map[string]string{
			"serviceaccounts.openshift.io/oauth-redirectreference.primary": "{\"kind\":\"OAuthRedirectReference\",\"apiVersion\":\"v1\",\"reference\":{\"kind\":\"Route\",\"name\":\"" + types.ConsoleRouteName + "\"}}",
		}
	}
	serviceAccounts = append(serviceAccounts, &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        types.ControllerServiceAccountName,
			Annotations: annotation,
			Labels:      options.Labels,
		},
	})
	van.Controller.ServiceAccounts = serviceAccounts

	roles := []*rbacv1.Role{}
	roles = append(roles, &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   types.ControllerRoleName,
			Labels: options.Labels,
		},
		Rules: cli.getControllerRules(options),
	})
	van.Controller.Roles = roles

	roleBindings := []*rbacv1.RoleBinding{}
	roleBindings = append(roleBindings, &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   types.ControllerRoleBindingName,
			Labels: options.Labels,
		},
		Subjects: []rbacv1.Subject{{
			Kind: "ServiceAccount",
			Name: types.ControllerServiceAccountName,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: types.ControllerRoleName,
		},
	})
	van.Controller.RoleBindings = roleBindings

	van.Controller.ClusterRoles = cli.ClusterRoles(options.EnableClusterPermissions)
	van.Controller.ClusterRoleBindings = ClusterRoleBindings(van.Namespace, options.EnableClusterPermissions)

	svctype := corev1.ServiceTypeClusterIP
	if options.IsConsoleIngressLoadBalancer() {
		svctype = corev1.ServiceTypeLoadBalancer
	} else if options.IsConsoleIngressNodePort() {
		svctype = corev1.ServiceTypeNodePort
	}
	annotations := map[string]string{}
	controllerPorts := []corev1.ServicePort{}
	routes := []*routev1.Route{}
	if options.EnableFlowCollector {
		metricsPort := corev1.ServicePort{
			Name:       "metrics",
			Protocol:   "TCP",
			Port:       types.FlowCollectorDefaultServicePort,
			TargetPort: intstr.FromInt(int(types.FlowCollectorDefaultServiceTargetPort)),
		}

		if options.IsConsoleIngressRoute() {
			annotations = getServingSecretAnnotations(types.ConsoleServerSecret)
			if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
				metricsPort = corev1.ServicePort{
					Name:       "metrics",
					Protocol:   "TCP",
					Port:       types.ConsoleOpenShiftOauthServicePort,
					TargetPort: intstr.FromInt(int(types.ConsoleOpenShiftOauthServiceTargetPort)),
				}
			}
			host := options.GetControllerIngressHost()
			if host != "" {
				host = types.ConsoleRouteName + "-" + van.Namespace + "." + host
			}
			routes = append(routes, &routev1.Route{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Route",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   types.ConsoleRouteName,
					Labels: options.Labels,
				},
				Spec: routev1.RouteSpec{
					Path: "",
					Host: host,
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromString("metrics"),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: types.ControllerServiceName,
					},
					TLS: &routev1.TLSConfig{
						Termination:                   routev1.TLSTerminationReencrypt,
						InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
					},
				},
			})
		} else if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
			annotations = getServingSecretAnnotations(types.ConsoleServerSecret)
		} else {
			controllerHosts := []string{types.ControllerServiceName + "." + van.Namespace}
			controllerIngressHost := options.GetControllerIngressHost()
			post := false // indicates whether credentials need to be revised after creating appropriate ingress resources
			if options.IsIngressNginxIngress() {
				if controllerIngressHost != "" {
					controllerHosts = append(controllerHosts, strings.Join([]string{"console", van.Namespace, controllerIngressHost}, "."))
				} else {
					post = true
				}
			} else if options.IsIngressContourHttpProxy() {
				if controllerIngressHost != "" {
					controllerHosts = append(controllerHosts, strings.Join([]string{"console", van.Namespace, controllerIngressHost}, "."))
				}
			} else if options.IsIngressLoadBalancer() {
				post = true
			} else {
				if controllerIngressHost != "" {
					controllerHosts = append(controllerHosts, controllerIngressHost)
				}
			}
			van.ControllerCredentials = append(van.ControllerCredentials, types.Credential{
				CA:          types.LocalCaSecret,
				Name:        types.ConsoleServerSecret,
				Subject:     types.ControllerServiceName,
				Hosts:       controllerHosts,
				ConnectJson: false,
				Post:        post,
				Labels:      options.Labels,
			})
		}
		controllerPorts = append(controllerPorts, metricsPort)
	}
	if options.EnableRestAPI {
		if !options.EnableFlowCollector {
			hosts := []string{types.ControllerServiceName + "." + van.Namespace, types.ControllerServiceName}
			van.ControllerCredentials = append(van.ControllerCredentials, types.Credential{
				CA:          types.LocalCaSecret,
				Name:        types.ConsoleServerSecret,
				Subject:     types.ControllerServiceName,
				Hosts:       hosts,
				ConnectJson: false,
				Post:        false,
				Labels:      options.Labels,
			})
		}
		controllerPorts = append(controllerPorts, corev1.ServicePort{
			Name:       "rest-api",
			Protocol:   "TCP",
			Port:       8080,
			TargetPort: intstr.FromInt(8080),
		})
	}

	for key, value := range options.Controller.ServiceAnnotations {
		annotations[key] = value
	}

	svcs := []*corev1.Service{}
	if len(controllerPorts) > 0 {
		svc := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        types.ControllerServiceName,
				Annotations: annotations,
				Labels:      options.Labels,
			},
			Spec: corev1.ServiceSpec{
				Selector: van.Controller.Labels,
				Ports:    controllerPorts,
				Type:     svctype,
			},
		}

		if options.Controller.LoadBalancerIp != "" && svctype == corev1.ServiceTypeLoadBalancer {
			svc.Spec.LoadBalancerIP = options.Controller.LoadBalancerIp
		}

		svcs = append(svcs, svc)
		for _, p := range controllerPorts {
			van.Controller.Ports = append(van.Controller.Ports, corev1.ContainerPort{
				Name:          p.Name,
				ContainerPort: p.Port,
			})
		}
	}
	van.Controller.Services = svcs

	van.Controller.Routes = routes
}

func ClusterRoleBindings(namespace string, enableClusterPermissions bool) []*rbacv1.ClusterRoleBinding {
	clusterRoleBindings := []*rbacv1.ClusterRoleBinding{}
	clusterRoleBindings = append(clusterRoleBindings, &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", types.ControllerClusterRoleName, namespace),
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      types.ControllerServiceAccountName,
			Namespace: namespace,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: types.ControllerClusterRoleName,
		},
	})
	if enableClusterPermissions {
		clusterRoleBindings = append(clusterRoleBindings, &rbacv1.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "ClusterRoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%s", types.ControllerExtendedClusterRoleName, namespace),
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      types.ControllerServiceAccountName,
				Namespace: namespace,
			}},
			RoleRef: rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: types.ControllerExtendedClusterRoleName,
			},
		})
	}
	return clusterRoleBindings
}

func (cli *VanClient) ClusterRoles(enablePermissions bool) []*rbacv1.ClusterRole {
	var clusterRoles []*rbacv1.ClusterRole
	clusterRoles = append(clusterRoles, &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: types.ControllerClusterRoleName,
		},
		Rules: types.ClusterControllerPolicyRules,
	})
	if enablePermissions {
		clusterRoles = append(clusterRoles, &rbacv1.ClusterRole{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "ClusterRole",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: types.ControllerExtendedClusterRoleName,
			},
			Rules: types.ClusterControllerExtendedPolicyRules,
		})
	}
	return clusterRoles
}

func configureDeployment(spec *types.DeploymentSpec, options *types.Tuning) error {
	errs := []string{}
	if options.Affinity != "" {
		spec.Affinity = utils.LabelToMap(options.Affinity)
	}
	if options.AntiAffinity != "" {
		spec.AntiAffinity = utils.LabelToMap(options.AntiAffinity)
	}
	if options.NodeSelector != "" {
		spec.NodeSelector = utils.LabelToMap(options.NodeSelector)
	}
	if options.Cpu != "" {
		cpu, err := resource.ParseQuantity(options.Cpu)
		if err == nil {
			spec.CpuRequest = &cpu
		} else {
			errs = append(errs, fmt.Sprintf("Invalid value for cpu: %s", err))
		}
	}
	if options.Memory != "" {
		memory, err := resource.ParseQuantity(options.Memory)
		if err == nil {
			spec.MemoryRequest = &memory
		} else {
			errs = append(errs, fmt.Sprintf("Invalid value for memory: %s", err))
		}
	}
	if options.CpuLimit != "" {
		cpu, err := resource.ParseQuantity(options.CpuLimit)
		if err == nil {
			spec.CpuLimit = &cpu
		} else {
			errs = append(errs, fmt.Sprintf("Invalid value for cpu: %s", err))
		}
	}
	if options.MemoryLimit != "" {
		memory, err := resource.ParseQuantity(options.MemoryLimit)
		if err == nil {
			spec.MemoryLimit = &memory
		} else {
			errs = append(errs, fmt.Sprintf("Invalid value for memory: %s", err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, ", "))
	} else {
		return nil
	}
}

func (cli *VanClient) GetRouterSpecFromOpts(options types.SiteConfigSpec, siteId string) *types.RouterSpec {
	// skupper-router container index
	// TODO: update after dataplance changes
	const (
		qdrouterd = iota
		configSync
		oauthProxy
	)

	van := &types.RouterSpec{}
	// todo: think through van name, router name, secret names, etc.
	if options.SkupperNamespace == "" {
		van.Namespace = cli.Namespace
	} else {
		van.Namespace = options.SkupperNamespace
	}
	if options.SkupperName == "" {
		van.Name = van.Namespace
	} else {
		van.Name = options.SkupperName
	}

	van.AuthMode = types.ConsoleAuthMode(options.AuthMode)

	van.Transport.Image = images.GetRouterImageDetails()
	van.Transport.Replicas = 1
	if options.Routers != 0 {
		van.Transport.Replicas = int32(options.Routers)
	}
	van.Transport.LabelSelector = map[string]string{
		types.ComponentAnnotation: types.TransportComponentName,
	}
	van.Transport.Labels = map[string]string{
		types.PartOfLabel: types.AppName,
		types.AppLabel:    types.TransportDeploymentName,
		"application":     types.TransportDeploymentName, // needed by automeshing in image
	}
	for key, value := range van.Transport.LabelSelector {
		van.Transport.Labels[key] = value
	}
	for key, value := range options.Labels {
		van.Transport.Labels[key] = value
	}
	van.Transport.Annotations = types.TransportPrometheusAnnotations
	for key, value := range options.Annotations {
		van.Transport.Annotations[key] = value
	}
	for key, value := range options.Router.PodAnnotations {
		van.Transport.Annotations[key] = value
	}
	runAsNonRoot := true
	van.Transport.SecurityContext = &corev1.SecurityContext{
		RunAsNonRoot: &runAsNonRoot,
	}
	if options.RunAsUser > 0 {
		van.Transport.SecurityContext.RunAsUser = &options.RunAsUser
	}
	if options.RunAsGroup > 0 {
		van.Transport.SecurityContext.RunAsGroup = &options.RunAsGroup
	}
	err := configureDeployment(&van.Transport, &options.Router.Tuning)
	if err != nil {
		fmt.Println("Error configuring router:", err)
	}

	isEdge := options.IsEdge()
	routerConfig := qdr.InitialConfigSkupperRouter(van.Name+"-${HOSTNAME}", siteId, version.Version, isEdge, 3, options.Router)
	van.RouterConfig, _ = qdr.MarshalRouterConfig(routerConfig)

	envVars := []corev1.EnvVar{}
	if !isEdge {
		envVars = append(envVars, corev1.EnvVar{Name: "APPLICATION_NAME", Value: types.TransportDeploymentName})
		envVars = append(envVars, corev1.EnvVar{Name: "POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
		})
		envVars = append(envVars, corev1.EnvVar{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "status.podIP",
			},
		},
		})
		envVars = append(envVars, corev1.EnvVar{Name: "QDROUTERD_AUTO_MESH_DISCOVERY", Value: "QUERY"})
	}
	envVars = append(envVars, corev1.EnvVar{Name: "QDROUTERD_CONF", Value: "/etc/skupper-router/config/" + types.TransportConfigFile})
	envVars = append(envVars, corev1.EnvVar{Name: "QDROUTERD_CONF_TYPE", Value: "json"})
	envVars = append(envVars, corev1.EnvVar{
		Name:  "SKUPPER_SITE_ID",
		Value: siteId,
	})
	van.Transport.EnvVar = envVars

	ports := []corev1.ContainerPort{}
	ports = append(ports, corev1.ContainerPort{
		Name:          "amqps",
		ContainerPort: types.AmqpsDefaultPort,
	})
	ports = append(ports, corev1.ContainerPort{
		Name:          "http",
		ContainerPort: types.TransportLivenessPort,
	})
	if !isEdge {
		ports = append(ports, corev1.ContainerPort{
			Name:          types.InterRouterRole,
			ContainerPort: types.InterRouterListenerPort,
		})
		ports = append(ports, corev1.ContainerPort{
			Name:          types.EdgeRole,
			ContainerPort: types.EdgeListenerPort,
		})
	}
	van.Transport.Ports = ports

	err = configureDeployment(&van.ConfigSync, &options.ConfigSync.Tuning)
	if err != nil {
		fmt.Println("Error configuring config-sync sidecar:", err)
	}

	van.ConfigSync.Image = images.GetConfigSyncImageDetails()

	sidecars := []*corev1.Container{
		kube.ContainerForConfigSync(van.ConfigSync),
	}

	volumes := []corev1.Volume{}
	mounts := make([][]corev1.VolumeMount, 2)
	kube.AppendSecretVolume(&volumes, &mounts[qdrouterd], types.LocalServerSecret, "/etc/skupper-router-certs/skupper-amqps/")
	kube.AppendConfigVolume(&volumes, &mounts[qdrouterd], "router-config", types.TransportConfigMapName, "/etc/skupper-router/config/")
	if !isEdge {
		kube.AppendSecretVolume(&volumes, &mounts[qdrouterd], types.SiteServerSecret, "/etc/skupper-router-certs/skupper-internal/")
		kube.AppendSecretVolumeWithVolumeName(&volumes, &mounts[configSync], types.SiteServerSecret, "claims-cert", "/etc/skupper-internal/")
	}

	kube.AppendSharedVolume(&volumes, []*[]corev1.VolumeMount{&mounts[qdrouterd], &mounts[configSync]}, "skupper-router-certs", "/etc/skupper-router-certs")

	van.Transport.Volumes = volumes
	van.Transport.VolumeMounts = mounts
	van.Transport.Sidecars = sidecars

	roles := []*rbacv1.Role{}
	roles = append(roles, &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   types.TransportRoleName,
			Labels: options.Labels,
		},
		Rules: cli.adjustRules(options, types.TransportPolicyRule),
	})
	van.Transport.Roles = roles

	roleBindings := []*rbacv1.RoleBinding{}
	roleBindings = append(roleBindings, &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   types.TransportRoleBindingName,
			Labels: options.Labels,
		},
		Subjects: []rbacv1.Subject{{
			Kind: "ServiceAccount",
			Name: types.TransportServiceAccountName,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: types.TransportRoleName,
		},
	})
	van.Transport.RoleBindings = roleBindings

	serviceAccounts := []*corev1.ServiceAccount{}
	annotation := map[string]string{}
	if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
		annotation = map[string]string{
			"serviceaccounts.openshift.io/oauth-redirectreference.primary": "{\"kind\":\"OAuthRedirectReference\",\"apiVersion\":\"v1\",\"reference\":{\"kind\":\"Route\",\"name\":\"skupper-router-console\"}}",
		}
	}
	serviceAccounts = append(serviceAccounts, &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        types.TransportServiceAccountName,
			Annotations: annotation,
			Labels:      options.Labels,
		},
	})
	van.Transport.ServiceAccounts = serviceAccounts

	cas := []types.CertAuthority{}
	cas = append(cas, types.CertAuthority{
		Name:   types.LocalCaSecret,
		Labels: options.Labels,
	})
	if !isEdge {
		cas = append(cas, types.CertAuthority{
			Name:   types.SiteCaSecret,
			Labels: options.Labels,
		})
	}

	cas = append(cas, types.CertAuthority{
		Name:   types.ServiceCaSecret,
		Labels: options.Labels,
	})

	van.CertAuthoritys = cas

	credentials := []types.Credential{}
	credentials = append(credentials, types.Credential{
		CA:          types.LocalCaSecret,
		Name:        types.LocalServerSecret,
		Subject:     types.LocalTransportServiceName,
		Hosts:       []string{types.LocalTransportServiceName, types.LocalTransportServiceName + "." + van.Namespace + ".svc.cluster.local"},
		ConnectJson: false,
		Post:        false,
		Labels:      options.Labels,
	})
	credentials = append(credentials, types.Credential{
		CA:          types.LocalCaSecret,
		Name:        types.LocalClientSecret,
		Subject:     types.LocalTransportServiceName,
		Hosts:       []string{},
		ConnectJson: true,
		Post:        false,
		Labels:      options.Labels,
	})

	credentials = append(credentials, types.Credential{
		CA:          types.ServiceCaSecret,
		Name:        types.ServiceClientSecret,
		Hosts:       []string{},
		ConnectJson: false,
		Post:        false,
		Simple:      true,
		Labels:      options.Labels,
	})

	if !isEdge {
		credentials = append(credentials, types.Credential{
			CA:          types.SiteCaSecret,
			Name:        types.SiteServerSecret,
			Subject:     types.TransportServiceName,
			Hosts:       []string{types.TransportServiceName + "." + van.Namespace, types.TransportServiceName + "." + van.Namespace + ".svc.cluster.local"},
			ConnectJson: false,
			Post:        true,
			Labels:      options.Labels,
		})
	}
	if options.AuthMode == string(types.ConsoleAuthModeInternal) && (options.EnableFlowCollector || options.EnableRestAPI) {
		userData := map[string][]byte{}
		if options.User != "" {
			userData[options.User] = []byte(options.Password)
		}
		credentials = append(credentials, types.Credential{
			CA:          "",
			Name:        "skupper-console-users",
			Subject:     "",
			ConnectJson: false,
			Data:        userData,
			Post:        false,
			Labels:      options.Labels,
		})
	}
	van.TransportCredentials = credentials

	// TODO: this is a hack for ports, fix this
	svcs := []*corev1.Service{}
	svcs = append(svcs, &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        types.LocalTransportServiceName,
			Annotations: map[string]string{},
			Labels:      options.Labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: van.Transport.Labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "amqps",
					Protocol:   "TCP",
					Port:       types.AmqpsDefaultPort,
					TargetPort: intstr.FromInt(int(types.AmqpsDefaultPort)),
				},
			},
			Type: "",
		},
	})
	if !isEdge {
		svcType := corev1.ServiceTypeClusterIP
		if options.IsIngressLoadBalancer() {
			svcType = corev1.ServiceTypeLoadBalancer
		} else if options.IsIngressNodePort() {
			svcType = corev1.ServiceTypeNodePort
		}

		svc := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        types.TransportServiceName,
				Annotations: options.Router.ServiceAnnotations,
				Labels:      options.Labels,
			},
			Spec: corev1.ServiceSpec{
				Selector: van.Transport.Labels,
				Ports: []corev1.ServicePort{
					{
						Name:       types.InterRouterRole,
						Protocol:   "TCP",
						Port:       types.InterRouterListenerPort,
						TargetPort: intstr.FromInt(int(types.InterRouterListenerPort)),
					},
					{
						Name:       types.EdgeRole,
						Protocol:   "TCP",
						Port:       types.EdgeListenerPort,
						TargetPort: intstr.FromInt(int(types.EdgeListenerPort)),
					},
					{
						Name:       types.ClaimRedemptionPortName,
						Protocol:   "TCP",
						Port:       types.ClaimRedemptionPort,
						TargetPort: intstr.FromInt(int(types.ClaimRedemptionPort)),
					},
				},
				Type: svcType,
			},
		}

		if options.Router.LoadBalancerIp != "" && svcType == corev1.ServiceTypeLoadBalancer {
			svc.Spec.LoadBalancerIP = options.Router.LoadBalancerIp
		}

		svcs = append(svcs, svc)
	}
	van.Transport.Services = svcs

	routes := []*routev1.Route{}
	if !isEdge && options.IsIngressRoute() {
		hostInterRouter := ""
		hostEdge := ""
		hostClaims := ""
		if host := options.GetRouterIngressHost(); host != "" {
			hostInterRouter = types.InterRouterRouteName + "-" + van.Namespace + "." + host
			hostEdge = types.EdgeRouteName + "-" + van.Namespace + "." + host
			hostClaims = types.ClaimRedemptionRouteName + "-" + van.Namespace + "." + host
		}
		routes = append(routes, &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Route",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        types.InterRouterRouteName,
				Annotations: options.IngressAnnotations,
				Labels:      options.Labels,
			},
			Spec: routev1.RouteSpec{
				Path: "",
				Host: hostInterRouter,
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromString(types.InterRouterRole),
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: types.TransportServiceName,
				},
				TLS: &routev1.TLSConfig{
					Termination:                   routev1.TLSTerminationPassthrough,
					InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
				},
			},
		})
		routes = append(routes, &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Route",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        types.EdgeRouteName,
				Annotations: options.IngressAnnotations,
				Labels:      options.Labels,
			},
			Spec: routev1.RouteSpec{
				Path: "",
				Host: hostEdge,
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromString(types.EdgeRole),
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: types.TransportServiceName,
				},
				TLS: &routev1.TLSConfig{
					Termination:                   routev1.TLSTerminationPassthrough,
					InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
				},
			},
		})
		routes = append(routes, &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Route",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        types.ClaimRedemptionRouteName,
				Annotations: options.IngressAnnotations,
				Labels:      options.Labels,
			},
			Spec: routev1.RouteSpec{
				Path: "",
				Host: hostClaims,
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromString(types.ClaimRedemptionPortName),
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: types.TransportServiceName,
				},
				TLS: &routev1.TLSConfig{
					Termination:                   routev1.TLSTerminationPassthrough,
					InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				},
			},
		})
	}
	van.Transport.Routes = routes

	return van
}

func (cli *VanClient) GetRouterHostAliasesSpecFromTokens(ctx context.Context, namespace string) ([]corev1.HostAlias, error) {
	hostAliasesMap := make(map[string]map[string]bool)
	secrets, err := cli.KubeClient.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: types.SkupperTypeQualifier + "=" + types.TypeToken})
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve connection token: %w", err)
	}
	if len(secrets.Items) == 0 {
		return nil, nil
	} else {
		for _, s := range secrets.Items {
			if alias, ok := s.ObjectMeta.Annotations["edge-alias"]; ok {
				if net.ParseIP(alias) == nil {
					log.Printf("Skipping edge-alias: %s is not a valid textual representation of an IP address\n", alias)
				} else {
					name := s.ObjectMeta.Annotations["edge-host"]
					if msgs := validation.IsDNS1123Subdomain(name); len(msgs) != 0 {
						log.Printf("Skipping edge-alias: %v\n", msgs)
					} else {
						if _, ok := hostAliasesMap[alias][name]; !ok {
							hostAliasesMap[alias] = make(map[string]bool)
							hostAliasesMap[alias][name] = true
						}
					}
				}
			}
			if alias, ok := s.ObjectMeta.Annotations["inter-router-alias"]; ok {
				if net.ParseIP(alias) == nil {
					log.Printf("Skipping inter-router-alias: %s is not a valid textual representation of an IP address", alias)
				} else {
					name := s.ObjectMeta.Annotations["inter-router-host"]
					if msgs := validation.IsDNS1123Subdomain(name); len(msgs) != 0 {
						log.Printf("Skipping inter-router-alias: %v\n", msgs)
					} else {
						if _, ok := hostAliasesMap[alias][name]; !ok {
							hostAliasesMap[alias] = make(map[string]bool)
							hostAliasesMap[alias][name] = true
						}
					}
				}
			}
		}
	}
	hostAliases := []corev1.HostAlias{}
	for alias, v := range hostAliasesMap {
		new := corev1.HostAlias{
			IP:        alias,
			Hostnames: []string{},
		}
		for hn, _ := range v {
			new.Hostnames = append(new.Hostnames, hn)
		}
		hostAliases = append(hostAliases, new)
	}

	return hostAliases, nil
}

// RouterCreate instantiates a VAN (router and controller) deployment
func (cli *VanClient) RouterCreate(ctx context.Context, options types.SiteConfig) error {
	// todo return error
	if options.Spec.IsIngressRoute() && cli.RouteClient == nil {
		return fmt.Errorf("OpenShift cluster not detected for --ingress type route")
	}

	if options.Spec.EnableFlowCollector || options.Spec.EnableRestAPI {
		if options.Spec.AuthMode == string(types.ConsoleAuthModeInternal) || options.Spec.AuthMode == "" {
			options.Spec.AuthMode = string(types.ConsoleAuthModeInternal)
			if options.Spec.User == "" {
				options.Spec.User = "admin"
			}
			if options.Spec.Password == "" {
				options.Spec.Password = utils.RandomId(10)
			}
		} else {
			if options.Spec.User != "" {
				fmt.Println("--router-console-user only valid when --router-console-auth=internal")
			}
			if options.Spec.Password != "" {
				fmt.Println("--router-console-password only valid when --router-console-auth=internal")
			}
		}
	}

	siteId := options.Reference.UID
	if siteId == "" {
		siteId = utils.RandomId(10)
	}
	van := cli.GetRouterSpecFromOpts(options.Spec, siteId)
	siteOwnerRef := asOwnerReference(options.Reference)
	var ownerRefs []metav1.OwnerReference
	if siteOwnerRef != nil {
		ownerRefs = []metav1.OwnerReference{*siteOwnerRef}
	}
	var err error
	hostAliases, err := cli.GetRouterHostAliasesSpecFromTokens(ctx, cli.GetNamespace())
	if err != nil {
		return err
	}
	van.Transport.HostAliases = hostAliases
	if options.Spec.AuthMode == string(types.ConsoleAuthModeInternal) {
		config := `
pwcheck_method: auxprop
auxprop_plugin: sasldb
sasldb_path: /tmp/skrouterd.sasldb
`
		saslData := &map[string]string{
			"skrouterd.conf": config,
		}
		kube.NewConfigMap("skupper-sasl-config", saslData, &options.Spec.Labels, nil, siteOwnerRef, van.Namespace, cli.KubeClient)
	}
	for _, sa := range van.Transport.ServiceAccounts {
		sa.ObjectMeta.OwnerReferences = ownerRefs
		_, err = kube.CreateServiceAccount(van.Namespace, sa, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	for _, role := range van.Transport.Roles {
		role.ObjectMeta.OwnerReferences = ownerRefs
		_, err = kube.CreateRole(van.Namespace, role, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	for _, roleBinding := range van.Transport.RoleBindings {
		roleBinding.ObjectMeta.OwnerReferences = ownerRefs
		_, err = kube.CreateRoleBinding(van.Namespace, roleBinding, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	for _, ca := range van.CertAuthoritys {
		_, err = kube.NewCertAuthority(ca, siteOwnerRef, van.Namespace, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	for _, cred := range van.TransportCredentials {
		if !cred.Post {
			_, err = kube.NewSecret(cred, siteOwnerRef, van.Namespace, cli.KubeClient)
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		}
	}
	for _, svc := range van.Transport.Services {
		svc.ObjectMeta.OwnerReferences = ownerRefs
		_, err = kube.CreateService(svc, van.Namespace, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	if options.Spec.IsIngressRoute() {
		for _, rte := range van.Transport.Routes {
			rte.ObjectMeta.OwnerReferences = ownerRefs
			_, err = kube.CreateRoute(rte, van.Namespace, cli.RouteClient)
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		}
	}

	dep, err := kube.NewTransportDeployment(van, siteOwnerRef, cli.KubeClient)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	kube.NewConfigMap(types.ServiceInterfaceConfigMap, nil, &options.Spec.Labels, nil, siteOwnerRef, van.Namespace, cli.KubeClient)
	initialConfig := qdr.AsConfigMapData(van.RouterConfig)
	kube.NewConfigMap(types.TransportConfigMapName, &initialConfig, &options.Spec.Labels, nil, siteOwnerRef, van.Namespace, cli.KubeClient)

	currentContext, cn := getCurrentContextOrDefault(ctx)
	if cn != nil {
		defer cn()
	}

	if options.Spec.RouterMode == string(types.TransportModeInterior) {
		if options.Spec.IsIngressNginxIngress() || options.Spec.IsIngressKubernetes() {
			err = cli.createIngress(options)
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		} else if options.Spec.IsIngressContourHttpProxy() {
			err = cli.createContourProxies(options)
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		}
		for _, cred := range van.TransportCredentials {
			if cred.Post {
				deadlineExceeded := false
				rslvr, err := resolver.NewResolver(cli, van.Namespace, &options.Spec)
				if err != nil {
					return err
				}
				hosts, err := rslvr.GetAllHosts()
				if err != nil {
					return err
				}

				var spin *spinner.Spinner
				if len(hosts) == 0 {
					waitingFor := "Waiting to try and resolve SANs for router..."

					if options.Spec.IsIngressLoadBalancer() {
						waitingFor = "Waiting for LoadBalancer IP or hostname..."
					}

					spin = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
					spin.Prefix = waitingFor
					spin.FinalMSG = waitingFor + "\n"
				}

				for len(hosts) == 0 && !deadlineExceeded {
					spin.Start()
					select {
					case <-ctx.Done():
						fmt.Println("context deadline exceeded")
						deadlineExceeded = true
						break
					default:
						time.Sleep(time.Second)
						hosts, err = rslvr.GetAllHosts()
						if err != nil {
							return err
						}
					}
				}

				if spin != nil {
					spin.Stop()
				}

				if len(hosts) == 0 {
					if options.Spec.IsIngressLoadBalancer() {
						return fmt.Errorf("Failed to get LoadBalancer IP or Hostname for service %s", types.TransportServiceName)
					} else {
						return fmt.Errorf("Failed to resolve SANs for %s", cred.Name)
					}
				} else {
					for _, host := range hosts {
						cred.Hosts = append(cred.Hosts, host)
						if len(host) < 64 {
							cred.Subject = host
						}
					}
				}
				kube.NewSecret(cred, siteOwnerRef, van.Namespace, cli.KubeClient)
			}
		}
	}

	if options.Spec.EnableController {
		cli.GetVanControllerSpec(options.Spec, van, dep, siteId)
		err := configureDeployment(&van.Controller, &options.Spec.Controller.Tuning)
		if err != nil {
			fmt.Println("Error configuring controller:", err)
		}
		for _, sa := range van.Controller.ServiceAccounts {
			sa.ObjectMeta.OwnerReferences = ownerRefs
			_, err = kube.CreateServiceAccount(van.Namespace, sa, cli.KubeClient)
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		}
		for _, role := range van.Controller.Roles {
			role.ObjectMeta.OwnerReferences = ownerRefs
			_, err = kube.CreateRole(van.Namespace, role, cli.KubeClient)
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		}
		for _, roleBinding := range van.Controller.RoleBindings {
			roleBinding.ObjectMeta.OwnerReferences = ownerRefs
			_, err = kube.CreateRoleBinding(van.Namespace, roleBinding, cli.KubeClient)
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		}
		policyValidator := NewClusterPolicyValidator(cli)
		for _, clusterRole := range van.Controller.ClusterRoles {
			// optional (in case of failure, cluster admin can add necessary cluster roles manually)
			kube.CreateClusterRole(clusterRole, cli.KubeClient)
		}
		for _, clusterRoleBinding := range van.Controller.ClusterRoleBindings {
			clusterRoleBinding.ObjectMeta.OwnerReferences = ownerRefs
			_, err = kube.CreateClusterRoleBinding(clusterRoleBinding, cli.KubeClient)
			if err != nil && !errors.IsAlreadyExists(err) {
				if policyValidator.Enabled() {
					log.Printf("unable to define cluster role binding - %v", err)
					break
				}
			}
		}
		for _, svc := range van.Controller.Services {
			svc.ObjectMeta.OwnerReferences = ownerRefs
			_, err = kube.CreateService(svc, van.Namespace, cli.KubeClient)
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		}
		if options.Spec.IsConsoleIngressRoute() {
			for _, rte := range van.Controller.Routes {
				rte.ObjectMeta.OwnerReferences = ownerRefs
				_, err = kube.CreateRoute(rte, van.Namespace, cli.RouteClient)
				if err != nil && !errors.IsAlreadyExists(err) {
					return err
				}
			}
		}
		for _, cred := range van.ControllerCredentials {
			if options.Spec.IsConsoleIngressRoute() {
				rte, err := kube.GetRoute(types.ClaimRedemptionRouteName, van.Namespace, cli.RouteClient)
				if err == nil {
					cred.Hosts = append(cred.Hosts, rte.Spec.Host)
				} else {
					log.Printf("Failed to retrieve route %q: %s", types.ClaimRedemptionRouteName, err.Error())
				}
			} else if options.Spec.IsConsoleIngressLoadBalancer() {
				err = cli.appendLoadBalancerHostOrIp(currentContext, types.ControllerServiceName, van.Namespace, &cred)
				if err != nil {
					return err
				}
			} else if options.Spec.IsConsoleIngressNginxIngress() && cred.Post {
				err = cli.appendIngressHost([]string{"claims", "console"}, van.Namespace, &cred)
				if err != nil {
					return err
				}
			}
			_, err = kube.NewSecret(cred, siteOwnerRef, van.Namespace, cli.KubeClient)
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		}
		_, err = kube.NewControllerDeployment(van, siteOwnerRef, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}

	if options.Spec.EnableFlowCollector && options.Spec.PrometheusServer.ExternalServer == "" {
		//	Stand up local prometheus server for metric aggregation
		cli.GetVanPrometheusServerSpec(options.Spec, van)
		err := configureDeployment(&van.PrometheusServer, &options.Spec.Controller.Tuning)
		if err != nil {
			return err
		}
		err = cli.createPrometheus(ctx, &options, *van)
		if err != nil {
			return err
		}
	}

	if options.Spec.CreateNetworkPolicy {
		err = kube.CreateNetworkPolicy(ownerRefs, van.Namespace, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}

	if options.Spec.EnableSkupperEvents {
		err = kube.AddEventRecorderPermissions(van.Namespace, ownerRefs, cli.KubeClient, types.ControllerServiceAccountName)
		if err != nil {
			log.Printf("Failed to add permissions for the event recorder: %s\n", err)
		}
	}

	return nil
}

func (cli *VanClient) appendIngressHost(prefixes []string, namespace string, cred *types.Credential) error {
	routes, err := kube.GetIngressRoutes(types.IngressName, namespace, cli)
	if err != nil {
		return err
	}
	for _, route := range routes {
		for _, prefix := range prefixes {
			if strings.HasPrefix(route.Host, prefix) {
				cred.Hosts = append(cred.Hosts, route.Host)
			}
		}
	}
	return nil
}

func (cli *VanClient) appendLoadBalancerHostOrIp(ctx context.Context, serviceName string, namespace string, cred *types.Credential) error {
	service, err := kube.GetService(serviceName, namespace, cli.KubeClient)
	if err != nil {
		return err
	}
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return nil
	}
	host := kube.GetLoadBalancerHostOrIP(service)

	ctx, _ = getCurrentContextOrDefault(ctx)
	deadlineExceeded := false
	spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond)

	message := "Waiting for LoadBalancer IP or hostname..."
	spin.Prefix = message
	spin.FinalMSG = message + "\n"

	for host == "" && !deadlineExceeded {
		spin.Start()
		select {
		case <-ctx.Done():
			fmt.Println("context deadline exceeded")
			deadlineExceeded = true
			break
		default:
			time.Sleep(time.Second)
			service, err = kube.GetService(serviceName, namespace, cli.KubeClient)
			host = kube.GetLoadBalancerHostOrIP(service)
		}
	}

	spin.Stop()

	if host == "" {
		return fmt.Errorf("Failed to get LoadBalancer IP or Hostname for service %s", serviceName)
	} else {
		cred.Hosts = append(cred.Hosts, host)
		if len(host) < 64 {
			cred.Subject = host
		}
		return nil
	}
}

func (cli *VanClient) createIngress(site types.SiteConfig) error {
	namespace := site.Spec.SkupperNamespace
	if namespace == "" {
		namespace = cli.Namespace
	}

	var routes []kube.IngressRoute
	if site.Spec.EnableFlowCollector {
		if site.Spec.GetControllerIngressHost() != "" {
			routes = append(routes, kube.IngressRoute{
				Host:        strings.Join([]string{"console", namespace, site.Spec.GetControllerIngressHost()}, "."),
				ServiceName: types.ControllerServiceName,
				ServicePort: int(types.FlowCollectorDefaultServicePort),
			})
		} else {
			routes = append(routes, kube.IngressRoute{
				Host:        "console",
				ServiceName: types.ControllerServiceName,
				ServicePort: int(types.FlowCollectorDefaultServicePort),
				Resolve:     true,
			})
		}
	}
	if site.Spec.GetRouterIngressHost() != "" {
		routes = append(routes, kube.IngressRoute{
			Host:        strings.Join([]string{"inter-router", namespace, site.Spec.GetRouterIngressHost()}, "."),
			ServiceName: types.TransportServiceName,
			ServicePort: int(types.InterRouterListenerPort),
		})
		routes = append(routes, kube.IngressRoute{
			Host:        strings.Join([]string{"edge", namespace, site.Spec.GetRouterIngressHost()}, "."),
			ServiceName: types.TransportServiceName,
			ServicePort: int(types.EdgeListenerPort),
		})
		routes = append(routes, kube.IngressRoute{
			Host:        strings.Join([]string{"claims", namespace, site.Spec.GetRouterIngressHost()}, "."),
			ServiceName: types.TransportServiceName,
			ServicePort: int(types.ClaimRedemptionPort),
		})
	} else {
		routes = append(routes, kube.IngressRoute{
			Host:        "inter-router",
			ServiceName: types.TransportServiceName,
			ServicePort: int(types.InterRouterListenerPort),
			Resolve:     true,
		})
		routes = append(routes, kube.IngressRoute{
			Host:        "edge",
			ServiceName: types.TransportServiceName,
			ServicePort: int(types.EdgeListenerPort),
			Resolve:     true,
		})
		routes = append(routes, kube.IngressRoute{
			Host:        "claims",
			ServiceName: types.TransportServiceName,
			ServicePort: int(types.ClaimRedemptionPort),
			Resolve:     true,
		})
	}

	return kube.CreateIngress(types.IngressName, routes, site.Spec.IsIngressNginxIngress(), true, asOwnerReferences(site.Reference), namespace, site.Spec.IngressAnnotations, site.Spec.Labels, cli)
}

func (cli *VanClient) createContourProxies(site types.SiteConfig) error {
	namespace := site.Spec.SkupperNamespace
	if namespace == "" {
		namespace = cli.Namespace
	}

	var routes []kube.IngressRoute
	if site.Spec.EnableFlowCollector && site.Spec.GetControllerIngressHost() != "" {
		routes = append(routes, kube.IngressRoute{
			Name:        types.ConsoleIngressPrefix,
			Host:        strings.Join([]string{types.ConsoleIngressPrefix, namespace, site.Spec.GetControllerIngressHost()}, "."),
			ServiceName: types.ControllerServiceName,
			ServicePort: int(types.FlowCollectorDefaultServicePort),
		})
	}
	if site.Spec.GetRouterIngressHost() != "" {
		routes = append(routes, kube.IngressRoute{
			Name:        types.InterRouterIngressPrefix,
			Host:        strings.Join([]string{types.InterRouterIngressPrefix, namespace, site.Spec.GetRouterIngressHost()}, "."),
			ServiceName: types.TransportServiceName,
			ServicePort: int(types.InterRouterListenerPort),
		})
		routes = append(routes, kube.IngressRoute{
			Name:        types.EdgeIngressPrefix,
			Host:        strings.Join([]string{types.EdgeIngressPrefix, namespace, site.Spec.GetRouterIngressHost()}, "."),
			ServiceName: types.TransportServiceName,
			ServicePort: int(types.EdgeListenerPort),
		})
		routes = append(routes, kube.IngressRoute{
			Name:        types.ClaimsIngressPrefix,
			Host:        strings.Join([]string{types.ClaimsIngressPrefix, namespace, site.Spec.GetRouterIngressHost()}, "."),
			ServiceName: types.TransportServiceName,
			ServicePort: int(types.ClaimRedemptionPort),
		})
	}
	return kube.CreateContourProxies(routes, site.Spec.Labels, asOwnerReference(site.Reference), cli.DynamicClient, namespace)
}

func asOwnerReference(ref types.SiteConfigReference) *metav1.OwnerReference {
	if ref.Name == "" || ref.UID == "" {
		return nil
	}
	owner := metav1.OwnerReference{
		APIVersion: ref.APIVersion,
		Kind:       ref.Kind,
		Name:       ref.Name,
		UID:        kubetypes.UID(ref.UID),
	}
	if owner.Kind == "" {
		owner.Kind = "ConfigMap"
	}
	if owner.APIVersion == "" {
		owner.APIVersion = "v1"
	}
	return &owner
}

func getCurrentContextOrDefault(ctx context.Context) (context.Context, context.CancelFunc) {
	var currentContext context.Context
	var cancel context.CancelFunc

	currentContext = ctx
	_, ok := ctx.Deadline()

	if !ok {
		currentContext, cancel = context.WithTimeout(ctx, types.DefaultTimeoutDuration)
	}

	return currentContext, cancel
}

func asOwnerReferences(in types.SiteConfigReference) []metav1.OwnerReference {
	ref := asOwnerReference(in)
	if ref == nil {
		return nil
	}
	return []metav1.OwnerReference{*ref}
}

func (cli *VanClient) createPrometheus(ctx context.Context, siteConfig *types.SiteConfig, van types.RouterSpec) error {
	promInfo := config.PrometheusInfo{
		BasicAuth:   false,
		TlsAuth:     false,
		ServiceName: types.ControllerServiceName,
		Namespace:   van.Namespace,
		Port:        strconv.Itoa(int(types.FlowCollectorDefaultServicePort)),
		User:        "admin",
		Password:    "admin",
		Hash:        "",
	}
	if siteConfig.Spec.PrometheusServer.AuthMode == string(types.PrometheusAuthModeBasic) {
		promInfo.BasicAuth = true
		if siteConfig.Spec.PrometheusServer.User != "" {
			promInfo.User = siteConfig.Spec.PrometheusServer.User
		}
		if siteConfig.Spec.PrometheusServer.Password != "" {
			promInfo.Password = siteConfig.Spec.PrometheusServer.Password
		}
		hash, _ := config.HashPrometheusPassword(promInfo.Password)
		promInfo.Hash = string(hash)
	} else if siteConfig.Spec.PrometheusServer.AuthMode == string(types.PrometheusAuthModeTls) {
		promInfo.TlsAuth = true
	}
	prometheusData := &map[string]string{
		"prometheus.yml": config.ScrapeConfigForPrometheus(promInfo),
		"web-config.yml": config.ScrapeWebConfigForPrometheus(promInfo),
	}
	siteOwnerRef := asOwnerReference(siteConfig.Reference)
	var ownerRefs []metav1.OwnerReference
	if siteOwnerRef != nil {
		ownerRefs = []metav1.OwnerReference{*siteOwnerRef}
	}

	kube.NewConfigMap("prometheus-server-config", prometheusData, &siteConfig.Spec.Labels, nil, siteOwnerRef, van.Namespace, cli.KubeClient)

	for _, sa := range van.PrometheusServer.ServiceAccounts {
		sa.ObjectMeta.OwnerReferences = ownerRefs
		_, err := kube.CreateServiceAccount(van.Namespace, sa, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	for _, role := range van.PrometheusServer.Roles {
		role.ObjectMeta.OwnerReferences = ownerRefs
		_, err := kube.CreateRole(van.Namespace, role, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	for _, roleBinding := range van.PrometheusServer.RoleBindings {
		roleBinding.ObjectMeta.OwnerReferences = ownerRefs
		_, err := kube.CreateRoleBinding(van.Namespace, roleBinding, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	for _, svc := range van.PrometheusServer.Services {
		svc.ObjectMeta.OwnerReferences = ownerRefs
		_, err := kube.CreateService(svc, van.Namespace, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	_, err := kube.NewPrometheusServerDeployment(&van, siteOwnerRef, cli.KubeClient)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}
