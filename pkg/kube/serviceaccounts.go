package kube

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateServiceAccount(namespace string, sa *corev1.ServiceAccount, cli kubernetes.Interface) (*corev1.ServiceAccount, error) {
	serviceAccounts := cli.CoreV1().ServiceAccounts(namespace)
	created, err := serviceAccounts.Create(sa)
	if err != nil {
		return nil, fmt.Errorf("Failed to create service account: %w", err)
	} else {
		return created, nil
	}
}

func CopyServiceAccount(src string, dest string, annotations map[string]string, namespace string, kubeclient kubernetes.Interface) error {
	original, err := kubeclient.CoreV1().ServiceAccounts(namespace).Get(src, metav1.GetOptions{})
	if err != nil {
		return err
	}
	serviceAccount := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            dest,
			Annotations:     map[string]string{},
			OwnerReferences: original.ObjectMeta.OwnerReferences,
		},
	}
	for key, value := range original.ObjectMeta.Annotations {
		if alternative, ok := annotations[key]; ok {
			serviceAccount.ObjectMeta.Annotations[key] = alternative
		} else {
			serviceAccount.ObjectMeta.Annotations[key] = value
		}
	}
	_, err = kubeclient.CoreV1().ServiceAccounts(namespace).Create(serviceAccount)
	if err != nil {
		return err
	}
	return nil
}
