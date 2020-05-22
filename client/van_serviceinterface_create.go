package client

import (
	"context"
	"fmt"

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

func (cli *VanClient) VanServiceInterfaceCreate(ctx context.Context, targetType string, targetName string, options types.VanServiceInterfaceCreateOptions) error {
	current, err := cli.KubeClient.AppsV1().Deployments(cli.Namespace).Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	} else if options.Headless && targetType != "statefulset" {
		return fmt.Errorf("The headless option is only supported for statefulsets")
	} else {
		owner := kube.GetDeploymentOwnerReference(current)
		if targetType == "deployment" {
			target, err := cli.KubeClient.AppsV1().Deployments(cli.Namespace).Get(targetName, metav1.GetOptions{})
			if err == nil {
				//TODO: handle case where there is more than one container (need --container option?)
				port := options.Port
				targetPort := options.TargetPort
				if target.Spec.Template.Spec.Containers[0].Ports != nil {
					if port == 0 {
						port = int(target.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
					} else if targetPort == 0 {
						targetPort = int(target.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
					}
				}
				if port == 0 {
					return fmt.Errorf("Container in deployment does not specify port, use --port option to provide it")
				} else {
					selector := stringifySelector(target.Spec.Selector.MatchLabels)
					if options.Address == "" {
						err = kube.UpdateConfigMapForServiceInterface(target.ObjectMeta.Name, "", selector, port, options, &owner, cli.Namespace, cli.KubeClient)
					} else {
						err = kube.UpdateConfigMapForServiceInterface(options.Address, target.ObjectMeta.Name, selector, port, options, &owner, cli.Namespace, cli.KubeClient)
					}
					return err
				}
			} else {
				return fmt.Errorf("Could not read deployment %s: %s", targetName, err)
			}
		} else if targetType == "statfulset" {
			if options.Headless {
				statefulset, err := cli.KubeClient.AppsV1().StatefulSets(cli.Namespace).Get(targetName, metav1.GetOptions{})
				if err == nil {
					if options.Address != "" && options.Address != statefulset.Spec.ServiceName {
						return fmt.Errorf("Cannot specify different address from service name for headless service.")
					}
					service, err := cli.KubeClient.CoreV1().Services(cli.Namespace).Get(statefulset.Spec.ServiceName, metav1.GetOptions{})
					if err == nil {
						var port int
						var headless types.Headless
						if options.Port != 0 {
							port = options.Port
						} else if len(service.Spec.Ports) == 1 {
							port = int(service.Spec.Ports[0].Port)
							if service.Spec.Ports[0].TargetPort.IntValue() != 0 && int(service.Spec.Ports[0].Port) != service.Spec.Ports[0].TargetPort.IntValue() {
								//TODO: handle string ports
								headless.TargetPort = service.Spec.Ports[0].TargetPort.IntValue()
							}
						} else {
							return fmt.Errorf("Service %s has multiple ports, specify which to use with --port", statefulset.Spec.ServiceName)
						}
						if port > 0 {
							headless.Name = statefulset.ObjectMeta.Name
							headless.Size = int(*statefulset.Spec.Replicas)
							//updateHeadlessServiceDefinition(service.ObjectMeta.Name, headless, port, options, &owner, kube)
							err := kube.UpdateConfigMapForHeadlessServiceInterface(service.ObjectMeta.Name, headless, port, options, &owner, cli.Namespace, cli.KubeClient)
							return err
						}
					} else if errors.IsNotFound(err) {
						return fmt.Errorf("Service %s not found for statefulset %s", statefulset.Spec.ServiceName, targetName)
					} else {
						return fmt.Errorf("Could not read service %s: %s", statefulset.Spec.ServiceName, err)
					}
				} else if errors.IsNotFound(err) {
					return fmt.Errorf("StatefulSet %s not found", targetName)
				} else {
					return fmt.Errorf("Could not read StatefulSet %s: %s", targetName, err)
				}
			} else {
				target, err := cli.KubeClient.AppsV1().StatefulSets(cli.Namespace).Get(targetName, metav1.GetOptions{})
				if err == nil {
					//TODO: handle case where there is more than one container (need --container option?)
					port := options.Port
					targetPort := options.TargetPort
					if target.Spec.Template.Spec.Containers[0].Ports != nil {
						if port == 0 {
							port = int(target.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
						} else if targetPort == 0 {
							targetPort = int(target.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
						}
					}
					if port == 0 {
						return fmt.Errorf("Container in statefulset does not specify port, use --port option to provide it")
					} else {
						selector := stringifySelector(target.Spec.Selector.MatchLabels)
						if options.Address == "" {
							err = kube.UpdateConfigMapForServiceInterface(target.ObjectMeta.Name, "", selector, port, options, &owner, cli.Namespace, cli.KubeClient)
						} else {
							err = kube.UpdateConfigMapForServiceInterface(options.Address, target.ObjectMeta.Name, selector, port, options, &owner, cli.Namespace, cli.KubeClient)
						}
						return err
					}
				} else {
					return fmt.Errorf("Could not read statefulset %s: %s", targetName, err)
				}
			}
		} else if targetType == "pods" {
			return fmt.Errorf("VAN service interfaces for pods not yet implemented")
		} else {
			return fmt.Errorf("VAN service interface unsupported target type")
		}
	}
	return nil
}
