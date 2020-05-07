package client

import (
	"context"
	"fmt"
	"os"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ajssmith/skupper/api/types"
	"github.com/ajssmith/skupper/pkg/kube"
	"github.com/ajssmith/skupper/pkg/utils"
	"github.com/ajssmith/skupper/pkg/utils/configs"
)

func GetVanControllerSpec(options types.VanRouterCreateOptions, van *types.VanRouterSpec, transport *appsv1.Deployment) {

	if os.Getenv("SKUPPER_CONTROLLER_IMAGE") != "" {
		van.Controller.Image = os.Getenv("SKUPPER_CONTROLLER_IMAGE")
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

	volumes := &[]corev1.Volume{}
	mounts := &[]corev1.VolumeMount{}
	if options.EnableServiceSync {
		origin := utils.RandomId(10)
		envVars = append(envVars, corev1.EnvVar{Name: "SKUPPER_SERVICE_SYNC_ORIGIN", Value: origin})
		kube.AppendSecretVolume(volumes, mounts, "skupper", "/etc/messaging/")
	}
	van.Controller.EnvVar = envVars
	van.Controller.Volumes = *volumes
	van.Controller.VolumeMounts = *mounts

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
}

func GetVanRouterSpecFromOpts(options types.VanRouterCreateOptions, client *VanClient, lbip bool) *types.VanRouterSpec {
	van := &types.VanRouterSpec{}
	//todo: think through van name, router name, secret names, etc.
	if options.SkupperName == "" {
		van.Name = client.Namespace
	} else {
		van.Name = options.SkupperName
	}

	van.Namespace = client.Namespace
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
		Port: 5672,
	})
	sslProfiles = append(sslProfiles, types.SslProfile{
		Name: "skupper-amqps",
	})
	//TODO: vcabbage issue with EXTERNAL, requires ANONYMOUS,false
	listeners = append(listeners, types.Listener{
		Name:             "amqps",
		Host:             "0.0.0.0",
		Port:             5671,
		SslProfile:       "skupper-amqps",
		SaslMechanisms:   "EXTERNAL",
		AuthenticatePeer: true,
	})
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
	van.Transport.EnvVar = envVars

	ports := []corev1.ContainerPort{}
	ports = append(ports, corev1.ContainerPort{
		Name:          "amqps",
		ContainerPort: 5671,
	})
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

	volumes := &[]corev1.Volume{}
	mounts := &[]corev1.VolumeMount{}
	kube.AppendSecretVolume(volumes, mounts, "skupper-amqps", "/etc/qpid-dispatch-certs/skupper-amqps/")
	if !options.IsEdge {
		kube.AppendSecretVolume(volumes, mounts, "skupper-internal", "/etc/qpid-dispatch-certs/skupper-internal/")
	}
	if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
		kube.AppendSecretVolume(volumes, mounts, "skupper-proxy-certs", "/etc/tls/proxy-certs/")
	} else if options.AuthMode == string(types.ConsoleAuthModeInternal) {
		kube.AppendSecretVolume(volumes, mounts, "skupper-console-users", "/etc/qpid-dispatch/sasl-users/")
		kube.AppendConfigVolume(volumes, mounts, "skupper-sasl-config", "/etc/sasl2/")
	}
	van.Transport.Volumes = *volumes
	van.Transport.VolumeMounts = *mounts

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
			"serviceaccounts.openshift.io/oauth-redirectreference.primary": "{\"kind\":\"OAuthRedirectReference\",\"apiVersion\":\"v1\",\"reference\":{\"kind\":\"Route\",\"name\":\"skupper-console\"}}",
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
		Hosts:       "skupper-messaging,skupper-messaging." + client.Namespace + ".svc.cluster.local",
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
	van.Credentials = credentials

	// TODO: this is a hack for ports, fix this
	svcs := []types.Service{}
	svcs = append(svcs, types.Service{
		Name: "skupper-messaging",
		Ports: []corev1.ServicePort{
			corev1.ServicePort{
				Name:       "amqps",
				Protocol:   "TCP",
				Port:       5671,
				TargetPort: intstr.FromInt(5671),
			},
		},
		Type:        "",
		Annotations: map[string]string{},
	})
	if options.EnableConsole {
		if options.AuthMode == string(types.ConsoleAuthModeOpenshift) {
			svcs = append(svcs, types.Service{
				Name: "skupper-console",
				Ports: []corev1.ServicePort{
					corev1.ServicePort{
						Name:       "console",
						Protocol:   "TCP",
						Port:       443,
						TargetPort: intstr.FromInt(443),
					},
				},
				Type:        "",
				Annotations: map[string]string{"service.alpha.openshift.io/serving-cert-secret-name": "skupper-proxy-certs"},
				Termination: routev1.TLSTerminationReencrypt,
			})
		} else {
			svcs = append(svcs, types.Service{
				Name: "skupper-console",
				Ports: []corev1.ServicePort{
					corev1.ServicePort{
						Name:       "console",
						Protocol:   "TCP",
						Port:       8080,
						TargetPort: intstr.FromInt(8080),
					},
				},
				Type:        "",
				Annotations: map[string]string{},
				Termination: routev1.TLSTerminationEdge,
			})
			// if authmode internl then sasl config map and sasl users
			// pick it up here!!!!
		}
	}
	if !options.IsEdge {
		svctype := "ClusterIP"
		if lbip {
			svctype = "LoadBalancer"
		}
		svcs = append(svcs, types.Service{
			Name: "skupper-internal",
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Name:       "inter-router",
					Protocol:   "TCP",
					Port:       55671,
					TargetPort: intstr.FromInt(55671),
				},
				corev1.ServicePort{
					Name:       "edge",
					Protocol:   "TCP",
					Port:       45671,
					TargetPort: intstr.FromInt(45671),
				},
			},
			Type:        svctype,
			Annotations: map[string]string{},
			Termination: routev1.TLSTerminationEdge,
		})
	}
	van.Transport.Services = svcs

	routes := []types.Route{}
	if !lbip {

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
	if options.EnableConsole && client.RouteClient != nil {
		routes = append(routes, types.Route{
			Name:          "skupper-router-console",
			TargetService: types.ConsoleServiceName,
			TargetPort:    types.ConsolePortName,
			Termination:   routev1.TLSTerminationEdge,
		})
	}

	van.Assembly.Routes = routes
	return van
}

