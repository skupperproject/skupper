package client

import (
	"context"
	"fmt"
	"strconv"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
)

func OauthProxyContainer(serviceAccount string, servicePort string) *corev1.Container {
	return &corev1.Container{
		Image: "openshift/oauth-proxy:latest",
		Name:  "oauth-proxy",
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
				ContainerPort: types.ConsoleDefaultServicePort,
			},
			{
				Name:          "https",
				ContainerPort: types.ConsoleOpenShiftOauthServiceTargetPort,
			},
		},
	}
}

func (cli *VanClient) GetVanControllerSpec(options types.SiteConfigSpec, van *types.RouterSpec, transport *appsv1.Deployment, siteId string) {
	// service-controller container index
	const (
		serviceController = iota
		oauthProxy
	)

	van.Controller.Image = GetServiceControllerImageDetails()
	van.Controller.Replicas = 1
	//TODO: change these to types constants
	van.Controller.Labels = map[string]string{
		"application":          "skupper",
		"skupper.io/component": "proxy-controller",
	}

	envVars := []corev1.EnvVar{}
	envVars = append(envVars, corev1.EnvVar{Name: "SKUPPER_NAMESPACE", Value: van.Namespace})
	envVars = append(envVars, corev1.EnvVar{Name: "SKUPPER_SITE_NAME", Value: van.Name})
	envVars = append(envVars, corev1.EnvVar{Name: "SKUPPER_SITE_ID", Value: siteId})
	envVars = append(envVars, corev1.EnvVar{Name: "SKUPPER_SERVICE_ACCOUNT", Value: types.TransportServiceAccountName})
	envVars = append(envVars, corev1.EnvVar{Name: "OWNER_NAME", Value: transport.ObjectMeta.Name})
	envVars = append(envVars, corev1.EnvVar{Name: "OWNER_UID", Value: string(transport.ObjectMeta.UID)})
	envVars = addRouterImageOverrideToEnv(envVars)
	if !options.EnableServiceSync {
		envVars = append(envVars, corev1.EnvVar{Name: "SKUPPER_DISABLE_SERVICE_SYNC", Value: "true"})
	}

	sidecars := []*corev1.Container{}
	volumes := []corev1.Volume{}
	mounts := make([][]corev1.VolumeMount, 1)

	if options.EnableConsole {
		if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
			csp := strconv.Itoa(int(types.ConsoleOpenShiftServicePort))
			sidecars = append(sidecars, OauthProxyContainer(types.ControllerServiceAccountName, csp))
			envVars = append(envVars, corev1.EnvVar{Name: "METRICS_PORT", Value: csp})
			envVars = append(envVars, corev1.EnvVar{Name: "METRICS_HOST", Value: "localhost"})
			mounts = append(mounts, []corev1.VolumeMount{})
			kube.AppendSecretVolume(&volumes, &mounts[oauthProxy], types.OauthConsoleSecret, "/etc/tls/proxy-certs/")
		} else if options.AuthMode == string(types.ConsoleAuthModeInternal) {
			envVars = append(envVars, corev1.EnvVar{Name: "METRICS_USERS", Value: "/etc/console-users"})
			kube.AppendSecretVolume(&volumes, &mounts[serviceController], "skupper-console-users", "/etc/console-users/")
		}
	}
	//mount secret needed for communication with router
	kube.AppendSecretVolume(&volumes, &mounts[serviceController], types.LocalClientSecret, "/etc/messaging/")
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
			Name: types.ControllerRoleName,
		},
		Rules: types.ControllerPolicyRule,
	})
	van.Controller.Roles = roles

	roleBindings := []*rbacv1.RoleBinding{}
	roleBindings = append(roleBindings, &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: types.ControllerRoleBindingName,
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

	if options.EnableConsole {
		svctype := corev1.ServiceTypeClusterIP
		metricsPort := []corev1.ServicePort{
			{
				Name:       "metrics",
				Protocol:   "TCP",
				Port:       types.ConsoleDefaultServicePort,
				TargetPort: intstr.FromInt(int(types.ConsoleDefaultServiceTargetPort)),
			},
		}
		termination := routev1.TLSTerminationEdge
		annotations := map[string]string{}

		svcs := []*corev1.Service{}
		if options.IsConsoleIngressRoute() {
			if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
				termination = routev1.TLSTerminationReencrypt
				metricsPort = []corev1.ServicePort{
					{
						Name:       "metrics",
						Protocol:   "TCP",
						Port:       types.ConsoleOpenShiftOauthServicePort,
						TargetPort: intstr.FromInt(int(types.ConsoleOpenShiftOauthServiceTargetPort)),
					},
				}
				annotations = map[string]string{"service.alpha.openshift.io/serving-cert-secret-name": types.OauthConsoleSecret}
			}
		} else if options.IsConsoleIngressLoadBalancer() {
			svctype = corev1.ServiceTypeLoadBalancer
		}
		svcs = append(svcs, &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        types.ControllerServiceName,
				Annotations: annotations,
			},
			Spec: corev1.ServiceSpec{
				Selector: van.Controller.Labels,
				Ports:    metricsPort,
				Type:     svctype,
			},
		})
		van.Controller.Services = svcs

		routes := []*routev1.Route{}
		if options.IsConsoleIngressRoute() {
			routes = append(routes, &routev1.Route{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Route",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: types.ConsoleRouteName,
				},
				Spec: routev1.RouteSpec{
					Path: "",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromString("metrics"),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: types.ControllerServiceName,
					},
					TLS: &routev1.TLSConfig{
						Termination:                   termination,
						InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
					},
				},
			})
		}
		van.Controller.Routes = routes
	}
}

