package kube

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/ajssmith/skupper/api/types"
	"github.com/ajssmith/skupper/pkg/certs"
	"github.com/ajssmith/skupper/pkg/utils/configs"
)

func NewCertAuthorityWithOwner(ca types.CertAuthority, owner metav1.OwnerReference, namespace string, cli *kubernetes.Clientset) (*corev1.Secret, error) {

	existing, err := cli.CoreV1().Secrets(namespace).Get(ca.Name, metav1.GetOptions{})
	if err == nil {
		return existing, nil
	} else if errors.IsNotFound(err) {
		newca := certs.GenerateCASecret(ca.Name, ca.Name)
		newca.ObjectMeta.OwnerReferences = []metav1.OwnerReference{owner}
		_, err := cli.CoreV1().Secrets(namespace).Create(&newca)
		if err == nil {
			return &newca, nil
		} else {
			return nil, fmt.Errorf("Failed to create CA %s : %w", ca.Name, err)
		}
	} else {
		return nil, fmt.Errorf("Failed to check CA %s : %w", ca.Name, err)
	}
}

func NewSecretWithOwner(cred types.Credential, owner metav1.OwnerReference, namespace string, cli *kubernetes.Clientset) (*corev1.Secret, error) {
	caSecret, err := cli.CoreV1().Secrets(namespace).Get(cred.CA, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve CA: %w", err)
	}
	secret := certs.GenerateSecret(cred.Name, cred.Subject, cred.Hosts, caSecret)
	if cred.ConnectJson {
		secret.Data["connect.json"] = []byte(configs.ConnectJson())
	}
	secret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{owner}
	_, err = cli.CoreV1().Secrets(namespace).Create(&secret)
	if err != nil {
		return nil, fmt.Errorf("Could not create secret: %w", err)
	}

	return &secret, nil
}

func DeleteSecret(name string, namespace string, cli *kubernetes.Clientset) error {
	secrets := cli.CoreV1().Secrets(namespace)
	err := secrets.Delete(name, &metav1.DeleteOptions{})

	if err != nil {
		return fmt.Errorf("Failed to delete secret: %w", err)
	}
	return nil
}