// VanRouterCreate instantiates a VAN (router and controller) deployment
func (cli *VanClient) VanRouterCreate(ctx context.Context, options types.VanRouterCreateOptions) error {
	// todo return error
	if options.EnableConsole {
		if options.AuthMode == string(types.ConsoleAuthModeInternal) || options.AuthMode == "" {
			options.AuthMode = string(types.ConsoleAuthModeInternal)
			if options.User == "" {
				options.User = "admin"
			}
			if options.Password == "" {
				options.Password = utils.RandomId(10)
			}
		} else {
			if options.User != "" {
				fmt.Println("--router-console-user only valid when --router-console-auth=internal")
			}
			if options.Password != "" {
				fmt.Println("--router-console-password only valid when --router-console-auth=internal")
			}
		}
	}

	lbip := !options.ClusterLocal && cli.RouteClient == nil

	van := GetVanRouterSpecFromOpts(options, cli, lbip)

	dep, err := kube.NewTransportDeployment(van, cli.KubeClient)
	if err != nil {
		return err
	}
	ownerRef := kube.GetOwnerReference(dep)
	if options.AuthMode == string(types.ConsoleAuthModeInternal) {
		if err := cli.ensureSaslConfig(&ownerRef); err != nil {
			return err
		}
		if err := cli.ensureSaslUsers(options.User, options.Password, &ownerRef); err != nil {
			return err
		}
	}

	for _, sa := range van.Transport.ServiceAccounts {
		kube.NewServiceAccountWithOwner(sa, ownerRef, van.Namespace, cli.KubeClient)
	}
	for _, role := range van.Transport.Roles {
		kube.NewRoleWithOwner(role, ownerRef, van.Namespace, cli.KubeClient)
	}
	for _, roleBinding := range van.Transport.RoleBindings {
		kube.NewRoleBindingWithOwner(roleBinding, ownerRef, van.Namespace, cli.KubeClient)
	}
	for _, ca := range van.CertAuthoritys {
		kube.NewCertAuthorityWithOwner(ca, ownerRef, van.Namespace, cli.KubeClient)
	}
	for _, cred := range van.Credentials {
		if !cred.Post {
			kube.NewSecretWithOwner(cred, ownerRef, van.Namespace, cli.KubeClient)
		}
	}
	for _, svc := range van.Transport.Services {
		kube.NewServiceWithOwner(svc, ownerRef, van.Namespace, cli.KubeClient)
	}
	for _, rte := range van.Assembly.Routes {
		kube.NewRouteWithOwner(rte, ownerRef, van.Namespace, cli.RouteClient)
	}

	// TODO: should this be part of van spec?
	kube.NewConfigMapWithOwner("skupper-services", ownerRef, van.Namespace, cli.KubeClient)

	if !options.IsEdge {
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
							fmt.Println("Failed to get LoadBalancer IP or Hostname for service skupper-internal")
						} else {
							cred.Hosts = host
							if len(host) < 64 {
								cred.Subject = host
							}
						}
					}
				}
				kube.NewSecretWithOwner(cred, ownerRef, van.Namespace, cli.KubeClient)
			}
		}
	}

	if options.EnableController {
		GetVanControllerSpec(options, van, dep)
		depController, err := kube.NewControllerDeployment(van, ownerRef, cli.KubeClient)
		if err != nil {
			return err
		}
		ownerRef = kube.GetOwnerReference(depController)
		for _, sa := range van.Controller.ServiceAccounts {
			kube.NewServiceAccountWithOwner(sa, ownerRef, van.Namespace, cli.KubeClient)
		}
		for _, role := range van.Controller.Roles {
			kube.NewRoleWithOwner(role, ownerRef, van.Namespace, cli.KubeClient)
		}
		for _, roleBinding := range van.Controller.RoleBindings {
			kube.NewRoleBindingWithOwner(roleBinding, ownerRef, van.Namespace, cli.KubeClient)
		}
	}

	return nil
}

