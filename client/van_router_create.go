package client

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubetypes "k8s.io/apimachinery/pkg/types"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/pkg/utils/configs"
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
			corev1.ContainerPort{
				Name:          "http",
				ContainerPort: types.ConsoleDefaultServicePort,
			},
			corev1.ContainerPort{
				Name:          "https",
				ContainerPort: types.ConsoleOpenShiftOauthServiceTargetPort,
			},
		},
	}
}

func (cli *VanClient) GetVanControllerSpec(options types.VanSiteConfigSpec, van *types.VanRouterSpec, transport *appsv1.Deployment, siteId string) {

	if os.Getenv("SKUPPER_SERVICE_CONTROLLER_IMAGE") != "" {
		van.Controller.Image = os.Getenv("SKUPPER_SERVICE_CONTROLLER_IMAGE")
	} else {
		van.Controller.Image = types.DefaultControllerImage
	}
	van.Controller.Replicas = 1
	//TODO: change these to types constants
	van.Controller.Labels = map[string]string{
		"application":          "skupper",
		"skupper.io/component": "proxy-controller",
	}

	var proxyImage string
	if os.Getenv("SKUPPER_PROXY_IMAGE") != "" {
		proxyImage = os.Getenv("SKUPPER_PROXY_IMAGE")
	} else {
		proxyImage = types.DefaultProxyImage
	}

	envVars := []corev1.EnvVar{}
	envVars = append(envVars, corev1.EnvVar{Name: "SKUPPER_NAMESPACE", Value: van.Namespace})
	envVars = append(envVars, corev1.EnvVar{Name: "SKUPPER_PROXY_IMAGE", Value: proxyImage})
	envVars = append(envVars, corev1.EnvVar{Name: "SKUPPER_SERVICE_ACCOUNT", Value: "skupper"})
	envVars = append(envVars, corev1.EnvVar{Name: "OWNER_NAME", Value: transport.ObjectMeta.Name})
	envVars = append(envVars, corev1.EnvVar{Name: "OWNER_UID", Value: string(transport.ObjectMeta.UID)})

	sidecars := []*corev1.Container{}
	volumes := []corev1.Volume{}
	mounts := make([][]corev1.VolumeMount, 0)
	mounts = append(mounts, []corev1.VolumeMount{})

	if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
		csp := strconv.Itoa(int(types.ConsoleOpenShiftServicePort))
		sidecars = append(sidecars, OauthProxyContainer("skupper-proxy-controller", csp))
		envVars = append(envVars, corev1.EnvVar{Name: "METRICS_PORT", Value: csp})
		envVars = append(envVars, corev1.EnvVar{Name: "METRICS_HOST", Value: "localhost"})
		mounts = append(mounts, []corev1.VolumeMount{})
		kube.AppendSecretVolume(&volumes, &mounts[1], "skupper-controller-certs", "/etc/tls/proxy-certs/")
	} else if options.AuthMode == string(types.ConsoleAuthModeInternal) {
		envVars = append(envVars, corev1.EnvVar{Name: "METRICS_USERS", Value: "/etc/console-users"})
		kube.AppendSecretVolume(&volumes, &mounts[0], "skupper-console-users", "/etc/console-users/")
	}

	if options.EnableServiceSync {
		envVars = append(envVars, corev1.EnvVar{
			Name: "SKUPPER_SERVICE_SYNC_ORIGIN",
			Value: siteId,
		})
		kube.AppendSecretVolume(&volumes, &mounts[0], "skupper", "/etc/messaging/")
	}
	van.Controller.EnvVar = envVars
	van.Controller.Volumes = volumes
	van.Controller.VolumeMounts = mounts
	van.Controller.Sidecars = sidecars

	serviceAccounts := []types.ServiceAccount{}
	serviceAccounts = append(serviceAccounts, types.ServiceAccount{
		ServiceAccount: "skupper-proxy-controller",
		Annotations:    map[string]string{},
	})
	van.Controller.ServiceAccounts = serviceAccounts

	roles := []types.Role{}
	roles = append(roles, types.Role{
		Name:  "skupper-edit",
		Rules: types.ControllerEditPolicyRule,
	})
	van.Controller.Roles = roles

	roleBindings := []types.RoleBinding{}
	roleBindings = append(roleBindings, types.RoleBinding{
		ServiceAccount: "skupper-proxy-controller",
		Role:           "skupper-edit",
	})
	van.Controller.RoleBindings = roleBindings

	svctype := "ClusterIP"
	metricsPort := []corev1.ServicePort{
		corev1.ServicePort{
			Name:       "metrics",
			Protocol:   "TCP",
			Port:       types.ConsoleDefaultServicePort,
			TargetPort: intstr.FromInt(int(types.ConsoleDefaultServiceTargetPort)),
		},
	}
	termination := routev1.TLSTerminationEdge
	annotations := map[string]string{}

	svcs := []types.Service{}
	if cli.RouteClient != nil {
		if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
			termination = routev1.TLSTerminationReencrypt
			metricsPort = []corev1.ServicePort{
				corev1.ServicePort{
					Name:       "metrics",
					Protocol:   "TCP",
					Port:       types.ConsoleOpenShiftOauthServicePort,
					TargetPort: intstr.FromInt(int(types.ConsoleOpenShiftOauthServiceTargetPort)),
				},
			}
			annotations = map[string]string{"service.alpha.openshift.io/serving-cert-secret-name": "skupper-controller-certs"}
		}
	} else if !options.ClusterLocal {
		svctype = "LoadBalancer"
	}
	svcs = append(svcs, types.Service{
		Name:        "skupper-controller",
		Ports:       metricsPort,
		Type:        svctype,
		Annotations: annotations,
	})
	van.Controller.Services = svcs

	routes := []types.Route{}
	if !options.ClusterLocal && cli.RouteClient != nil {
		routes = append(routes, types.Route{
			Name:          "skupper-controller",
			TargetService: "skupper-controller",
			TargetPort:    "metrics",
			Termination:   termination,
		})
	}
	van.Controller.Routes = routes
}

