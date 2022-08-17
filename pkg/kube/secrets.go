package kube

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/utils/configs"
)

func NewCertAuthority(ca types.CertAuthority, owner *metav1.OwnerReference, namespace string, cli kubernetes.Interface) (*corev1.Secret, error) {

	existing, err := cli.CoreV1().Secrets(namespace).Get(ca.Name, metav1.GetOptions{})
	if err == nil {
		return existing, nil
	} else if errors.IsNotFound(err) {
		newCA := certs.GenerateCASecret(ca.Name, ca.Name)
		if owner != nil {
			newCA.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
				*owner,
			}
		}
		_, err := cli.CoreV1().Secrets(namespace).Create(&newCA)
		if err == nil {
			return &newCA, nil
		} else {
			return nil, fmt.Errorf("Failed to create CA %s : %w", ca.Name, err)
		}
	} else {
		return nil, fmt.Errorf("Failed to check CA %s : %w", ca.Name, err)
	}
}

func NewSecret(cred types.Credential, owner *metav1.OwnerReference, namespace string, cli kubernetes.Interface) (*corev1.Secret, error) {
	var secret corev1.Secret

	if cred.CA != "" {
		caSecret, err := cli.CoreV1().Secrets(namespace).Get(cred.CA, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve CA: %w", err)
		}

		if cred.Simple {
			secret = certs.GenerateSimpleSecret(cred.Name, caSecret)
		} else {
			secret = certs.GenerateSecret(cred.Name, cred.Subject, strings.Join(cred.Hosts, ","), caSecret)
		}
		if cred.ConnectJson {
			secret.Data["connect.json"] = []byte(configs.ConnectJson(types.QualifiedServiceName(cred.Subject, namespace)))
		}
	} else {
		secret = corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: cred.Name,
			},
			Data: cred.Data,
		}
	}
	if owner != nil {
		secret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			*owner,
		}
	}
	_, err := cli.CoreV1().Secrets(namespace).Create(&secret)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			// TODO : come up with a policy for already-exists errors.
			fmt.Println("Secret already exists: ", cred.Name)
		} else {
			fmt.Println("Could not create secret: ", err.Error())
		}
		return nil, err

	}

	return &secret, nil
}

func DeleteSecret(name string, namespace string, cli kubernetes.Interface) error {
	secrets := cli.CoreV1().Secrets(namespace)
	err := secrets.Delete(name, &metav1.DeleteOptions{})
	if err == nil {
		return err
	} else if errors.IsNotFound(err) {
		return fmt.Errorf("Secret %s does not exist.", name)
	} else {
		return fmt.Errorf("Failed to delete secret: %w", err)
	}
}

func CopySecret(src string, dest string, namespace string, kubeclient kubernetes.Interface) error {
	original, err := kubeclient.CoreV1().Secrets(namespace).Get(src, metav1.GetOptions{})
	if err != nil {
		return err
	}
	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            dest,
			Annotations:     original.ObjectMeta.Annotations,
			OwnerReferences: original.ObjectMeta.OwnerReferences,
		},
		Data: original.Data,
		Type: original.Type,
	}

	_, err = kubeclient.CoreV1().Secrets(namespace).Create(&secret)
	if err != nil {
		return err
	}
	return nil

}

func RegenerateCertAuthority(name string, namespace string, cli kubernetes.Interface) (*corev1.Secret, error) {
	current, err := cli.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	regenerated := certs.GenerateCASecret(name, name)
	current.Data = regenerated.Data
	return cli.CoreV1().Secrets(namespace).Update(current)
}

func RegenerateCredentials(credential types.Credential, namespace string, ca *corev1.Secret, cli kubernetes.Interface) (*corev1.Secret, error) {
	current, err := cli.CoreV1().Secrets(namespace).Get(credential.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	regenerated := certs.GenerateSecret(credential.Name, credential.Subject, strings.Join(credential.Hosts, ","), ca)
	current.Data = regenerated.Data
	return cli.CoreV1().Secrets(namespace).Update(current)
}
