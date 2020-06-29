package client

import (
	"context"
	jsonencoding "encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
)

// TODO: Move to utils
func stringifySelector(labels map[string]string) string {
	result := ""
	for k, v := range labels {
		if result != "" {
			result += ","
		}
		result += k
		result += "="
		result += v
	}
	return result
}

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
				Selector: stringifySelector(deployment.Spec.Selector.MatchLabels),
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
	} else if targetType == "statfulset" {
		statefulset, err := cli.KubeClient.AppsV1().StatefulSets(cli.Namespace).Get(targetName, metav1.GetOptions{})
		if err == nil {
			target := types.ServiceInterfaceTarget{
				Name:     statefulset.ObjectMeta.Name,
				Selector: stringifySelector(statefulset.Spec.Selector.MatchLabels),
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
	} else {
		return nil, fmt.Errorf("VAN service interface unsupported target type")
	}
}

func updateServiceInterface(service *types.ServiceInterface, overwriteIfExists bool, owner *metav1.OwnerReference, cli *VanClient, rs *types.Results) {
	encoded, err := jsonencoding.Marshal(service)
	if err != nil {
                rs.AddError("Failed to encode service interface as json: %s", err)
                return
	}
	current, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get("skupper-services", metav1.GetOptions{})
	if err == nil {
		if overwriteIfExists || current.Data == nil || current.Data[service.Address] == "" {
			if current.Data == nil {
				current.Data = map[string]string{
					service.Address: string(encoded),
				}
			} else {
				current.Data[service.Address] = string(encoded)
			}
                        rs.AddInfo("Added service address |%s|", service.Address)
			_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Update(current)
			if err != nil {
                                rs.AddError("Failed to update skupper-services config map: %w", err)
                                return
			} else {
                                rs.AddInfo("success")
				return
			}
		} else {
                        rs.AddWarning("Service %s is already defined.", service.Address)
                        return
		}
	} else if errors.IsNotFound(err) {
		configMap := corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "skupper-services",
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
                        rs.AddError("Failed to create skupper-services config map: %s", err)
                        return
		} else {
                        rs.AddInfo("success")
			return 
		}
	} else {
                rs.AddError("Could not retrieve service interface definitions from configmap: %s", err)
                return
	}
}

func validateServiceInterface(service *types.ServiceInterface, rs *types.Results) {
	if service.Headless != nil {
		if service.Headless.TargetPort < 0 || 65535 < service.Headless.TargetPort {
                        rs.AddError("Bad headless target port number: %d", service.Headless.TargetPort)
                        return
		}
	}

	for _, target := range service.Targets {
		if target.TargetPort < 0 || 65535 < target.TargetPort {
                        rs.AddError("Bad target port number. Target: %s  Port: %d", target.Name, target.TargetPort)
                        return
		}
	}

	//TODO: change service.Protocol to service.Mapping
	if service.Port < 0 || 65535 < service.Port {
		rs.AddError("Port %d is outside valid range.", service.Port)
	} else if service.Aggregate != "" && service.EventChannel {
		rs.AddError("Only one of aggregate and event-channel can be specified for a given service.")
	} else if service.Aggregate != "" && service.Aggregate != "json" && service.Aggregate != "multipart" {
		rs.AddError("%s is not a valid aggregation strategy. Choose 'json' or 'multipart'.", service.Aggregate)
	} else if service.Protocol != "" && service.Protocol != "tcp" && service.Protocol != "http" && service.Protocol != "http2" {
		rs.AddError("%s is not a valid mapping. Choose 'tcp', 'http' or 'http2'.", service.Protocol)
	} else if service.Aggregate != "" && service.Protocol != "http" {
		rs.AddError("The aggregate option is currently only valid for http")
	} else if service.EventChannel && service.Protocol != "http" {
		rs.AddError("The event-channel option is currently only valid for http")
	} else {
		rs.AddInfo("Service Interface successfully validated.")
	}
}

func (cli *VanClient) VanServiceInterfaceUpdate(ctx context.Context, service *types.ServiceInterface, rs *types.Results) {
	owner, err := getRootObject(cli)
	if err == nil {
		validateServiceInterface(service, rs)
                if rs.ContainsError() {
                  rs.AddInfo("Aborting.")
                  return
                }
		updateServiceInterface(service, true, owner, cli, rs)
                return
	} else if errors.IsNotFound(err) {
                rs.AddError("Skupper not initialised in %s", cli.Namespace)
                return
	} else {
                rs.AddError("getRootObject error: %w", err)
		return 
	}
}

func (cli *VanClient) VanServiceInterfaceBind(ctx context.Context, service *types.ServiceInterface, targetType string, targetName string, protocol string, targetPort int, rs *types.Results) {
	owner, err := getRootObject(cli)
	if err == nil {
		validateServiceInterface(service, rs)
                if rs.ContainsError() {
                  rs.AddInfo("Aborting.")
                  return
                }
		if protocol != "" && service.Protocol != protocol {
                        rs.AddError("Invalid protocol %s for service with mapping %s", protocol, service.Protocol)
			return
		}
		target, err := getServiceInterfaceTarget(targetType, targetName, service.Port == 0 && targetPort == 0, cli)
		if err != nil {
                        rs.AddError("getServiceInterfaceTarget error: %w", err)
			return
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
		addTargetToServiceInterface(service, target)
		updateServiceInterface(service, true, owner, cli, rs)
                if rs.ContainsError() {
                  rs.AddError("Aborting.")
                  return
                }
	} else if errors.IsNotFound(err) {
                rs.AddError("Skupper not initialised in %s", cli.Namespace)
                return
	} else {
                rs.AddError("getRootObject error: %w", err)
		return
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
	current, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get("skupper-services", metav1.GetOptions{})
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

func (cli *VanClient) VanServiceInterfaceUnbind(ctx context.Context, targetType string, targetName string, address string, deleteIfNoTargets bool) error {
	if targetType == "deployment" || targetType == "statefulset" {
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
