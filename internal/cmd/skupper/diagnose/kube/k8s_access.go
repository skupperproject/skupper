package kube

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/diagnose/cli"
	"github.com/skupperproject/skupper/internal/cmd/skupper/diagnose/command"
	"github.com/skupperproject/skupper/internal/kube/client"
)

var diagnoseK8sAccess = newKubeDiagnoseCommand(
	"k8s-access",
	"the Kubernetes API server is accessible",
	k8sAccessRun,
)

func NewCmdDiagnoseK8sAccess() command.Diagnose {
	return diagnoseK8sAccess
}

func k8sAccessRun(status cli.Reporter, kubeClient *client.KubeClient) error {
	// We use this as a proxy for access to the Kubernetes API
	_, err := kubeClient.Kube.Discovery().ServerVersion()
	if err != nil {
		return status.Error(err, "The Kubernetes API server is not accessible")
	}

	return nil
}
