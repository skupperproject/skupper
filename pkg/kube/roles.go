package kube

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	"github.com/skupperproject/skupper/api/types"
)

func NewRole(newrole types.Role, owner *metav1.OwnerReference, namespace string, kubeclient kubernetes.Interface) (*rbacv1.Role, error) {
	roles := kubeclient.RbacV1().Roles(namespace)
	existing, err := roles.Get(newrole.Name, metav1.GetOptions{})
	if err == nil {
		//TODO: already exists
		return existing, nil
	} else if errors.IsNotFound(err) {
		role := &rbacv1.Role{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "Role",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: newrole.Name,
			},
			Rules: newrole.Rules,
		}
		if owner != nil {
			role.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
				*owner,
			}
		}
		created, err := roles.Create(role)

		if err != nil {
			return nil, fmt.Errorf("Failed to crate role: %w", err)
		} else {
			return created, nil
		}
	} else {
		role := &rbacv1.Role{}
		return role, fmt.Errorf("Failed to check existing config maps: %w", err)
	}
}
