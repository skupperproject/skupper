package client

import (
	"context"
	jsonencoding "encoding/json"
	"fmt"
	"strings"

	"github.com/skupperproject/skupper/pkg/kube/qdr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/util/retry"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
)

func getRootObject(cli *VanClient) (*metav1.OwnerReference, error) {
	root, err := cli.KubeClient.AppsV1().Deployments(cli.Namespace).Get(context.TODO(), types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	} else {
		owner := kube.GetDeploymentOwnerReference(root)
		return &owner, nil
	}
}

func updateServiceInterface(service *types.ServiceInterface, overwriteIfExists bool, owner *metav1.OwnerReference, cli *VanClient) error {
	encoded, err := jsonencoding.Marshal(service)
	if err != nil {
		return fmt.Errorf("Failed to encode service interface as json: %s", err)
	}
	var unretryable error = nil
	err = retry.RetryOnConflict(defaultRetry, func() error {
		current, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get(context.TODO(), types.ServiceInterfaceConfigMap, metav1.GetOptions{})
		if err == nil {
			if overwriteIfExists || current.Data == nil || current.Data[service.Address] == "" {
				if current.Data == nil {
					current.Data = map[string]string{
						service.Address: string(encoded),
					}
				} else {
					current.Data[service.Address] = string(encoded)
				}
				_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Update(context.TODO(), current, metav1.UpdateOptions{})
				if err != nil {
					// do not encapsulate this error, or it won't pass the errors.IsConflict test
					return err
				} else {
					return nil
				}
			} else {
				unretryable = fmt.Errorf("Service %s already defined", service.Address)
				return nil
			}
		} else if errors.IsNotFound(err) {
			configMap := corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: types.ServiceInterfaceConfigMap,
				},
				Data: map[string]string{
					service.Address: string(encoded),
				},
			}
			if owner != nil {
				configMap.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
					*owner,
				}
			}
			_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Create(context.TODO(), &configMap, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("Failed to create skupper-services config map: %s", err)
			} else {
				return nil
			}
		} else {
			return fmt.Errorf("Could not retrieve service interface definitions from configmap: %s", err)
		}
	})
	if unretryable != nil {
		return unretryable
	}
	return err
}

func validateServiceInterface(service *types.ServiceInterface, cli *VanClient) error {
	errs := validation.IsDNS1035Label(service.Address)
	if len(errs) > 0 {
		return fmt.Errorf("Invalid service name: %q", errs)
	}
	if service.Headless != nil && len(service.Headless.TargetPorts) > 0 {
		for _, targetPort := range service.Headless.TargetPorts {
			if targetPort < 0 || 65535 < targetPort {
				return fmt.Errorf("Bad headless target port number: %d", targetPort)
			}
		}
	}

	for _, target := range service.Targets {
		for _, targetPort := range target.TargetPorts {
			if targetPort < 0 || 65535 < targetPort {
				return fmt.Errorf("Bad target port number. Target: %s  Port: %d", target.Name, targetPort)
			}
		}
	}

	if service.TlsCredentials != "" && !strings.HasPrefix(service.TlsCredentials, types.SkupperServiceCertPrefix) {
		secret, err := cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Get(context.TODO(), service.TlsCredentials, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Secret %s not available for service %s", service.TlsCredentials, service.Address)
		}

		if _, ok := secret.Data["tls.crt"]; !ok {
			return fmt.Errorf("tls.crt file is missing in secret %s", service.TlsCredentials)
		}

		if _, ok := secret.Data["tls.key"]; !ok {
			return fmt.Errorf("tls.key file is missing in secret %s", service.TlsCredentials)
		}

		if _, ok := secret.Data["ca.crt"]; !ok {
			return fmt.Errorf("ca.crt file is missing in secret %s", service.TlsCredentials)
		}
	}
	if service.TlsCertAuthority != "" && service.TlsCertAuthority != types.ServiceClientSecret {
		secret, err := cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Get(context.TODO(), service.TlsCertAuthority, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Secret %s not available for service %s", service.TlsCertAuthority, service.Address)
		}
		if _, ok := secret.Data["ca.crt"]; !ok {
			return fmt.Errorf("ca.crt file is missing in secret %s", service.TlsCertAuthority)
		}
	}

	// TODO: change service.Protocol to service.Mapping
	for _, port := range service.Ports {
		if port < 0 || 65535 < port {
			return fmt.Errorf("Port %d is outside valid range.", port)
		}
	}
	if service.Aggregate != "" && service.EventChannel {
		return fmt.Errorf("Only one of aggregate and event-channel can be specified for a given service.")
	} else if service.Aggregate != "" && service.Aggregate != "json" && service.Aggregate != "multipart" {
		return fmt.Errorf("%s is not a valid aggregation strategy. Choose 'json' or 'multipart'.", service.Aggregate)
	} else if (service.Protocol != "" && service.Protocol != "tcp" && service.Protocol != "http" && service.Protocol != "http2") && service.BridgeImage == "" {
		return fmt.Errorf("%s is not a valid mapping. Choose 'tcp', 'http' or 'http2'.", service.Protocol)
	} else if service.Aggregate != "" && service.Protocol != "http" {
		return fmt.Errorf("The aggregate option is currently only valid for http")
	} else if service.EventChannel && service.Protocol != "http" && service.Protocol != "udp" {
		return fmt.Errorf("The event-channel option is currently only valid for http")
	} else if (service.TlsCredentials != "" || service.TlsCertAuthority != "") && service.Protocol == "http" {
		return fmt.Errorf("The TLS support is only available for http2 and tcp protocols")
	} else {
		return nil
	}

}

