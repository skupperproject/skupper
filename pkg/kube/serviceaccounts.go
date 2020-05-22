package kube

import (
	"fmt"

	//appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/skupperproject/skupper/api/types"
)

func NewServiceAccount(sa types.ServiceAccount, owner *metav1.OwnerReference, namespace string, cli kubernetes.Interface) (*corev1.ServiceAccount, error) {
	serviceAccounts := cli.CoreV1().ServiceAccounts(namespace)
	existing, err := serviceAccounts.Get(sa.ServiceAccount, metav1.GetOptions{})
	if err == nil {
		//TODO: already exists
		return existing, nil
	} else if errors.IsNotFound(err) {
		sa := &corev1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ServiceAccount",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        sa.ServiceAccount,
				Annotations: sa.Annotations,
			},
		}
		if owner != nil {
			sa.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
				*owner,
			}
		}

		created, err := serviceAccounts.Create(sa)

		if err != nil {
			return nil, fmt.Errorf("Failed to create service account: %w", err)
		} else {
			return created, nil
		}
	} else {
		sa := &corev1.ServiceAccount{}
		return sa, fmt.Errorf("Failed to check service account: %w", err)
	}
}
