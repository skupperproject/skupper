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

func NewConfigMapWithOwner(name string, owner metav1.OwnerReference, namespace string, kubeclient kubernetes.Interface) (*corev1.ConfigMap, error) {

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

func GetConfigMap(name string, namespace string, cli kubernetes.Interface) (*corev1.ConfigMap, error) {
	current, err := cli.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	} else {
		return current, err
	}
}

func UpdateSkupperServices(changed []types.ServiceInterface, deleted []string, origin string, namespace string, cli kubernetes.Interface) error {
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
