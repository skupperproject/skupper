package client

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

// TODO: should these move to types?
type HostPort struct {
	Host string
	Port string
}

type RouterHostPorts struct {
	Edge        HostPort
	InterRouter HostPort
	Hosts       string
	LocalOnly   bool
}

func annotateConnectionToken(secret *corev1.Secret, role string, host string, port string) {
	if secret.ObjectMeta.Annotations == nil {
		secret.ObjectMeta.Annotations = map[string]string{}
	}
	secret.ObjectMeta.Annotations[role+"-host"] = host
	secret.ObjectMeta.Annotations[role+"-port"] = port
}

func configureHostPortsFromRoutes(result *RouterHostPorts, cli *VanClient, namespace string) (bool, error) {
	if namespace == "" {
		namespace = cli.Namespace
	}
	if cli.RouteClient == nil {
		return false, nil
	} else {
		interRouterRoute, err1 := cli.RouteClient.Routes(namespace).Get("skupper-inter-router", metav1.GetOptions{})
		edgeRoute, err2 := cli.RouteClient.Routes(namespace).Get("skupper-edge", metav1.GetOptions{})
		if err1 != nil && err2 != nil && errors.IsNotFound(err1) && errors.IsNotFound(err2) {
			return false, nil
		} else if err1 != nil {
			return false, err1
		} else if err2 != nil {
			return false, err2
		} else {
			result.Edge.Host = edgeRoute.Spec.Host
			result.Edge.Port = "443"
			result.InterRouter.Host = interRouterRoute.Spec.Host
			result.InterRouter.Port = "443"
			result.Hosts = edgeRoute.Spec.Host + "," + interRouterRoute.Spec.Host
			return true, nil
		}
	}
}

func configureHostPortsForContourProxies(result *RouterHostPorts, cli *VanClient, namespace string) bool {
	if getIngressHost(result, cli, namespace, types.IngressContourHttpProxyString) {
		result.InterRouter.Host = strings.Join([]string{types.InterRouterIngressPrefix, namespace, result.InterRouter.Host}, ".")
		result.Edge.Host = strings.Join([]string{types.EdgeIngressPrefix, namespace, result.Edge.Host}, ".")
		result.InterRouter.Port = "443"
		result.Edge.Port = "443"
		result.Hosts = strings.Join([]string{result.InterRouter.Host, result.Edge.Host}, ",")
		return true
	}
	return false
}

func getNodePorts(result *RouterHostPorts, service *corev1.Service) bool {
	foundEdge, foundInterRouter := false, false
	for _, p := range service.Spec.Ports {
		if p.Name == "inter-router" {
			result.InterRouter.Port = strconv.Itoa(int(p.NodePort))
			foundInterRouter = true
		} else if p.Name == "edge" {
			result.Edge.Port = strconv.Itoa(int(p.NodePort))
			foundEdge = true
		}
	}
	return foundEdge && foundInterRouter
}

func getIngressHost(result *RouterHostPorts, cli *VanClient, namespace string, ingressType string) bool {
	config, err := cli.SiteConfigInspectInNamespace(context.TODO(), nil, namespace)
	if err != nil {
		fmt.Printf("Failed to look up ingress host: %s, ", err)
		fmt.Println()
		return false
	}
	if config == nil {
		return false
	}
	if ingressType != "" && config.Spec.Ingress != ingressType {
		return false
	}
	if host := config.Spec.GetRouterIngressHost(); host != "" {
		result.Hosts = host
		result.InterRouter.Host = result.Hosts
		result.Edge.Host = result.Hosts
		return true
	}
	return false
}

