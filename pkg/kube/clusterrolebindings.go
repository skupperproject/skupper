package kube

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateClusterRoleBinding(crb *rbacv1.ClusterRoleBinding, kubeclient kubernetes.Interface) (*rbacv1.ClusterRoleBinding, error) {
	clusterRoleBindings := kubeclient.RbacV1().ClusterRoleBindings()
	created, err := clusterRoleBindings.Create(context.TODO(), crb, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	} else {
		return created, nil
	}
}

func DeleteClusterRoleBinding(name string, kubeclient kubernetes.Interface) (bool, error) {
	err := kubeclient.RbacV1().ClusterRoleBindings().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}
