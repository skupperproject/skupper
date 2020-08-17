package kube

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
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
