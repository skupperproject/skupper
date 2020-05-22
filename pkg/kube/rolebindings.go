package kube

import (
	"fmt"

	//appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	"github.com/skupperproject/skupper/api/types"
)

func NewRoleBinding(rb types.RoleBinding, owner *metav1.OwnerReference, namespace string, kubeclient kubernetes.Interface) (*rbacv1.RoleBinding, error) {
	name := rb.ServiceAccount + "-" + rb.Role
	roleBindings := kubeclient.RbacV1().RoleBindings(namespace)
	existing, err := roleBindings.Get(name, metav1.GetOptions{})
	if err == nil {
		//TODO: already exists
		return existing, nil
	} else if errors.IsNotFound(err) {
		rolebinding := &rbacv1.RoleBinding{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "RoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
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
		if owner != nil {
			rolebinding.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
				*owner,
			}
		}

		created, err := roleBindings.Create(rolebinding)

		if err != nil {
			return nil, fmt.Errorf("Failed to create role binding: %w", err)
		} else {
			return created, nil
		}
	} else {
		rolebinding := &rbacv1.RoleBinding{}
		return rolebinding, fmt.Errorf("Failed to check role binding: %w", err)
	}
}
