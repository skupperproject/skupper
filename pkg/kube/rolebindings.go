package kube

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateRoleBinding(namespace string, rb *rbacv1.RoleBinding, kubeclient kubernetes.Interface) (*rbacv1.RoleBinding, error) {
	roleBindings := kubeclient.RbacV1().RoleBindings(namespace)
	created, err := roleBindings.Create(context.TODO(), rb, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	} else {
		return created, nil
	}
}
