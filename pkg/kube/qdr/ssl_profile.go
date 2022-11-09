package qdr

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"k8s.io/client-go/kubernetes"
)

func AddSslProfile(secretName string, namespace string, cli kubernetes.Interface) error {

	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, namespace, cli)
	if err != nil {
		return err
	}
	current, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return err
	}

	if _, ok := current.SslProfiles[secretName]; !ok {
		current.AddSslProfile(qdr.SslProfile{
			Name: secretName,
		})
	}
	_, err = current.UpdateConfigMap(configmap)
	if err != nil {
		return err
	}

	_, err = cli.CoreV1().ConfigMaps(namespace).Update(configmap)
	if err != nil {
		return err
	}
	return nil

}

func RemoveSslProfile(secretName string, namespace string, cli kubernetes.Interface) error {

	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, namespace, cli)
	if err != nil {
		return err
	}
	current, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return err
	}

	if _, ok := current.SslProfiles[secretName]; ok {
		current.RemoveSslProfile(secretName)
	}

	_, err = current.UpdateConfigMap(configmap)
	if err != nil {
		return err
	}

	_, err = cli.CoreV1().ConfigMaps(namespace).Update(configmap)
	if err != nil {
		return err
	}
	return nil
}

func ExistsSslProfile(secretName string, namespace string, cli kubernetes.Interface) (bool, error) {

	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, namespace, cli)
	if err != nil {
		return false, err
	}
	current, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return false, err
	}

	_, ok := current.SslProfiles[secretName]

	return ok, nil
}
