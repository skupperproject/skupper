package kube

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateRoleBinding(namespace string, rb *rbacv1.RoleBinding, kubeclient kubernetes.Interface) (*rbacv1.RoleBinding, error) {
	roleBindings := kubeclient.RbacV1().RoleBindings(namespace)
	created, err := roleBindings.Create(rb)
	if err != nil {
		return nil, err
	} else {
		return created, nil
	}
}
