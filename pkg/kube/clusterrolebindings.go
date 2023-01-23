package kube

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
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
