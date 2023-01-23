package kube

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateRole(namespace string, role *rbacv1.Role, kubeclient kubernetes.Interface) (*rbacv1.Role, error) {
	roles := kubeclient.RbacV1().Roles(namespace)
	created, err := roles.Create(context.TODO(), role, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	} else {
		return created, nil
	}
}

func UpdateRole(namespace string, name string, rules []rbacv1.PolicyRule, kubeclient kubernetes.Interface) error {
	role, err := kubeclient.RbacV1().Roles(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	role.Rules = rules
	_, err = kubeclient.RbacV1().Roles(namespace).Update(context.TODO(), role, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func CopyRole(src string, dest string, namespace string, kubeclient kubernetes.Interface) error {
	original, err := kubeclient.RbacV1().Roles(namespace).Get(context.TODO(), src, metav1.GetOptions{})
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
	_, err = kubeclient.RbacV1().Roles(namespace).Create(context.TODO(), role, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}
