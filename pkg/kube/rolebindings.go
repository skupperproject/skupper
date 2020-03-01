package kube

import (
	"fmt"

	//appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	"github.com/ajssmith/skupper/api/types"
)

func NewRoleBindingWithOwner(rb types.RoleBinding, owner metav1.OwnerReference, namespace string, kubeclient *kubernetes.Clientset) *rbacv1.RoleBinding {
	name := rb.ServiceAccount + "-" + rb.Role
	rolebinding := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		Subjects: []rbacv1.Subject{{
			Kind: "ServiceAccount",
			Name: rb.ServiceAccount,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: rb.Role,
		},
	}
	actual, err := kubeclient.RbacV1().RoleBindings(namespace).Create(rolebinding)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			fmt.Println("Role binding", name, "already exists")
		} else {
			fmt.Println("Could not create role binding", name, ":", err)
		}

	}
	return actual
}
