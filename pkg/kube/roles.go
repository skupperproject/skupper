package kube

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateRole(namespace string, role *rbacv1.Role, kubeclient kubernetes.Interface) (*rbacv1.Role, error) {
	roles := kubeclient.RbacV1().Roles(namespace)
	created, err := roles.Create(role)
	if err != nil {
		return nil, fmt.Errorf("Failed to create role: %w", err)
	} else {
		return created, nil
	}
}

func CopyRole(src string, dest string, namespace string, kubeclient kubernetes.Interface) error {
	original, err := kubeclient.RbacV1().Roles(namespace).Get(src, metav1.GetOptions{})
	if err != nil {
		return err
	}
	role := &rbacv1.Role{
		TypeMeta: original.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:            dest,
			Annotations:     original.ObjectMeta.Annotations,
			OwnerReferences: original.ObjectMeta.OwnerReferences,
		},
		Rules: original.Rules,
	}
	_, err = kubeclient.RbacV1().Roles(namespace).Create(role)
	if err != nil {
		return err
	}
	return nil
}
