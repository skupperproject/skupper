package kube

import (
	jsonencoding "encoding/json"
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetConfigMapOwnerReference(config *corev1.ConfigMap) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "core/v1",
		Kind:       "ConfigMap",
		Name:       config.ObjectMeta.Name,
		UID:        config.ObjectMeta.UID,
	}
}

func NewConfigMap(name string, data *map[string]string, labels *map[string]string, annotations *map[string]string, owner *metav1.OwnerReference, configMaps types.ConfigMaps) (*corev1.ConfigMap, error) {
	existing, _, err := configMaps.GetConfigMap(name, &metav1.GetOptions{})
	if err == nil {
		// TODO:  already exists
		return existing, nil
	} else if errors.IsNotFound(err) {
		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}

		if data != nil {
			cm.Data = *data
		}
		if labels != nil {
			cm.ObjectMeta.Labels = *labels
		}
		if annotations != nil {
			cm.ObjectMeta.Annotations = *annotations
		}
		if owner != nil {
			cm.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
				*owner,
			}
		}

		created, err := configMaps.CreateConfigMap(cm)

		if err != nil {
			return nil, fmt.Errorf("Failed to create config map: %w", err)
		} else {
			return created, nil
		}
	} else {
		cm := &corev1.ConfigMap{}
		return cm, fmt.Errorf("Failed to check existing config maps: %w", err)
	}
}

func GetConfigMap(name string, cli types.ConfigMaps) (*corev1.ConfigMap, error) {
	current, _, err := cli.GetConfigMap(name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	} else {
		return current, err
	}
}

func UpdateSkupperServices(changed []types.ServiceInterface, deleted []string, origin string, configMaps types.ConfigMaps) error {
	current, _, err := configMaps.GetConfigMap(types.ServiceInterfaceConfigMap, &metav1.GetOptions{})
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

		_, err = configMaps.UpdateConfigMap(current)
		if err != nil {
			return fmt.Errorf("Failed to update skupper-services config map: %s", err)
		}
	} else {
		return fmt.Errorf("Could not retrive service definitions from configmap 'skupper-services', Error: %v", err)
	}

	return nil
}