func configureHostPorts(result *RouterHostPorts, cli *VanClient, namespace string) bool {
	if namespace == "" {
		namespace = cli.Namespace
	}
	ok, err := configureHostPortsFromRoutes(result, cli, namespace)
	if err != nil {
		return false
	} else if ok {
		return ok
	} else {
		service, err := cli.KubeClient.CoreV1().Services(namespace).Get(types.TransportServiceName, metav1.GetOptions{})
		if err != nil {
			return false
		} else {
			if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
				host := kube.GetLoadBalancerHostOrIp(service)
				if host != "" {
					result.Hosts = host
					result.InterRouter.Host = host
					result.InterRouter.Port = "55671"
					result.Edge.Host = host
					result.Edge.Port = "45671"
					return true
				} else {
					fmt.Printf("LoadBalancer Host/IP not yet allocated for service %s, ", service.ObjectMeta.Name)
					fmt.Println()
				}
			} else if service.Spec.Type == corev1.ServiceTypeNodePort && getNodePorts(result, service) {
				getIngressHost(result, cli, namespace, "")
				return true
			} else if ok := configureHostPortsForContourProxies(result, cli, namespace); ok {
				return true
			} else {
				ingressRoutes, err := kube.GetIngressRoutes(types.IngressName, cli.Namespace, cli.KubeClient)
				if err != nil {
					fmt.Printf("Could not check for ingress: %s", err)
					fmt.Println()
				} else if len(ingressRoutes) > 0 {
					var edgeHost string
					var interRouterHost string
					for _, route := range ingressRoutes {
						if route.ServicePort == int(types.InterRouterListenerPort) {
							interRouterHost = route.Host
						} else if route.ServicePort == int(types.EdgeListenerPort) {
							edgeHost = route.Host
						}
					}
					if edgeHost != "" && interRouterHost != "" {
						result.Edge.Host = edgeHost
						result.Edge.Port = "443"
						result.InterRouter.Host = interRouterHost
						result.InterRouter.Port = "443"
						result.Hosts = edgeHost + "," + interRouterHost
						return true
					}
				}
			}
			result.LocalOnly = true
			host := fmt.Sprintf("%s.%s", types.TransportServiceName, namespace)
			result.Hosts = host
			result.InterRouter.Host = host
			result.InterRouter.Port = "55671"
			result.Edge.Host = host
			result.Edge.Port = "45671"
			return true
		}
	}
}

func (cli *VanClient) ConnectorTokenCreate(ctx context.Context, subject string, namespace string) (*corev1.Secret, bool, error) {
	if namespace == "" {
		namespace = cli.Namespace
	}
	// TODO: return error message for all the paths
	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, namespace, cli.KubeClient)
	if err != nil {
		return nil, false, err
	}
	current, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return nil, false, err
	}
	if current.IsEdge() {
		return nil, false, fmt.Errorf("Edge configuration cannot accept connections")
	}
	// TODO: creat const for ca
	caSecret, err := cli.KubeClient.CoreV1().Secrets(namespace).Get(types.SiteCaSecret, metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}
	// get the host and port for inter-router and edge
	var hostPorts RouterHostPorts
	if !configureHostPorts(&hostPorts, cli, namespace) {
		// TODO: return the actual error
		return nil, false, fmt.Errorf("Could not determine host/ports for token")
	}
	secret := certs.GenerateSecret(subject, subject, hostPorts.Hosts, caSecret)
	annotateConnectionToken(&secret, "inter-router", hostPorts.InterRouter.Host, hostPorts.InterRouter.Port)
	annotateConnectionToken(&secret, "edge", hostPorts.Edge.Host, hostPorts.Edge.Port)
	secret.Annotations[types.SiteVersion] = current.GetSiteMetadata().Version
	if secret.ObjectMeta.Labels == nil {
		secret.ObjectMeta.Labels = map[string]string{}
	}
	secret.ObjectMeta.Labels[types.SkupperTypeQualifier] = types.TypeToken
	// Store our siteID in the token, to prevent later self-connection.
	siteConfig, err := cli.SiteConfigInspect(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	if siteConfig != nil {
		secret.ObjectMeta.Annotations[types.TokenGeneratedBy] = siteConfig.Reference.UID
	}
	return &secret, hostPorts.LocalOnly, nil
}

func (cli *VanClient) ConnectorTokenCreateFile(ctx context.Context, subject string, secretFile string) error {
	policy := NewPolicyValidatorAPI(cli)
	res, err := policy.IncomingLink()
	if err != nil {
		return err
	}
	if !res.Allowed {
		return res.Err()
	}
	secret, localOnly, err := cli.ConnectorTokenCreate(ctx, subject, "")
	if err == nil {
		// generate yaml and save it to the specified path
		s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
		out, err := os.Create(secretFile)
		if err != nil {
			return fmt.Errorf("Could not write to file " + secretFile + ": " + err.Error())
		}
		err = s.Encode(secret, out)
		if err != nil {
			return fmt.Errorf("Could not write out generated secret: " + err.Error())
		} else {
			var extra string
			if localOnly {
				extra = "(Note: token will only be valid for local cluster)"
			}
			fmt.Printf("Connection token written to %s %s", secretFile, extra)
			fmt.Println()
			return nil
		}
	} else {
		return err
	}
}