func (cli *VanClient) GetVanRouterSpecFromOpts(options types.VanSiteConfigSpec, siteId string) *types.VanRouterSpec {
	van := &types.VanRouterSpec{}
	//todo: think through van name, router name, secret names, etc.
	if options.SkupperName == "" {
		van.Name = cli.Namespace
	} else {
		van.Name = options.SkupperName
	}

	van.Namespace = cli.Namespace
	van.AuthMode = types.ConsoleAuthMode(options.AuthMode)
	van.Transport.LivenessPort = types.TransportLivenessPort

	if os.Getenv("QDROUTERD_MAGE") != "" {
		van.Transport.Image = os.Getenv("QDROUTERD_IMAGE")
	} else {
		van.Transport.Image = types.DefaultTransportImage
	}
	van.Transport.Replicas = 1
	van.Transport.Labels = map[string]string{
		"application":          types.TransportDeploymentName,
		"skupper.io/component": types.TransportComponentName,
	}
	van.Transport.Annotations = types.TransportPrometheusAnnotations

	listeners := []types.Listener{}
	interRouterListeners := []types.Listener{}
	edgeListeners := []types.Listener{}
	sslProfiles := []types.SslProfile{}
	listeners = append(listeners, types.Listener{
		Name: "amqp",
		Host: "localhost",
		Port: types.AmqpDefaultPort,
	})
	sslProfiles = append(sslProfiles, types.SslProfile{
		Name: "skupper-amqps",
	})
	listeners = append(listeners, types.Listener{
		Name:             "amqps",
		Host:             "0.0.0.0",
		Port:             types.AmqpsDefaultPort,
		SslProfile:       "skupper-amqps",
		SaslMechanisms:   "EXTERNAL",
		AuthenticatePeer: true,
	})
	if options.EnableRouterConsole {
		if van.AuthMode == types.ConsoleAuthModeOpenshift {
			listeners = append(listeners, types.Listener{
				Name: types.ConsolePortName,
				Host: "localhost",
				Port: types.ConsoleOpenShiftServicePort,
				Http: true,
			})
		} else if van.AuthMode == types.ConsoleAuthModeInternal {
			listeners = append(listeners, types.Listener{
				Name:             types.ConsolePortName,
				Host:             "0.0.0.0",
				Port:             types.ConsoleDefaultServicePort,
				Http:             true,
				AuthenticatePeer: true,
			})
		} else if van.AuthMode == types.ConsoleAuthModeUnsecured {
			listeners = append(listeners, types.Listener{
				Name: types.ConsolePortName,
				Host: "0.0.0.0",
				Port: types.ConsoleDefaultServicePort,
				Http: true,
			})
		}
	}
	if !options.IsEdge {
		sslProfiles = append(sslProfiles, types.SslProfile{
			Name: "skupper-internal",
		})
		interRouterListeners = append(interRouterListeners, types.Listener{
			Name:             "interior-listener",
			Host:             "0.0.0.0",
			Port:             types.InterRouterListenerPort,
			SslProfile:       types.InterRouterProfile,
			SaslMechanisms:   "EXTERNAL",
			AuthenticatePeer: true,
		})
		edgeListeners = append(edgeListeners, types.Listener{
			Name:             "edge-listener",
			Host:             "0.0.0.0",
			Port:             types.EdgeListenerPort,
			SslProfile:       types.InterRouterProfile,
			SaslMechanisms:   "EXTERNAL",
			AuthenticatePeer: true,
		})
	}
	// TODO: remove redundancy, needed for now for config template
	van.Assembly.Name = van.Name
	if options.IsEdge {
		van.Assembly.Mode = string(types.TransportModeEdge)
	} else {
		van.Assembly.Mode = string(types.TransportModeInterior)
	}
	van.Assembly.Listeners = listeners
	van.Assembly.InterRouterListeners = interRouterListeners
	van.Assembly.EdgeListeners = edgeListeners
	van.Assembly.SslProfiles = sslProfiles

	envVars := []corev1.EnvVar{}
	if !options.IsEdge {
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
	if options.AuthMode == string(types.ConsoleAuthModeInternal) {
		envVars = append(envVars, corev1.EnvVar{Name: "QDROUTERD_AUTO_CREATE_SASLDB_SOURCE", Value: "/etc/qpid-dispatch/sasl-users/"})
		envVars = append(envVars, corev1.EnvVar{Name: "QDROUTERD_AUTO_CREATE_SASLDB_PATH", Value: "/tmp/qdrouterd.sasldb"})
	}
	envVars = append(envVars, corev1.EnvVar{Name: "QDROUTERD_CONF", Value: configs.QdrouterdConfig(&van.Assembly)})
	envVars = append(envVars, corev1.EnvVar{
		Name: "SKUPPER_SITE_ID",
		Value: siteId,
	})
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
	if !options.IsEdge {
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
	mounts := make([][]corev1.VolumeMount, 0)
	mounts = append(mounts, []corev1.VolumeMount{})
	kube.AppendSecretVolume(&volumes, &mounts[0], "skupper-amqps", "/etc/qpid-dispatch-certs/skupper-amqps/")
	if !options.IsEdge {
		kube.AppendSecretVolume(&volumes, &mounts[0], "skupper-internal", "/etc/qpid-dispatch-certs/skupper-internal/")
	}
	if options.EnableRouterConsole {
		if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
			sidecars = append(sidecars, OauthProxyContainer("skupper", strconv.Itoa(int(types.ConsoleOpenShiftServicePort))))
			mounts = append(mounts, []corev1.VolumeMount{})
			kube.AppendSecretVolume(&volumes, &mounts[1], "skupper-proxy-certs", "/etc/tls/proxy-certs/")
		} else if options.AuthMode == string(types.ConsoleAuthModeInternal) {
			kube.AppendSecretVolume(&volumes, &mounts[0], "skupper-console-users", "/etc/qpid-dispatch/sasl-users/")
			kube.AppendConfigVolume(&volumes, &mounts[0], "skupper-sasl-config", "/etc/sasl2/")
		}
	}
	van.Transport.Volumes = volumes
	van.Transport.VolumeMounts = mounts
	van.Transport.Sidecars = sidecars

	roles := []types.Role{}
	roles = append(roles, types.Role{
		Name:  types.TransportViewRoleName,
		Rules: types.TransportViewPolicyRule,
	})
	van.Transport.Roles = roles

	roleBindings := []types.RoleBinding{}
	roleBindings = append(roleBindings, types.RoleBinding{
		ServiceAccount: types.TransportServiceAccountName,
		Role:           types.TransportViewRoleName,
	})
	van.Transport.RoleBindings = roleBindings

	serviceAccounts := []types.ServiceAccount{}
	annotation := map[string]string{}
	if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
		annotation = map[string]string{
			"serviceaccounts.openshift.io/oauth-redirectreference.primary": "{\"kind\":\"OAuthRedirectReference\",\"apiVersion\":\"v1\",\"reference\":{\"kind\":\"Route\",\"name\":\"skupper-router-console\"}}",
		}
	}
	serviceAccounts = append(serviceAccounts, types.ServiceAccount{
		ServiceAccount: types.TransportServiceAccountName,
		Annotations:    annotation,
	})
	van.Transport.ServiceAccounts = serviceAccounts

	cas := []types.CertAuthority{}
	cas = append(cas, types.CertAuthority{
		Name: "skupper-ca",
	})
	if !options.IsEdge {
		cas = append(cas, types.CertAuthority{
			Name: "skupper-internal-ca",
		})
	}
	van.CertAuthoritys = cas

	credentials := []types.Credential{}
	credentials = append(credentials, types.Credential{
		CA:          "skupper-ca",
		Name:        "skupper-amqps",
		Subject:     "skupper-messaging",
		Hosts:       "skupper-messaging,skupper-messaging." + cli.Namespace + ".svc.cluster.local",
		ConnectJson: false,
		Post:        false,
	})
	credentials = append(credentials, types.Credential{
		CA:          "skupper-ca",
		Name:        "skupper",
		Subject:     "skupper-messaging",
		Hosts:       "",
		ConnectJson: true,
		Post:        false,
	})
	// TODO: deal with clusterLocal and .Routes != nil
	if !options.IsEdge {
		post := false
		if !options.ClusterLocal {
			post = true
		}
		credentials = append(credentials, types.Credential{
			CA:          "skupper-internal-ca",
			Name:        "skupper-internal",
			Subject:     "skupper-messaging",
			Hosts:       "",
			ConnectJson: false,
			Post:        post,
		})
	}
	if options.AuthMode == string(types.ConsoleAuthModeInternal) {
		credentials = append(credentials, types.Credential{
			CA:          "",
			Name:        "skupper-console-users",
			Subject:     "",
			Hosts:       "",
			ConnectJson: false,
			Post:        false,
			Data: map[string][]byte{
				options.User: []byte(options.Password),
			},
		})
	}
	van.Credentials = credentials

	// TODO: this is a hack for ports, fix this
	svcs := []types.Service{}
	svcs = append(svcs, types.Service{
		Name: "skupper-messaging",
		Ports: []corev1.ServicePort{
			corev1.ServicePort{
				Name:       "amqps",
				Protocol:   "TCP",
				Port:       types.AmqpsDefaultPort,
				TargetPort: intstr.FromInt(int(types.AmqpsDefaultPort)),
			},
		},
		Type:        "",
		Annotations: map[string]string{},
	})
	if options.EnableRouterConsole {
		if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
			svcs = append(svcs, types.Service{
				Name: "skupper-router-console",
				Ports: []corev1.ServicePort{
					corev1.ServicePort{
						Name:       "console",
						Protocol:   "TCP",
						Port:       types.ConsoleOpenShiftOauthServicePort,
						TargetPort: intstr.FromInt(int(types.ConsoleOpenShiftOauthServiceTargetPort)),
					},
				},
				Type:        "",
				Annotations: map[string]string{"service.alpha.openshift.io/serving-cert-secret-name": "skupper-proxy-certs"},
			})
		} else {
			svcs = append(svcs, types.Service{
				Name: "skupper-router-console",
				Ports: []corev1.ServicePort{
					corev1.ServicePort{
						Name:       "console",
						Protocol:   "TCP",
						Port:       types.ConsoleDefaultServicePort,
						TargetPort: intstr.FromInt(int(types.ConsoleDefaultServiceTargetPort)),
					},
				},
				Type:        "",
				Annotations: map[string]string{},
			})
		}
	}
	if !options.IsEdge {
		svctype := "ClusterIP"
		if !options.ClusterLocal && cli.RouteClient == nil {
			svctype = "LoadBalancer"
		}
		svcs = append(svcs, types.Service{
			Name: "skupper-internal",
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Name:       "inter-router",
					Protocol:   "TCP",
					Port:       types.InterRouterListenerPort,
					TargetPort: intstr.FromInt(int(types.InterRouterListenerPort)),
				},
				corev1.ServicePort{
					Name:       "edge",
					Protocol:   "TCP",
					Port:       types.EdgeListenerPort,
					TargetPort: intstr.FromInt(int(types.EdgeListenerPort)),
				},
			},
			Type:        svctype,
			Annotations: map[string]string{},
		})
	}
	van.Transport.Services = svcs

	routes := []types.Route{}
	if !options.ClusterLocal && cli.RouteClient != nil {
		routes = append(routes, types.Route{
			Name:          types.InterRouterRouteName,
			TargetService: types.InterRouterProfile,
			TargetPort:    types.InterRouterRole,
			Termination:   routev1.TLSTerminationPassthrough,
		})
		routes = append(routes, types.Route{
			Name:          types.EdgeRouteName,
			TargetService: types.InterRouterProfile,
			TargetPort:    types.EdgeRole,
			Termination:   routev1.TLSTerminationPassthrough,
		})
	}
	if options.EnableRouterConsole && cli.RouteClient != nil {
		termination := routev1.TLSTerminationEdge
		if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
			termination = routev1.TLSTerminationReencrypt
		}
		routes = append(routes, types.Route{
			Name:          "skupper-router-console",
			TargetService: "skupper-router-console",
			TargetPort:    types.ConsolePortName,
			Termination:   termination,
		})
	}
	van.Transport.Routes = routes

	return van
}

