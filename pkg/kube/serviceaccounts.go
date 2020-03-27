package kube

import (
	"fmt"

	//appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/ajssmith/skupper/api/types"
)

func NewServiceAccountWithOwner(sa types.ServiceAccount, owner metav1.OwnerReference, namespace string, cli *kubernetes.Clientset) (*corev1.ServiceAccount, error) {
	serviceaccount := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            sa.ServiceAccount,
			OwnerReferences: []metav1.OwnerReference{owner},
			Annotations:     sa.Annotations,
		},
	}
	actual, err := cli.CoreV1().ServiceAccounts(namespace).Create(serviceaccount)
	if err != nil {
		// TODO : come up with a policy for already-exists errors.
		if errors.IsAlreadyExists(err) {
			fmt.Println("Service account", sa.ServiceAccount, "already exists")
			return actual, nil
		} else {
			return actual, fmt.Errorf("Could not create service account %s : %w", sa.ServiceAccount, err)
		}
	}
	return actual, nil
}
