package client

import (
	"context"
	jsonencoding "encoding/json"
	"fmt"

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
	root, err := cli.KubeClient.AppsV1().Deployments(cli.Namespace).Get(types.TransportDeploymentName, metav1.GetOptions{})
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
		current, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get(types.ServiceInterfaceConfigMap, metav1.GetOptions{})
		if err == nil {
			if overwriteIfExists || current.Data == nil || current.Data[service.Address] == "" {
				if current.Data == nil {
					current.Data = map[string]string{
						service.Address: string(encoded),
					}
				} else {
					current.Data[service.Address] = string(encoded)
				}
				_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Update(current)
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
			_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Create(&configMap)
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

func validateServiceInterface(service *types.ServiceInterface) error {
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
	} else if service.Protocol != "" && service.Protocol != "tcp" && service.Protocol != "http" && service.Protocol != "http2" {
		return fmt.Errorf("%s is not a valid mapping. Choose 'tcp', 'http' or 'http2'.", service.Protocol)
	} else if service.Aggregate != "" && service.Protocol != "http" {
		return fmt.Errorf("The aggregate option is currently only valid for http")
	} else if service.EventChannel && service.Protocol != "http" {
		return fmt.Errorf("The event-channel option is currently only valid for http")
	} else if service.EnableTls && service.Protocol != "http2" {
		return fmt.Errorf("The TLS support is only available for http2")
	} else {
		return nil
	}
}

func (cli *VanClient) ServiceInterfaceUpdate(ctx context.Context, service *types.ServiceInterface) error {
	owner, err := getRootObject(cli)
	if err == nil {
		_, err = cli.ServiceInterfaceInspect(ctx, service.Address)
		if err == nil {
			err = validateServiceInterface(service)
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

func (cli *VanClient) ServiceInterfaceBind(ctx context.Context, service *types.ServiceInterface, targetType string, targetName string, protocol string, targetPorts map[int]int) error {
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
		err = validateServiceInterface(service)
		if err != nil {
			return err
		}
		if protocol != "" && service.Protocol != protocol {
			return fmt.Errorf("Invalid protocol %s for service with mapping %s", protocol, service.Protocol)
		}
		deducePorts := len(service.Ports) == 0 && len(targetPorts) == 0
		target, err := kube.GetServiceInterfaceTarget(targetType, targetName, deducePorts, cli.Namespace, cli.KubeClient)
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
			if protocol == "http" {
				service.Ports = append(service.Ports, 80)
			} else {
				return fmt.Errorf("Service port required and cannot be deduced.")
			}
		}
		service.AddTarget(target)
		return updateServiceInterface(service, true, owner, cli)
	} else if errors.IsNotFound(err) {
		return fmt.Errorf("Skupper is not enabled in namespace '%s'", cli.Namespace)
	} else {
		return err
	}
}

func (cli *VanClient) GetHeadlessServiceConfiguration(targetName string, protocol string, address string, ports []int) (*types.ServiceInterface, error) {
	statefulset, err := cli.KubeClient.AppsV1().StatefulSets(cli.Namespace).Get(targetName, metav1.GetOptions{})
	if err == nil {
		if address != "" && address != statefulset.Spec.ServiceName {
			return nil, fmt.Errorf("Cannot specify different address from service name for headless service.")
		}
		service, err := cli.KubeClient.CoreV1().Services(cli.Namespace).Get(statefulset.Spec.ServiceName, metav1.GetOptions{})
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
						Name:     statefulset.ObjectMeta.Name,
						Selector: utils.StringifySelector(statefulset.Spec.Selector.MatchLabels),
					},
				},
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

func removeServiceInterfaceTarget(serviceName string, targetName string, deleteIfNoTargets bool, cli *VanClient) error {
	current, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get(types.ServiceInterfaceConfigMap, metav1.GetOptions{})
	if err == nil {
		jsonDef := current.Data[serviceName]
		if jsonDef == "" {
			return fmt.Errorf("Could not find entry for service interface %s", serviceName)
		} else {
			service := types.ServiceInterface{}
			err = jsonencoding.Unmarshal([]byte(jsonDef), &service)
			if err != nil {
				return fmt.Errorf("Failed to read json for service interface %s: %s", serviceName, err)
			} else {
				modified := false
				targets := []types.ServiceInterfaceTarget{}
				for _, t := range service.Targets {
					if t.Name == targetName || (t.Name == "" && targetName == serviceName) {
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
			}
		}
		_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Update(current)
		if err != nil {
			return fmt.Errorf("Failed to update skupper-services config map: %v", err.Error())
		}
	} else if errors.IsNotFound(err) {
		return fmt.Errorf("No skupper service interfaces defined: %v", err.Error())
	} else {
		return fmt.Errorf("Could not retrieve service interfaces from configmap 'skupper-services': %v", err)
	}
	return nil
}

func (cli *VanClient) ServiceInterfaceUnbind(ctx context.Context, targetType string, targetName string, address string, deleteIfNoTargets bool) error {
	if targetType == "deployment" || targetType == "statefulset" || targetType == "service" {
		if address == "" {
			err := removeServiceInterfaceTarget(targetName, targetName, deleteIfNoTargets, cli)
			return err
		} else {
			err := removeServiceInterfaceTarget(address, targetName, deleteIfNoTargets, cli)
			return err
		}
	} else if targetType == "pods" {
		return fmt.Errorf("Target type for service interface not yet implemented")
	} else {
		return fmt.Errorf("Unsupported target type for service interface %s", targetType)
	}
}
