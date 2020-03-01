package kube

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	"github.com/ajssmith/skupper/api/types"
)

func NewRoleWithOwner(newrole types.Role, owner metav1.OwnerReference, namespace string, kubeclient *kubernetes.Clientset) *rbacv1.Role {
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
		if errors.IsAlreadyExists(err) {
			fmt.Println("Role", newrole.Name, "already exists")
		} else {
			fmt.Println("Could not create role", newrole.Name, ":", err)
		}

	}
	return actual
}
