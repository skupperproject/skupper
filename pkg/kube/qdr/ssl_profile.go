package qdr

import (
	"context"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func AddSslProfile(sslProfile qdr.SslProfile, namespace string, cli kubernetes.Interface) error {

	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, namespace, cli)
	if err != nil {
		return err
	}
	current, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return err
	}

	if current != nil {
		if _, ok := current.SslProfiles[sslProfile.Name]; !ok {
			current.AddSslProfile(qdr.SslProfile{
				Name:       sslProfile.Name,
				CertFile:   sslProfile.CertFile,
				CaCertFile: sslProfile.CaCertFile,
			})
		}
		_, err = current.UpdateConfigMap(configmap)
		if err != nil {
			return err
		}

		_, err = cli.CoreV1().ConfigMaps(namespace).Update(context.TODO(), configmap, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
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

	if current != nil {
		if _, ok := current.SslProfiles[secretName]; ok {
			current.RemoveSslProfile(secretName)
		}

		_, err = current.UpdateConfigMap(configmap)
		if err != nil {
			return err
		}

		_, err = cli.CoreV1().ConfigMaps(namespace).Update(context.TODO(), configmap, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
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

	if current != nil {
		_, ok := current.SslProfiles[secretName]

		return ok, nil
	}

	return false, nil

}
