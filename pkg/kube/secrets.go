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

func NewCertAuthorityWithOwner(ca types.CertAuthority, owner metav1.OwnerReference, namespace string, cli *kubernetes.Clientset) *corev1.Secret {

	existing, err := cli.CoreV1().Secrets(namespace).Get(ca.Name, metav1.GetOptions{})
	if err == nil {
		fmt.Println("CA", ca.Name, "already exists")
		return existing
	} else if errors.IsNotFound(err) {
		newca := certs.GenerateCASecret(ca.Name, ca.Name)
		newca.ObjectMeta.OwnerReferences = []metav1.OwnerReference{owner}
		_, err := cli.CoreV1().Secrets(namespace).Create(&newca)
		if err == nil {
			return &newca
		} else {
			fmt.Println("Failed to create CA", ca.Name, ": ", err.Error())
		}
	} else {
		fmt.Println("Failed to check CA", ca.Name, ": ", err.Error())
	}
	return nil

}

func NewSecretWithOwner(cred types.Credential, owner metav1.OwnerReference, namespace string, cli *kubernetes.Clientset) (*corev1.Secret, error) {
	caSecret, err := cli.CoreV1().Secrets(namespace).Get(cred.CA, metav1.GetOptions{})
	if err != nil {
		fmt.Println("Failed to retrieve CA", err.Error())
		return nil, err
	}
	secret := certs.GenerateSecret(cred.Name, cred.Subject, cred.Hosts, caSecret)
	if cred.ConnectJson {
		secret.Data["connect.json"] = []byte(configs.ConnectJson())
	}
	secret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{owner}
	_, err = cli.CoreV1().Secrets(namespace).Create(&secret)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			fmt.Println("Secret already exists: ", cred.Name)
		} else {
			fmt.Println("Could not create secret: ", err.Error())
		}
		return nil, err
	}

	return &secret, nil
}

func DeleteSecret(name string, namespace string, cli *kubernetes.Clientset) error {
	secrets := cli.CoreV1().Secrets(namespace)
	err := secrets.Delete(name, &metav1.DeleteOptions{})
	if err == nil {
		fmt.Println("Secret", name, "deleted")
	} else if errors.IsNotFound(err) {
		fmt.Println("Secret", name, "does not exist")
	} else {
		fmt.Println("Failed to delete secret: ", err.Error())
	}
	return err
}