func (cli *VanClient) ServiceInterfaceUpdate(ctx context.Context, service *types.ServiceInterface) error {
	owner, err := getRootObject(cli)
	if err == nil {
		_, err = cli.ServiceInterfaceInspect(ctx, service.Address)
		if err == nil {
			err = validateServiceInterface(service, cli)
			if err != nil {
				return err
			}
			return updateServiceInterface(service, true, owner, cli)
		} else {
			return fmt.Errorf("Service not found: %w", err)
		}
	} else if errors.IsNotFound(err) {
		return fmt.Errorf("Skupper is not enabled in namespace '%s'", cli.Namespace)
	} else {
		return err
	}
}

func (cli *VanClient) ServiceInterfaceBind(ctx context.Context, service *types.ServiceInterface, targetType string, targetName string, targetPorts map[int]int, namespace string) error {
	policy := NewPolicyValidatorAPI(cli)
	res, err := policy.Expose(targetType, targetName)
	if err != nil {
		return err
	}
	if !res.Allowed {
		return res.Err()
	}
	owner, err := getRootObject(cli)
	if err == nil {
		err = validateServiceInterface(service, cli)
		if err != nil {
			return err
		}
		err = validateCrossNamespacePermissions(cli, namespace)
		if err != nil {
			return err
		}
		deducePorts := len(service.Ports) == 0 && len(targetPorts) == 0
		svcNamespace := utils.GetOrDefault(namespace, cli.GetNamespace())
		target, err := kube.GetServiceInterfaceTarget(targetType, targetName, deducePorts, svcNamespace, cli.KubeClient, cli.OCAppsClient)
		if err != nil {
			return err
		}
		if len(service.Ports) == 0 && len(target.TargetPorts) > 0 {
			for _, ePort := range target.TargetPorts {
				service.Ports = append(service.Ports, ePort)
			}
			target.TargetPorts = map[int]int{}
		} else if len(targetPorts) > 0 {
			if len(service.Ports) == 0 {
				for iPort, _ := range targetPorts {
					service.Ports = append(service.Ports, iPort)
				}
			} else {
				target.TargetPorts = targetPorts
			}
		}
		if len(service.Ports) == 0 {
			if service.Protocol == "http" {
				service.Ports = append(service.Ports, 80)
			} else {
				return fmt.Errorf("Service port required and cannot be deduced.")
			}
		}
		service.AddTarget(target)

		tlsSupport := qdr.TlsServiceSupport{Address: service.Address, Credentials: service.TlsCredentials, CertAuthority: service.TlsCertAuthority}
		tlsManager := &qdr.TlsManager{KubeClient: cli.KubeClient, Namespace: cli.Namespace}
		err = tlsManager.EnableTlsSupport(tlsSupport)

		if err != nil {
			return err
		}

		return updateServiceInterface(service, true, owner, cli)
	} else if errors.IsNotFound(err) {
		return fmt.Errorf("Skupper is not enabled in namespace '%s'", cli.Namespace)
	} else {
		return err
	}
}

func (cli *VanClient) GetHeadlessServiceConfiguration(targetName string, protocol string, address string, ports []int, publishNotReadyAddresses bool, namespace string) (*types.ServiceInterface, error) {
	svcNamespace := utils.GetOrDefault(namespace, cli.GetNamespace())
	statefulset, err := cli.KubeClient.AppsV1().StatefulSets(svcNamespace).Get(context.TODO(), targetName, metav1.GetOptions{})
	if err == nil {
		if address != "" && address != statefulset.Spec.ServiceName {
			return nil, fmt.Errorf("Cannot specify different address from service name for headless service.")
		}
		service, err := cli.KubeClient.CoreV1().Services(svcNamespace).Get(context.TODO(), statefulset.Spec.ServiceName, metav1.GetOptions{})
		if err == nil {
			def := types.ServiceInterface{
				Address:  statefulset.Spec.ServiceName,
				Ports:    ports,
				Protocol: protocol,
				Headless: &types.Headless{
					Name: statefulset.ObjectMeta.Name,
					Size: int(*statefulset.Spec.Replicas),
				},
				Targets: []types.ServiceInterfaceTarget{
					types.ServiceInterfaceTarget{
						Name:      statefulset.ObjectMeta.Name,
						Selector:  utils.StringifySelector(statefulset.Spec.Selector.MatchLabels),
						Namespace: svcNamespace,
					},
				},
				PublishNotReadyAddresses: publishNotReadyAddresses,
			}
			if len(ports) == 0 {
				if len(service.Spec.Ports) > 0 {
					for _, port := range service.Spec.Ports {
						def.Ports = append(def.Ports, int(port.Port))
						if port.TargetPort.IntValue() != 0 && int(port.Port) != port.TargetPort.IntValue() {
							// TODO: handle string ports
							def.Headless.TargetPorts[int(port.Port)] = port.TargetPort.IntValue()
						}
					}
				} else {
					return nil, fmt.Errorf("Specify port")
				}
			}
			return &def, nil
		} else if errors.IsNotFound(err) {
			return nil, fmt.Errorf("Service %s not found for statefulset %s", statefulset.Spec.ServiceName, targetName)
		} else {
			return nil, fmt.Errorf("Could not read service %s: %s", statefulset.Spec.ServiceName, err)
		}
	} else if errors.IsNotFound(err) {
		return nil, fmt.Errorf("StatefulSet %s not found", targetName)
	} else {
		return nil, fmt.Errorf("Could not read StatefulSet %s: %s", targetName, err)
	}
}