func (cli *VanClient) GetRouterSpecFromOpts(options types.SiteConfigSpec, siteId string) *types.RouterSpec {
	// skupper-router container index
	// TODO: update after dataplance changes
	const (
		qdrouterd = iota
		oauthProxy
	)

	van := &types.RouterSpec{}
	//todo: think through van name, router name, secret names, etc.
	if options.SkupperName == "" {
		van.Name = cli.Namespace
	} else {
		van.Name = options.SkupperName
	}
	if options.SkupperNamespace == "" {
		van.Namespace = cli.Namespace
	} else {
		van.Namespace = options.SkupperNamespace
	}

	van.AuthMode = types.ConsoleAuthMode(options.AuthMode)
	van.Transport.LivenessPort = types.TransportLivenessPort

	van.Transport.Image = GetRouterImageDetails()
	van.Transport.Replicas = 1
	van.Transport.Labels = map[string]string{
		"application":          types.TransportDeploymentName,
		"skupper.io/component": types.TransportComponentName,
	}
	van.Transport.Annotations = types.TransportPrometheusAnnotations
	van.Controller.Annotations = options.Annotations
	for key, value := range options.Annotations {
		van.Transport.Annotations[key] = value
	}

	isEdge := options.RouterMode == string(types.TransportModeEdge)
	routerConfig := qdr.InitialConfig(van.Name+"-${HOSTNAME}", siteId, Version, isEdge, 3)
	if options.RouterLogging != nil {
		configureRouterLogging(&routerConfig, options.RouterLogging)
	}
	routerConfig.AddAddress(qdr.Address{
		Prefix:       "mc",
		Distribution: "multicast",
	})
	routerConfig.AddListener(qdr.Listener{
		Host:        "0.0.0.0",
		Port:        9090,
		Role:        "normal",
		Http:        true,
		HttpRootDir: "disabled",
		Websockets:  false,
		Healthz:     true,
		Metrics:     true,
	})
	routerConfig.AddListener(qdr.Listener{
		Name: "amqp",
		Host: "localhost",
		Port: types.AmqpDefaultPort,
	})
	routerConfig.AddSslProfile(qdr.SslProfile{
		Name: "skupper-amqps",
	})
	routerConfig.AddListener(qdr.Listener{
		Name:             "amqps",
		Host:             "0.0.0.0",
		Port:             types.AmqpsDefaultPort,
		SslProfile:       "skupper-amqps",
		SaslMechanisms:   "EXTERNAL",
		AuthenticatePeer: true,
	})
	if options.EnableRouterConsole {
		if van.AuthMode == types.ConsoleAuthModeOpenshift {
			routerConfig.AddListener(qdr.Listener{
				Name: types.ConsolePortName,
				Host: "localhost",
				Port: types.ConsoleOpenShiftServicePort,
				Http: true,
			})
		} else if van.AuthMode == types.ConsoleAuthModeInternal {
			routerConfig.AddListener(qdr.Listener{
				Name:             types.ConsolePortName,
				Host:             "0.0.0.0",
				Port:             types.ConsoleDefaultServicePort,
				Http:             true,
				AuthenticatePeer: true,
			})
		} else if van.AuthMode == types.ConsoleAuthModeUnsecured {
			routerConfig.AddListener(qdr.Listener{
				Name: types.ConsolePortName,
				Host: "0.0.0.0",
				Port: types.ConsoleDefaultServicePort,
				Http: true,
			})
		}
	}
	if !isEdge {
		routerConfig.AddSslProfile(qdr.SslProfile{
			Name: types.InterRouterProfile,
		})
		routerConfig.AddListener(qdr.Listener{
			Name:             "interior-listener",
			Host:             "0.0.0.0",
			Role:             qdr.RoleInterRouter,
			Port:             types.InterRouterListenerPort,
			SslProfile:       types.InterRouterProfile,
			SaslMechanisms:   "EXTERNAL",
			AuthenticatePeer: true,
		})
		routerConfig.AddListener(qdr.Listener{
			Name:             "edge-listener",
			Host:             "0.0.0.0",
			Role:             qdr.RoleEdge,
			Port:             types.EdgeListenerPort,
			SslProfile:       types.InterRouterProfile,
			SaslMechanisms:   "EXTERNAL",
			AuthenticatePeer: true,
		})
	}
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
	if options.EnableRouterConsole && options.AuthMode == string(types.ConsoleAuthModeInternal) {
		envVars = append(envVars, corev1.EnvVar{Name: "QDROUTERD_AUTO_CREATE_SASLDB_SOURCE", Value: "/etc/qpid-dispatch/sasl-users/"})
		envVars = append(envVars, corev1.EnvVar{Name: "QDROUTERD_AUTO_CREATE_SASLDB_PATH", Value: "/tmp/qdrouterd.sasldb"})
	}
	envVars = append(envVars, corev1.EnvVar{Name: "QDROUTERD_CONF", Value: "/etc/qpid-dispatch/config/" + types.TransportConfigFile})
	envVars = append(envVars, corev1.EnvVar{Name: "QDROUTERD_CONF_TYPE", Value: "json"})
	envVars = append(envVars, corev1.EnvVar{
		Name:  "SKUPPER_SITE_ID",
		Value: siteId,
	})
	if options.RouterDebugMode != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "QDROUTERD_DEBUG",
			Value: options.RouterDebugMode,
		})
	}
	van.Transport.EnvVar = envVars

	ports := []corev1.ContainerPort{}
	ports = append(ports, corev1.ContainerPort{
		Name:          "amqps",
		ContainerPort: types.AmqpsDefaultPort,
	})
	if options.EnableRouterConsole {
		if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
			ports = append(ports, corev1.ContainerPort{
				Name:          types.ConsolePortName,
				ContainerPort: types.ConsoleOpenShiftServicePort,
			})
		} else if options.AuthMode != "" {
			ports = append(ports, corev1.ContainerPort{
				Name:          types.ConsolePortName,
				ContainerPort: types.ConsoleDefaultServicePort,
			})
		}
	}
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

	sidecars := []*corev1.Container{}
	volumes := []corev1.Volume{}
	mounts := make([][]corev1.VolumeMount, 1)
	kube.AppendSecretVolume(&volumes, &mounts[qdrouterd], types.LocalServerSecret, "/etc/qpid-dispatch-certs/skupper-amqps/")
	kube.AppendConfigVolume(&volumes, &mounts[qdrouterd], "router-config", types.TransportConfigMapName, "/etc/qpid-dispatch/config/")
	if !isEdge {
		kube.AppendSecretVolume(&volumes, &mounts[qdrouterd], types.SiteServerSecret, "/etc/qpid-dispatch-certs/skupper-internal/")
	}
	if options.EnableRouterConsole {
		if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
			sidecars = append(sidecars, OauthProxyContainer(types.TransportServiceAccountName, strconv.Itoa(int(types.ConsoleOpenShiftServicePort))))
			mounts = append(mounts, []corev1.VolumeMount{})
			kube.AppendSecretVolume(&volumes, &mounts[oauthProxy], types.OauthRouterConsoleSecret, "/etc/tls/proxy-certs/")
		} else if options.AuthMode == string(types.ConsoleAuthModeInternal) {
			kube.AppendSecretVolume(&volumes, &mounts[qdrouterd], "skupper-console-users", "/etc/qpid-dispatch/sasl-users/")
			kube.AppendConfigVolume(&volumes, &mounts[qdrouterd], "skupper-sasl-config", "skupper-sasl-config", "/etc/sasl2/")
		}
	}
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
			Name: types.TransportRoleName,
		},
		Rules: types.TransportPolicyRule,
	})
	van.Transport.Roles = roles

	roleBindings := []*rbacv1.RoleBinding{}
	roleBindings = append(roleBindings, &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: types.TransportRoleBindingName,
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
		},
	})
	van.Transport.ServiceAccounts = serviceAccounts

	cas := []types.CertAuthority{}
	cas = append(cas, types.CertAuthority{
		Name: types.LocalCaSecret,
	})
	if !isEdge {
		cas = append(cas, types.CertAuthority{
			Name: types.SiteCaSecret,
		})
	}
	van.CertAuthoritys = cas

	credentials := []types.Credential{}
	credentials = append(credentials, types.Credential{
		CA:          types.LocalCaSecret,
		Name:        types.LocalServerSecret,
		Subject:     types.LocalTransportServiceName,
		Hosts:       []string{types.LocalTransportServiceName, types.LocalTransportServiceName + "." + van.Namespace + ".svc.cluster.local"},
		ConnectJson: false,
		Post:        false,
	})
	credentials = append(credentials, types.Credential{
		CA:          types.LocalCaSecret,
		Name:        types.LocalClientSecret,
		Subject:     types.LocalTransportServiceName,
		Hosts:       []string{},
		ConnectJson: true,
		Post:        false,
	})

	if !isEdge {
		if options.IsIngressNone() {
			credentials = append(credentials, types.Credential{
				CA:          types.SiteCaSecret,
				Name:        types.SiteServerSecret,
				Subject:     types.TransportServiceName,
				Hosts:       []string{types.TransportServiceName + "." + van.Namespace},
				ConnectJson: false,
				Post:        false,
			})
		} else {
			credentials = append(credentials, types.Credential{
				CA:          types.SiteCaSecret,
				Name:        types.SiteServerSecret,
				Subject:     types.TransportServiceName,
				Hosts:       []string{types.TransportServiceName + "." + van.Namespace},
				ConnectJson: false,
				Post:        true,
			})
		}
	}
	if options.AuthMode == string(types.ConsoleAuthModeInternal) {
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
		})
	}
	van.Credentials = credentials

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
	if options.EnableRouterConsole {
		if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
			svcs = append(svcs, &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "skupper-router-console",
					Annotations: map[string]string{"service.alpha.openshift.io/serving-cert-secret-name": types.OauthRouterConsoleSecret},
				},
				Spec: corev1.ServiceSpec{
					Selector: van.Transport.Labels,
					Ports: []corev1.ServicePort{
						{
							Name:       "console",
							Protocol:   "TCP",
							Port:       types.ConsoleOpenShiftOauthServicePort,
							TargetPort: intstr.FromInt(int(types.ConsoleOpenShiftOauthServiceTargetPort)),
						},
					},
					Type: "",
				},
			})
		} else {
			svcs = append(svcs, &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "skupper-router-console",
					Annotations: map[string]string{},
				},
				Spec: corev1.ServiceSpec{
					Selector: van.Transport.Labels,
					Ports: []corev1.ServicePort{
						{
							Name:       "console",
							Protocol:   "TCP",
							Port:       types.ConsoleDefaultServicePort,
							TargetPort: intstr.FromInt(int(types.ConsoleDefaultServiceTargetPort)),
						},
					},
					Type: "",
				},
			})
		}
	}
	if !isEdge {
		svcType := corev1.ServiceTypeClusterIP
		if options.IsIngressLoadBalancer() {
			svcType = corev1.ServiceTypeLoadBalancer
		}
		svcs = append(svcs, &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        types.TransportServiceName,
				Annotations: map[string]string{},
			},
			Spec: corev1.ServiceSpec{
				Selector: van.Transport.Labels,
				Ports: []corev1.ServicePort{
					{
						Name:       "inter-router",
						Protocol:   "TCP",
						Port:       types.InterRouterListenerPort,
						TargetPort: intstr.FromInt(int(types.InterRouterListenerPort)),
					},
					{
						Name:       "edge",
						Protocol:   "TCP",
						Port:       types.EdgeListenerPort,
						TargetPort: intstr.FromInt(int(types.EdgeListenerPort)),
					},
				},
				Type: svcType,
			},
		})
	}
	van.Transport.Services = svcs

	routes := []*routev1.Route{}
	if options.IsIngressRoute() {
		routes = append(routes, &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Route",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: types.InterRouterRouteName,
			},
			Spec: routev1.RouteSpec{
				Path: "",
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
				Name: types.EdgeRouteName,
			},
			Spec: routev1.RouteSpec{
				Path: "",
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
	}
	if options.EnableRouterConsole && cli.RouteClient != nil {
		termination := routev1.TLSTerminationEdge
		if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
			termination = routev1.TLSTerminationReencrypt
		}
		routes = append(routes, &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Route",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: types.EdgeRouteName,
			},
			Spec: routev1.RouteSpec{
				Path: "",
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromString(types.EdgeRole),
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: types.InterRouterProfile,
				},
				TLS: &routev1.TLSConfig{
					Termination:                   termination,
					InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				},
			},
		})
	}
	van.Transport.Routes = routes

	return van
}

