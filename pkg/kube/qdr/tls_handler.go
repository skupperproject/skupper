package qdr

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/service"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strings"
)

type TlsManagerInterface interface {
	EnableTlsSupport(support TlsServiceSupport) error
	DisableTlsSupport(support TlsServiceSupport, serviceList []*types.ServiceInterface) error
}

type TlsServiceSupport struct {
	Address       string
	Credentials   string
	CertAuthority string
}

func (manager *TlsManager) EnableTlsSupport(support TlsServiceSupport) error {

	if support.Credentials != "" {
		if isGeneratedBySkupper(support.Credentials) {
			// If the requested certificate is one generated by skupper it can be generated in other sites as well
			_, err := manager.GetSecret(support.Credentials)
			if err != nil {
				serviceSecret, err := generateNewSecret(support, manager)
				if err != nil {
					return err
				}

				return manager.AddSslProfile(qdr.SslProfile{Name: serviceSecret.Name})
			}
		} else {
			_, err := manager.GetSecret(support.Credentials)
			if err != nil {
				return fmt.Errorf("The secret %s for address %s is missing", support.Credentials, support.Address)
			}

			err = checkAndIncludeSslProfile(qdr.SslProfile{Name: support.Credentials}, manager)
			if err != nil {
				return err
			}
		}
	}

	if support.CertAuthority != "" && support.CertAuthority != types.ServiceClientSecret {
		_, err := manager.GetSecret(support.CertAuthority)
		if err != nil {
			return fmt.Errorf("The secret %s for address %s is missing", support.CertAuthority, support.Address)
		}

		sslProfile := qdr.SslProfile{
			Name:       support.CertAuthority,
			CaCertFile: fmt.Sprintf("/etc/skupper-router-certs/%s/ca.crt", support.CertAuthority),
		}

		err = checkAndIncludeSslProfile(sslProfile, manager)
		if err != nil {
			return err
		}
	}

	return nil
}

func (manager *TlsManager) DisableTlsSupport(tlsSupport TlsServiceSupport, serviceList []*types.ServiceInterface) error {
	if len(tlsSupport.Credentials) > 0 {
		if isGeneratedBySkupper(tlsSupport.Credentials) {

			if len(serviceList) > 0 {

				for _, service := range serviceList {
					if service.TlsCredentials == tlsSupport.Credentials && service.Address != tlsSupport.Address {
						return nil
					}
				}

			}

			err := manager.RemoveSslProfile(tlsSupport.Credentials)
			if err != nil {
				return err
			}

			_, err = manager.GetSecret(tlsSupport.Credentials)
			if err == nil {
				err = manager.DeleteSecret(tlsSupport.Credentials)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func CheckBindingSecrets(services map[string]*service.ServiceBindings, namespace string, cli kubernetes.Interface) error {
	for _, service := range services {

		if service.TlsCredentials != "" {
			_, err := cli.CoreV1().Secrets(namespace).Get(context.TODO(), service.TlsCredentials, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("SslProfile %s for service %s does not exist in this cluster", service.TlsCredentials, service.Address)
			}
		}

		if service.TlsCertAuthority != "" {
			_, err := cli.CoreV1().Secrets(namespace).Get(context.TODO(), service.TlsCertAuthority, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("SslProfile %s for service %s does not exist in this cluster", service.TlsCertAuthority, service.Address)
			}
		}
	}
	return nil
}

func checkAndIncludeSslProfile(sslProfile qdr.SslProfile, tlsManager *TlsManager) error {
	ok, err := tlsManager.ExistsSslProfile(sslProfile.Name)

	if err != nil {
		return fmt.Errorf("Error checking if if credentials exist in the router config")
	}

	if !ok {
		return tlsManager.AddSslProfile(sslProfile)
	}

	return nil
}

func generateNewSecret(support TlsServiceSupport, tlsManager *TlsManager) (*corev1.Secret, error) {
	configmap, err := tlsManager.GetConfigMap()

	if err != nil {
		return nil, err
	}

	serviceCredential := types.Credential{
		CA:          types.ServiceCaSecret,
		Name:        support.Credentials,
		Subject:     support.Address,
		Hosts:       []string{support.Address},
		ConnectJson: false,
		Post:        false,
	}

	ownerReference := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       configmap.Name,
		UID:        configmap.UID,
	}
	serviceSecret, err := tlsManager.NewSecret(serviceCredential, &ownerReference)

	if err != nil {
		return nil, err
	}

	return serviceSecret, nil
}

func isGeneratedBySkupper(credentials string) bool {
	return strings.HasPrefix(credentials, types.SkupperServiceCertPrefix)
}

type TlsManager struct {
	KubeClient kubernetes.Interface
	Namespace  string
}

func (manager *TlsManager) GetSecret(name string) (*corev1.Secret, error) {
	return manager.KubeClient.CoreV1().Secrets(manager.Namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (manager *TlsManager) GetConfigMap() (*corev1.ConfigMap, error) {

	result, err := manager.KubeClient.CoreV1().ConfigMaps(manager.Namespace).Get(context.TODO(), types.TransportConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (manager *TlsManager) NewSecret(credential types.Credential, ownerReference *metav1.OwnerReference) (*corev1.Secret, error) {
	return kube.NewSecret(credential, ownerReference, manager.Namespace, manager.KubeClient)
}

func (manager *TlsManager) AddSslProfile(sslProfile qdr.SslProfile) error {
	return AddSslProfile(sslProfile, manager.Namespace, manager.KubeClient)
}

func (manager *TlsManager) ExistsSslProfile(sslProfile string) (bool, error) {
	return ExistsSslProfile(sslProfile, manager.Namespace, manager.KubeClient)
}

func (manager *TlsManager) RemoveSslProfile(sslProfile string) error {
	return RemoveSslProfile(sslProfile, manager.Namespace, manager.KubeClient)
}

func (manager *TlsManager) DeleteSecret(secretName string) error {
	return manager.KubeClient.CoreV1().Secrets(manager.Namespace).Delete(context.TODO(), secretName, metav1.DeleteOptions{})
}
