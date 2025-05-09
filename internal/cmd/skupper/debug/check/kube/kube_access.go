package kube

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/check/cli"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/check/command"
	"github.com/skupperproject/skupper/internal/kube/client"
)

var checkK8sAccess = newKubeCheckCommand(
	"kube-access",
	"the Kubernetes API server is accessible",
	kubeAccessRun,
)

func NewCmdCheckK8sAccess() command.Check {
	return checkK8sAccess
}

func kubeAccessRun(status cli.Reporter, kubeClient *client.KubeClient) error {
	// We use this as a proxy for access to the Kubernetes API
	_, err := kubeClient.Kube.Discovery().ServerVersion()
	if err != nil {
		return status.Error(err, "The Kubernetes API server is not accessible")
	}

	return nil
}
