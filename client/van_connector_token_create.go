package client

import (
	"context"
	"fmt"
	"os"

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
		service, err := cli.KubeClient.CoreV1().Services(namespace).Get("skupper-internal", metav1.GetOptions{})
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
				}
			}
			result.LocalOnly = true
			host := fmt.Sprintf("skupper-internal.%s", namespace)
			result.Hosts = host
			result.InterRouter.Host = host
			result.InterRouter.Port = "55671"
			result.Edge.Host = host
			result.Edge.Port = "45671"
			return true
		}
	}
}

func (cli *VanClient) VanConnectorTokenCreate(ctx context.Context, subject string, namespace string) (*corev1.Secret, bool, error) {
	if namespace == "" {
		namespace = cli.Namespace
	}
	// verify that the local deployment is interior mode
	current, err := kube.GetDeployment(types.TransportDeploymentName, namespace, cli.KubeClient)
	// TODO: return error message for all the paths
	if err == nil {
		if qdr.IsInterior(current) {
			//TODO: creat const for ca
			caSecret, err := cli.KubeClient.CoreV1().Secrets(namespace).Get("skupper-internal-ca", metav1.GetOptions{})
			if err == nil {
				//get the host and port for inter-router and edge
				var hostPorts RouterHostPorts
				if configureHostPorts(&hostPorts, cli, namespace) {
					secret := certs.GenerateSecret(subject, subject, hostPorts.Hosts, caSecret)
					annotateConnectionToken(&secret, "inter-router", hostPorts.InterRouter.Host, hostPorts.InterRouter.Port)
					annotateConnectionToken(&secret, "edge", hostPorts.Edge.Host, hostPorts.Edge.Port)
					if secret.ObjectMeta.Labels == nil {
						secret.ObjectMeta.Labels = map[string]string{}
					}
					secret.ObjectMeta.Labels[types.SkupperTypeQualifier] = types.TypeToken
					return &secret, hostPorts.LocalOnly, nil
				} else {
					//TODO: return the actual error
					return nil, false, fmt.Errorf("Could not determine host/ports for token")
				}
			} else {
				return nil, false, err
			}
		} else {
			return nil, false, fmt.Errorf("Edge configuration cannot accept connections")
		}
	} else {
		return nil, false, fmt.Errorf("Could not find skupper router in %q: %s", namespace, err)
	}
}

func (cli *VanClient) VanConnectorTokenCreateFile(ctx context.Context, subject string, secretFile string) error {
	secret, localOnly, err := cli.VanConnectorTokenCreate(ctx, subject, "")
	if err == nil {
		//generate yaml and save it to the specified path
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