// RouterCreate instantiates a VAN (router and controller) deployment
func (cli *VanClient) RouterCreate(ctx context.Context, options types.SiteConfig) error {
	// todo return error
	if options.Spec.IsIngressRoute() && cli.RouteClient == nil {
		return fmt.Errorf("OpenShift cluster not detected for --ingress type route")
	}

	if options.Spec.EnableRouterConsole || options.Spec.EnableConsole {
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
	dep, err := kube.NewTransportDeployment(van, siteOwnerRef, cli.KubeClient)
	if err != nil {
		return err
	}
	if siteOwnerRef == nil {
		depRef := kube.GetDeploymentOwnerReference(dep)
		siteOwnerRef = &depRef
	}
	if options.Spec.AuthMode == string(types.ConsoleAuthModeInternal) {
		config := `
pwcheck_method: auxprop
auxprop_plugin: sasldb
sasldb_path: /tmp/qdrouterd.sasldb
`
		saslData := &map[string]string{
			"qdrouterd.conf": config,
		}
		kube.NewConfigMap("skupper-sasl-config", saslData, siteOwnerRef, van.Namespace, cli.KubeClient)
	}
	for _, sa := range van.Transport.ServiceAccounts {
		sa.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*siteOwnerRef}
		kube.CreateServiceAccount(van.Namespace, sa, cli.KubeClient)
	}
	for _, role := range van.Transport.Roles {
		role.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*siteOwnerRef}
		kube.CreateRole(van.Namespace, role, cli.KubeClient)
	}
	for _, roleBinding := range van.Transport.RoleBindings {
		roleBinding.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*siteOwnerRef}
		kube.CreateRoleBinding(van.Namespace, roleBinding, cli.KubeClient)
	}
	for _, ca := range van.CertAuthoritys {
		kube.NewCertAuthority(ca, siteOwnerRef, van.Namespace, cli.KubeClient)
	}
	for _, cred := range van.Credentials {
		if !cred.Post {
			kube.NewSecret(cred, siteOwnerRef, van.Namespace, cli.KubeClient)
		}
	}
	for _, svc := range van.Transport.Services {
		svc.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*siteOwnerRef}
		kube.CreateService(svc, van.Namespace, cli.KubeClient)
	}
	if options.Spec.IsIngressRoute() {
		for _, rte := range van.Transport.Routes {
			rte.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*siteOwnerRef}
			kube.CreateRoute(rte, van.Namespace, cli.RouteClient)
		}
	}

	kube.NewConfigMap(types.ServiceInterfaceConfigMap, nil, siteOwnerRef, van.Namespace, cli.KubeClient)
	initialConfig := qdr.AsConfigMapData(van.RouterConfig)
	kube.NewConfigMap(types.TransportConfigMapName, &initialConfig, siteOwnerRef, van.Namespace, cli.KubeClient)

	if options.Spec.RouterMode == string(types.TransportModeInterior) {
		for _, cred := range van.Credentials {
			if cred.Post {
				if options.Spec.IsIngressRoute() {
					rte, err := kube.GetRoute(types.InterRouterRouteName, van.Namespace, cli.RouteClient)
					if err == nil {
						cred.Hosts = append(cred.Hosts, rte.Spec.Host)
					} else {
						fmt.Println("Failed to retrieve route: ", err.Error())
					}
					rte, err = kube.GetRoute(types.EdgeRouteName, van.Namespace, cli.RouteClient)
					if err == nil {
						cred.Hosts = append(cred.Hosts, rte.Spec.Host)
					} else {
						fmt.Println("Failed to retrieve route: ", err.Error())
					}

				} else {
					service, err := kube.GetService(types.TransportServiceName, van.Namespace, cli.KubeClient)
					if err == nil {
						host := kube.GetLoadBalancerHostOrIP(service)
						for i := 0; host == "" && i < 120; i++ {
							if i == 0 {
								fmt.Println("Waiting for LoadBalancer IP or hostname...")
							}
							time.Sleep(time.Second)
							service, err = kube.GetService(types.TransportServiceName, van.Namespace, cli.KubeClient)
							host = kube.GetLoadBalancerHostOrIP(service)
						}
						if host == "" {
							return fmt.Errorf("Failed to get LoadBalancer IP or Hostname for service %s", types.TransportServiceName)
						} else {
							cred.Hosts = append(cred.Hosts, host)
							if len(host) < 64 {
								cred.Subject = host
							}
						}
					}
				}
				kube.NewSecret(cred, siteOwnerRef, van.Namespace, cli.KubeClient)
			}
		}
	}

	if options.Spec.EnableController {
		cli.GetVanControllerSpec(options.Spec, van, dep, siteId)
		_, err := kube.NewControllerDeployment(van, siteOwnerRef, cli.KubeClient)
		if err != nil {
			return err
		}
		for _, sa := range van.Controller.ServiceAccounts {
			sa.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*siteOwnerRef}
			kube.CreateServiceAccount(van.Namespace, sa, cli.KubeClient)
		}
		for _, role := range van.Controller.Roles {
			role.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*siteOwnerRef}
			kube.CreateRole(van.Namespace, role, cli.KubeClient)
		}
		for _, roleBinding := range van.Controller.RoleBindings {
			roleBinding.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*siteOwnerRef}
			kube.CreateRoleBinding(van.Namespace, roleBinding, cli.KubeClient)
		}
		for _, svc := range van.Controller.Services {
			svc.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*siteOwnerRef}
			kube.CreateService(svc, van.Namespace, cli.KubeClient)
		}
		if options.Spec.IsIngressRoute() {
			for _, rte := range van.Controller.Routes {
				rte.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*siteOwnerRef}
				kube.CreateRoute(rte, van.Namespace, cli.RouteClient)
			}
		}
	}

	return nil
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
