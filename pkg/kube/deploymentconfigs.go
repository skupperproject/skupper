package kube

import (
	appv1 "github.com/openshift/api/apps/v1"
	"github.com/openshift/client-go/apps/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDeploymentConfig(name string, namespace string, appsClient versioned.Interface) (*appv1.DeploymentConfig, error) {
	depConfig, err := appsClient.AppsV1().DeploymentConfigs(namespace).Get(name, metav1.GetOptions{})
	return depConfig, err
}