func (cli *VanClient) ensureSaslConfig(owner *metav1.OwnerReference) error {
	name := "skupper-sasl-config"
	_, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get(name, metav1.GetOptions{})
	if err == nil {
		fmt.Println("sasl config already exists")
	} else if errors.IsNotFound(err) {
		config := `
pwcheck_method: auxprop
auxprop_plugin: sasldb
sasldb_path: /tmp/qdrouterd.sasldb
`
		configMap := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Data: map[string]string{
				"qdrouterd.conf": config,
			},
		}
		if owner != nil {
			configMap.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
				*owner,
			}
		}
		_, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Create(configMap)
		if err != nil {
			return fmt.Errorf("Failed to create sasl config: %w", err)
		}
	} else {
		return fmt.Errorf("Failed to check for sasl config: %w", err)
	}
	return nil
}

func (cli *VanClient) ensureSaslUsers(user string, password string, owner *metav1.OwnerReference) error {
	name := "skupper-console-users"
	_, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(name, metav1.GetOptions{})
	if err == nil {
		fmt.Println("console users secret already exists")
	} else if errors.IsNotFound(err) {
		secret := corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Data: map[string][]byte{
				user: []byte(password),
			},
		}
		if owner != nil {
			secret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
				*owner,
			}
		}

		_, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Create(&secret)
		if err != nil {
			return fmt.Errorf("Failed to create console users secret: %w", err)
		}
	} else {
		return fmt.Errorf("Failed to create console users secret: %w", err)
	}
	return nil
}
