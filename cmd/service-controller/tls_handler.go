package main

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type GetSecretFunc func(string) (*corev1.Secret, error)
type GetConfigMapFunc func() (*corev1.ConfigMap, error)
type NewSecretFunc func(types.Credential, *metav1.OwnerReference) (*corev1.Secret, error)
type DeleteSecretFunc func(string) error
type AddSslProfileFunc func(string) error
type ExistsSslProfileFunc func(string) (bool, error)
type RemoveSslProfileFunc func(string) error

type TlsSupport struct {
	address       string
	credentials   string
	certAuthority string
}

func EnableTlsSupport(support TlsSupport, getSecret GetSecretFunc, getConfigMap GetConfigMapFunc, newSecret NewSecretFunc, addSslProfile AddSslProfileFunc, existsSslProfile ExistsSslProfileFunc) error {

	if support.credentials != "" {
		if isGeneratedBySkupper(support.credentials) {
			// If the requested certificate is one generated by skupper it can be generated in other sites as well
			_, err := getSecret(support.credentials)
			if err != nil {
				serviceSecret, err := generateNewSecret(support, getConfigMap, newSecret)
				if err != nil {
					return err
				}

				return includeSslProfile(serviceSecret.Name, addSslProfile)
			}
		} else {
			_, err := getSecret(support.credentials)
			if err != nil {
				return fmt.Errorf("The secret %s is missing", support.credentials)
			}

			err = checkAndIncludeSslProfile(support.credentials, addSslProfile, existsSslProfile)
			if err != nil {
				return err
			}
		}
	}

	if support.certAuthority != "" && support.certAuthority != types.ServiceClientSecret {
		_, err := getSecret(support.certAuthority)
		if err != nil {
			return fmt.Errorf("The secret %s is missing", support.certAuthority)
		}

		err = checkAndIncludeSslProfile(support.certAuthority, addSslProfile, existsSslProfile)
		if err != nil {
			return err
		}
	}

	return nil
}

func DisableTlsSupport(support TlsSupport, removeSslProfile RemoveSslProfileFunc, getSecret GetSecretFunc, deleteSecret DeleteSecretFunc) error {

	if len(support.credentials) > 0 {
		err := removeSslProfile(support.credentials)
		if err != nil {
			return err
		}

		if isGeneratedBySkupper(support.credentials) {
			_, err = getSecret(support.credentials)
			if err == nil {
				err = deleteSecret(support.credentials)
				if err != nil {
					return err
				}
			}
		}
	}

	//skupper-service-client profile is used by more than one connector, thus it can't be deleted
	if len(support.certAuthority) > 0 && support.certAuthority != types.ServiceClientSecret {
		err := removeSslProfile(support.certAuthority)
		if err != nil {
			return err
		}
	}

	return nil
}

func includeSslProfile(sslProfile string, addSslProfile AddSslProfileFunc) error {
	errAddSslProfile := addSslProfile(sslProfile)
	if errAddSslProfile != nil {
		return errAddSslProfile
	}
	return nil
}

func checkAndIncludeSslProfile(sslProfile string, addSsl AddSslProfileFunc, checkIfExists ExistsSslProfileFunc) error {
	ok, err := checkIfExists(sslProfile)

	if err != nil {
		return fmt.Errorf("Error checking if if credentials exist in the router config")
	}

	if !ok {
		return includeSslProfile(sslProfile, addSsl)
	}

	return nil
}

func generateNewSecret(support TlsSupport, getConfigMap GetConfigMapFunc, newSecret NewSecretFunc) (*corev1.Secret, error) {
	configmap, err := getConfigMap()

	if err != nil {
		return nil, err
	}

	serviceCredential := types.Credential{
		CA:          types.ServiceCaSecret,
		Name:        support.credentials,
		Subject:     support.address,
		Hosts:       []string{support.address},
		ConnectJson: false,
		Post:        false,
	}

	ownerReference := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       configmap.Name,
		UID:        configmap.UID,
	}
	serviceSecret, err := newSecret(serviceCredential, &ownerReference)

	if err != nil {
		return nil, err
	}

	return serviceSecret, nil
}

func isGeneratedBySkupper(credentials string) bool {
	return strings.HasPrefix(credentials, types.SkupperServiceCertPrefix)
}
