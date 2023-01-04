package kube

import (
	appv1 "github.com/openshift/api/apps/v1"
	appsv1client "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDeploymentConfig(name string, namespace string, appsClient *appsv1client.AppsV1Client) (*appv1.DeploymentConfig, error) {
	depConfig, err := appsClient.DeploymentConfigs(namespace).Get(name, metav1.GetOptions{})
	return depConfig, err
}