// VanRouterCreate instantiates a VAN (router and controller) deployment
func (cli *VanClient) VanRouterCreate(ctx context.Context, options types.VanSiteConfig) error {
	// todo return error
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
	van := cli.GetVanRouterSpecFromOpts(options.Spec, siteId)
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
		kube.NewServiceAccount(sa, siteOwnerRef, van.Namespace, cli.KubeClient)
	}
	for _, role := range van.Transport.Roles {
		kube.NewRole(role, siteOwnerRef, van.Namespace, cli.KubeClient)
	}
	for _, roleBinding := range van.Transport.RoleBindings {
		kube.NewRoleBinding(roleBinding, siteOwnerRef, van.Namespace, cli.KubeClient)
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
		kube.NewService(svc, van.Transport.Labels, siteOwnerRef, van.Namespace, cli.KubeClient)
	}
	if cli.RouteClient != nil {
		for _, rte := range van.Transport.Routes {
			kube.NewRoute(rte, siteOwnerRef, van.Namespace, cli.RouteClient)
		}
	}

	kube.NewConfigMap("skupper-services", nil, siteOwnerRef, van.Namespace, cli.KubeClient)

	if !options.Spec.IsEdge {
		for _, cred := range van.Credentials {
			if cred.Post {
				if cli.RouteClient != nil {
					rte, err := kube.GetRoute(types.InterRouterRouteName, van.Namespace, cli.RouteClient)
					if err == nil {
						cred.Hosts = rte.Spec.Host
					} else {
						fmt.Println("Failed to retrieve route: ", err.Error())
					}
					rte, err = kube.GetRoute(types.EdgeRouteName, van.Namespace, cli.RouteClient)
					if err == nil {
						cred.Hosts += "," + rte.Spec.Host
					} else {
						fmt.Println("Failed to retrieve route: ", err.Error())
					}

				} else {
					service, err := kube.GetService(types.InterRouterProfile, van.Namespace, cli.KubeClient)
					if err == nil {
						host := kube.GetLoadBalancerHostOrIP(service)
						for i := 0; host == "" && i < 120; i++ {
							if i == 0 {
								fmt.Println("Waiting for LoadBalancer IP or hostname...")
							}
							time.Sleep(time.Second)
							service, err = kube.GetService(types.InterRouterProfile, van.Namespace, cli.KubeClient)
							host = kube.GetLoadBalancerHostOrIP(service)
						}
						if host == "" {
							return fmt.Errorf("Failed to get LoadBalancer IP or Hostname for service skupper-internal")
						} else {
							cred.Hosts = host
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
			kube.NewServiceAccount(sa, siteOwnerRef, van.Namespace, cli.KubeClient)
		}
		for _, role := range van.Controller.Roles {
			kube.NewRole(role, siteOwnerRef, van.Namespace, cli.KubeClient)
		}
		for _, roleBinding := range van.Controller.RoleBindings {
			kube.NewRoleBinding(roleBinding, siteOwnerRef, van.Namespace, cli.KubeClient)
		}
		for _, svc := range van.Controller.Services {
			kube.NewService(svc, van.Controller.Labels, siteOwnerRef, van.Namespace, cli.KubeClient)
		}
		if cli.RouteClient != nil {
			for _, rte := range van.Controller.Routes {
				kube.NewRoute(rte, siteOwnerRef, van.Namespace, cli.RouteClient)
			}
		}
	}

	return nil
}


func asOwnerReference(ref types.VanSiteConfigReference) *metav1.OwnerReference {
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
