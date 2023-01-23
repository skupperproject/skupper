package kube

import (
	"context"

	appv1 "github.com/openshift/api/apps/v1"
	"github.com/openshift/client-go/apps/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDeploymentConfig(name string, namespace string, appsClient versioned.Interface) (*appv1.DeploymentConfig, error) {
	depConfig, err := appsClient.AppsV1().DeploymentConfigs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	return depConfig, err
}

func GetContainerPortForDeploymentConfig(deploymentConfig *appv1.DeploymentConfig) map[int]int {
	if len(deploymentConfig.Spec.Template.Spec.Containers) > 0 && len(deploymentConfig.Spec.Template.Spec.Containers[0].Ports) > 0 {
		return GetAllContainerPorts(deploymentConfig.Spec.Template.Spec.Containers[0])
	} else {
		return map[int]int{}
	}
}
