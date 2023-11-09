package kube

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateClusterRole(clusterRole *rbacv1.ClusterRole, kubeclient kubernetes.Interface) (*rbacv1.ClusterRole, error) {
	clusterRoles := kubeclient.RbacV1().ClusterRoles()
	created, err := clusterRoles.Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	} else {
		return created, nil
	}
}

func UpdateClusterRole(name string, rules []rbacv1.PolicyRule, kubeclient kubernetes.Interface) error {
	clusterRole, err := kubeclient.RbacV1().ClusterRoles().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	clusterRole.Rules = rules
	_, err = kubeclient.RbacV1().ClusterRoles().Update(context.TODO(), clusterRole, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func DeleteClusterRole(name string, kubeclient kubernetes.Interface) (bool, error) {
	err := kubeclient.RbacV1().ClusterRoles().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	} else {
		return true, nil
	}
}

func CopyClusterRole(src string, dest string, kubeclient kubernetes.Interface) error {
	original, err := kubeclient.RbacV1().ClusterRoles().Get(context.TODO(), src, metav1.GetOptions{})
	if err != nil {
		return err
	}
	clusterRole := &rbacv1.ClusterRole{
		TypeMeta: original.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:            dest,
			Annotations:     original.ObjectMeta.Annotations,
			OwnerReferences: original.ObjectMeta.OwnerReferences,
		},
		Rules: original.Rules,
	}
	_, err = kubeclient.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}
