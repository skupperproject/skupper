package kube

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
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