func removeServiceInterfaceTarget(serviceName string, targetName string, deleteIfNoTargets bool, namespace string, cli *VanClient) error {
	current, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get(context.TODO(), types.ServiceInterfaceConfigMap, metav1.GetOptions{})
	if err == nil {
		jsonDef := current.Data[serviceName]
		if jsonDef == "" {
			return fmt.Errorf("Could not find entry for service interface %s", serviceName)
		}
		service := types.ServiceInterface{}
		err = jsonencoding.Unmarshal([]byte(jsonDef), &service)
		if err != nil {
			return fmt.Errorf("Failed to read json for service interface %s: %s", serviceName, err)
		}
		if service.IsAnnotated() && kube.IsOriginalServiceModified(service.Address, namespace, cli.GetKubeClient()) {
			_, err = kube.RemoveServiceAnnotations(service.Address, namespace, cli.KubeClient, []string{types.ProxyQualifier})
			if err != nil {
				return fmt.Errorf("Failed to remove %s annotation from modified service: %v", types.ProxyQualifier, err)
			}
		} else {
			modified := false
			targets := []types.ServiceInterfaceTarget{}
			for _, t := range service.Targets {
				if (t.Name == targetName || (t.Name == "" && t.Service == targetName)) && (t.Namespace == "" || t.Namespace == namespace) {
					modified = true
				} else {
					targets = append(targets, t)
				}
			}
			if !modified {
				return fmt.Errorf("Could not find target %s for service interface %s", targetName, serviceName)
			}
			if len(targets) == 0 && deleteIfNoTargets {
				delete(current.Data, serviceName)
			} else {
				service.Targets = targets
				encoded, err := jsonencoding.Marshal(service)
				if err != nil {
					return fmt.Errorf("Failed to create json for service interface: %s", err)
				} else {
					current.Data[serviceName] = string(encoded)
				}
			}
			_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Update(context.TODO(), current, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("Failed to update skupper-services config map: %v", err.Error())
			}
		}
	} else if errors.IsNotFound(err) {
		return fmt.Errorf("No skupper service interfaces defined: %v", err.Error())
	} else {
		return fmt.Errorf("Could not retrieve service interfaces from configmap 'skupper-services': %v", err)
	}
	return nil
}

func validateCrossNamespacePermissions(cli *VanClient, targetNamespace string) error {
	if len(targetNamespace) > 0 && targetNamespace != cli.GetNamespace() {
		clusterRole, err := cli.KubeClient.RbacV1().ClusterRoles().Get(context.TODO(), types.ControllerClusterRoleName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Failed fetching cluster roles: %s", err)
		}
		policyRules := append(types.ClusterControllerPolicyRules, cli.getControllerRules()...)
		if !ContainsAllPolicies(policyRules, clusterRole.Rules) {
			return fmt.Errorf("Current site does not included needed permissions to expose targets in other namespaces")
		}
		return nil
	}
	return nil
}

func (cli *VanClient) ServiceInterfaceUnbind(ctx context.Context, targetType string, targetName string, address string, deleteIfNoTargets bool, namespace string) error {
	svcNamespace := utils.GetOrDefault(namespace, cli.Namespace)
	if targetType == "deployment" || targetType == "statefulset" || targetType == "service" || targetType == "deploymentconfig" {
		if address == "" {
			err := removeServiceInterfaceTarget(targetName, targetName, deleteIfNoTargets, svcNamespace, cli)
			return err
		} else {
			err := removeServiceInterfaceTarget(address, targetName, deleteIfNoTargets, svcNamespace, cli)
			return err
		}
	} else if targetType == "pods" {
		return fmt.Errorf("Target type for service interface not yet implemented")
	} else {
		return fmt.Errorf("Unsupported target type for service interface %s", targetType)
	}
}
