package kube

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"

	"github.com/ajssmith/skupper/api/types"
)

func NewRoleWithOwner(newrole types.Role, owner metav1.OwnerReference, namespace string, kubeclient *kubernetes.Clientset) (*rbacv1.Role, error) {
	role := &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            newrole.Name,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		Rules: newrole.Rules,
	}
	actual, err := kubeclient.RbacV1().Roles(namespace).Create(role)
	if err != nil {
		return nil, fmt.Errorf("Could not create role: %w", err)
	}
	return actual, nil
}
