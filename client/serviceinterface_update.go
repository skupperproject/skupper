package client

import (
	"context"
	jsonencoding "encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func addTargetToServiceInterface(service *types.ServiceInterface, target *types.ServiceInterfaceTarget) {
	modified := false
	targets := []types.ServiceInterfaceTarget{}
	for _, t := range service.Targets {
		if t.Name == target.Name {
			modified = true
			targets = append(targets, *target)
		} else {
			targets = append(targets, t)
		}
	}
	if !modified {
		targets = append(targets, *target)
	}
	service.Targets = targets
}

func getServiceInterfaceTarget(targetType string, targetName string, deducePort bool, cli *VanClient) (*types.ServiceInterfaceTarget, error) {
	if targetType == "deployment" {
		deployment, err := cli.KubeClient.AppsV1().Deployments(cli.Namespace).Get(targetName, metav1.GetOptions{})
		if err == nil {
			target := types.ServiceInterfaceTarget{
				Name:     deployment.ObjectMeta.Name,
				Selector: utils.StringifySelector(deployment.Spec.Selector.MatchLabels),
			}
			if deducePort {
				//TODO: handle case where there is more than one container (need --container option?)
				if deployment.Spec.Template.Spec.Containers[0].Ports != nil {
					target.TargetPort = int(deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
				}
			}
			return &target, nil
		} else {
			return nil, fmt.Errorf("Could not read deployment %s: %s", targetName, err)
		}
	} else if targetType == "statefulset" {
		statefulset, err := cli.KubeClient.AppsV1().StatefulSets(cli.Namespace).Get(targetName, metav1.GetOptions{})
		if err == nil {
			target := types.ServiceInterfaceTarget{
				Name:     statefulset.ObjectMeta.Name,
				Selector: utils.StringifySelector(statefulset.Spec.Selector.MatchLabels),
			}
			if deducePort {
				//TODO: handle case where there is more than one container (need --container option?)
				if statefulset.Spec.Template.Spec.Containers[0].Ports != nil {
					target.TargetPort = int(statefulset.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
				}
			}
			return &target, nil
		} else {
			return nil, fmt.Errorf("Could not read statefulset %s: %s", targetName, err)
		}
	} else if targetType == "pods" {
		return nil, fmt.Errorf("VAN service interfaces for pods not yet implemented")
	} else if targetType == "service" {
		target := types.ServiceInterfaceTarget{
			Name:    targetName,
			Service: targetName,
		}
		if deducePort {
			port, err := kube.GetPortForServiceTarget(targetName, cli.Namespace, cli.KubeClient)
			if err != nil {
				return nil, err
			}
			if port != 0 {
				target.TargetPort = port
			}
		}
		return &target, nil
	} else {
		return nil, fmt.Errorf("VAN service interface unsupported target type")
	}
}

func updateServiceInterface(service *types.ServiceInterface, overwriteIfExists bool, owner *metav1.OwnerReference, cli *VanClient) error {
	encoded, err := jsonencoding.Marshal(service)
	if err != nil {
		return fmt.Errorf("Failed to encode service interface as json: %s", err)
	}
	var unretryable error = nil
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
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
					return fmt.Errorf("Failed to update skupper-services config map: %s", err)
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
	if service.Headless != nil {
		if service.Headless.TargetPort < 0 || 65535 < service.Headless.TargetPort {
			return fmt.Errorf("Bad headless target port number: %d", service.Headless.TargetPort)
		}
	}

	for _, target := range service.Targets {
		if target.TargetPort < 0 || 65535 < target.TargetPort {
			return fmt.Errorf("Bad target port number. Target: %s  Port: %d", target.Name, target.TargetPort)
		}
	}

	//TODO: change service.Protocol to service.Mapping
	if service.Port < 0 || 65535 < service.Port {
		return fmt.Errorf("Port %d is outside valid range.", service.Port)
	} else if service.Aggregate != "" && service.EventChannel {
		return fmt.Errorf("Only one of aggregate and event-channel can be specified for a given service.")
	} else if service.Aggregate != "" && service.Aggregate != "json" && service.Aggregate != "multipart" {
		return fmt.Errorf("%s is not a valid aggregation strategy. Choose 'json' or 'multipart'.", service.Aggregate)
	} else if service.Protocol != "" && service.Protocol != "tcp" && service.Protocol != "http" && service.Protocol != "http2" {
		return fmt.Errorf("%s is not a valid mapping. Choose 'tcp', 'http' or 'http2'.", service.Protocol)
	} else if service.Aggregate != "" && service.Protocol != "http" {
		return fmt.Errorf("The aggregate option is currently only valid for http")
	} else if service.EventChannel && service.Protocol != "http" {
		return fmt.Errorf("The event-channel option is currently only valid for http")
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
		return fmt.Errorf("Skupper not initialised in %s", cli.Namespace)
	} else {
		return err
	}
}

func (cli *VanClient) ServiceInterfaceBind(ctx context.Context, service *types.ServiceInterface, targetType string, targetName string, protocol string, targetPort int) error {
	owner, err := getRootObject(cli)
	if err == nil {
		err = validateServiceInterface(service)
		if err != nil {
			return err
		}
		if protocol != "" && service.Protocol != protocol {
			return fmt.Errorf("Invalid protocol %s for service with mapping %s", protocol, service.Protocol)
		}
		target, err := getServiceInterfaceTarget(targetType, targetName, service.Port == 0 && targetPort == 0, cli)
		if err != nil {
			return err
		}
		if target.TargetPort != 0 {
			service.Port = target.TargetPort
			target.TargetPort = 0
		} else if targetPort != 0 {
			if service.Port == 0 {
				service.Port = targetPort
			} else {
				target.TargetPort = targetPort
			}
		}
		if service.Port == 0 {
			if protocol == "http" {
				service.Port = 80
			} else {
				return fmt.Errorf("Service port required and cannot be deduced.")
			}
		}
		addTargetToServiceInterface(service, target)
		return updateServiceInterface(service, true, owner, cli)
	} else if errors.IsNotFound(err) {
		return fmt.Errorf("Skupper not initialised in %s", cli.Namespace)
	} else {
		return err
	}
}

func (cli *VanClient) GetHeadlessServiceConfiguration(targetName string, protocol string, address string, port int) (*types.ServiceInterface, error) {
	statefulset, err := cli.KubeClient.AppsV1().StatefulSets(cli.Namespace).Get(targetName, metav1.GetOptions{})
	if err == nil {
		if address != "" && address != statefulset.Spec.ServiceName {
			return nil, fmt.Errorf("Cannot specify different address from service name for headless service.")
		}
		service, err := cli.KubeClient.CoreV1().Services(cli.Namespace).Get(statefulset.Spec.ServiceName, metav1.GetOptions{})
		if err == nil {
			def := types.ServiceInterface{
				Address:  statefulset.Spec.ServiceName,
				Port:     port,
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
			if port == 0 {
				if len(service.Spec.Ports) == 1 {
					def.Port = int(service.Spec.Ports[0].Port)
					if service.Spec.Ports[0].TargetPort.IntValue() != 0 && int(service.Spec.Ports[0].Port) != service.Spec.Ports[0].TargetPort.IntValue() {
						//TODO: handle string ports
						def.Headless.TargetPort = service.Spec.Ports[0].TargetPort.IntValue()
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
