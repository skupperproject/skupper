package kube

import (
	jsonencoding "encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/skupperproject/skupper/api/types"
)

func NewConfigMapWithOwner(name string, owner metav1.OwnerReference, namespace string, kubeclient *kubernetes.Clientset) (*corev1.ConfigMap, error) {

	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
	}

	actual, err := kubeclient.CoreV1().ConfigMaps(namespace).Create(configMap)

	if err != nil {
		// TODO : come up with a policy for already-exists errors.
		if errors.IsAlreadyExists(err) {
			fmt.Println("ConfigMap", name, "already exists")
			return actual, nil
		} else {
			return actual, fmt.Errorf("Could not create ConfigMap %s: %w", name, err)
		}
	}
	return actual, nil
}

func GetConfigMap(name string, namespace string, cli *kubernetes.Clientset) (*corev1.ConfigMap, error) {
	current, err := cli.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	} else {
		return current, err
	}
}

func UpdateSkupperServices(changed []types.ServiceInterface, deleted []string, origin string, namespace string, cli *kubernetes.Clientset) error {

	current, err := cli.CoreV1().ConfigMaps(namespace).Get("skupper-services", metav1.GetOptions{})
	if err == nil {
        if current.Data == nil {
            current.Data = make(map[string]string)
        }
		for _, def := range changed {
			jsonDef, _ := jsonencoding.Marshal(def)
			current.Data[def.Address] = string(jsonDef)
		}

		for _, name := range deleted {
			delete(current.Data, name)
		}

		_, err = cli.CoreV1().ConfigMaps(namespace).Update(current)
		if err != nil {
			return fmt.Errorf("Failed to update skupper-services config map: %s", err)
		}
	} else {
		return fmt.Errorf("Could not retrive service definitions from configmap 'skupper-services', Error: %v", err)
	}

	return nil
}
func UpdateConfigMapForHeadlessServiceInterface(serviceName string, headless types.Headless, port int, options types.VanServiceInterfaceCreateOptions, owner *metav1.OwnerReference, namespace string, cli *kubernetes.Clientset) error {
	current, err := cli.CoreV1().ConfigMaps(namespace).Get("skupper-services", metav1.GetOptions{})
	if err == nil {
		//is the service already defined?
        if current.Data == nil {
            current.Data = make(map[string]string)
        }
		jsonDef := current.Data[serviceName]
		if jsonDef == "" {
			serviceInterface := types.ServiceInterface{
				Address:  serviceName,
				Protocol: options.Protocol,
				Port:     port,
				Headless: &headless,
			}
			encoded, err := jsonencoding.Marshal(serviceInterface)
			if err != nil {
				return fmt.Errorf("Failed to create json for service definition: %s", err)
			} else {
				current.Data[serviceName] = string(encoded)
			}
		} else {
			service := types.ServiceInterface{}
			err = jsonencoding.Unmarshal([]byte(jsonDef), &service)
			if err != nil {
				return fmt.Errorf("Failed to read json for service definition %s: %s", serviceName, err)
			} else {
				if len(service.Targets) > 0 {
					return fmt.Errorf("Non-headless service definition already exists for %s; unexpose first\n", serviceName)
				}
				service.Address = serviceName
				service.Protocol = options.Protocol
				service.Port = port
				service.Headless = &headless

				encoded, err := jsonencoding.Marshal(service)
				if err != nil {
					return fmt.Errorf("Failed to create json for service definition: %s\n", err)
				} else {
					current.Data[serviceName] = string(encoded)
				}
			}
		}
		_, err = cli.CoreV1().ConfigMaps(namespace).Update(current)
		if err != nil {
			return fmt.Errorf("Failed to update skupper-services config map: %v\n", err.Error())
		}
	} else {
		return fmt.Errorf("Could not retrieve service definitions from configmap 'skupper-services': %v\n", err)
	}
	return nil
}

func UpdateConfigMapForServiceInterface(serviceName string, targetName string, selector string, port int, options types.VanServiceInterfaceCreateOptions, owner *metav1.OwnerReference, namespace string, cli *kubernetes.Clientset) error {
	current, err := cli.CoreV1().ConfigMaps(namespace).Get("skupper-services", metav1.GetOptions{})
	if err == nil {
		//is the service already defined?
        if current.Data == nil {
            current.Data = make(map[string]string)
        }
		serviceTarget := types.ServiceInterfaceTarget{
			Selector: selector,
		}
		if targetName != "" {
			serviceTarget.Name = targetName
		}
		if options.TargetPort != 0 {
			serviceTarget.TargetPort = options.TargetPort
		}
		if current.Data == nil {
			current.Data = make(map[string]string)
		}
		jsonDef := current.Data[serviceName]
		if jsonDef == "" {
			// "entry" seems a bit vague to me here
			serviceInterface := types.ServiceInterface{
				Address:  serviceName,
				Protocol: options.Protocol,
				Port:     port,
				Targets: []types.ServiceInterfaceTarget{
					serviceTarget,
				},
			}
			encoded, err := jsonencoding.Marshal(serviceInterface)
			if err != nil {
				return fmt.Errorf("Failed to create json for service definition: %s", err)
			} else {
				current.Data[serviceName] = string(encoded)
			}
		} else {
			service := types.ServiceInterface{}
			err = jsonencoding.Unmarshal([]byte(jsonDef), &service)
			if err != nil {
				return fmt.Errorf("Failed to read json for service definition %s: %s", serviceName, err)
			} else if service.Headless != nil {
				return fmt.Errorf("Service %s already defined as headless. To allow target use skupper unexpose.", serviceName)
			} else {
				if options.TargetPort != 0 {
					serviceTarget.TargetPort = options.TargetPort
				} else if port != service.Port {
					serviceTarget.TargetPort = port
				}
				modified := false
				targets := []types.ServiceInterfaceTarget{}
				for _, t := range service.Targets {
					if t.Name == serviceTarget.Name {
						modified = true
						targets = append(targets, serviceTarget)
					} else {
						targets = append(targets, t)
					}
				}
				if !modified {
					targets = append(targets, serviceTarget)
				}
				service.Targets = targets
				encoded, err := jsonencoding.Marshal(service)
				if err != nil {
					return fmt.Errorf("Failed to create json for service interface: %s", err)
				} else {
					current.Data[serviceName] = string(encoded)
				}
			}
		}
		_, err = cli.CoreV1().ConfigMaps(namespace).Update(current)
		if err != nil {
			return fmt.Errorf("Failed to update skupper-services config map: %s", err)
		}
	} else {
		return fmt.Errorf("Could not retrieve service interface definitions from configmap: %s", err)
	}
	return nil

}
