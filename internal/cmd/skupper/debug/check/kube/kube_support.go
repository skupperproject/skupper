package kube

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/check/cli"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/check/command"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/spf13/cobra"
)

type KubeCheck struct {
	command.BaseCheck
	diagnostic func(cli.Reporter, *client.KubeClient) error
}

func newKubeCheckCommand(
	name,
	shortDescription string,
	cmd func(cli.Reporter, *client.KubeClient) error,
	dependencies ...*command.Check,
) command.Check {
	return &KubeCheck{
		BaseCheck:  command.NewBaseCheckCommand(name, shortDescription, dependencies...),
		diagnostic: cmd,
	}
}

func (kd *KubeCheck) Run(status cli.Reporter, cmd *cobra.Command) error {
	newClient, err := client.NewClient(cmd.Flag("namespace").Value.String(), cmd.Flag("context").Value.String(), cmd.Flag("kubeconfig").Value.String())
	if err != nil {
		return status.Error(err, "failed to obtain a Kubernetes client")
	}

	return kd.diagnostic(status, newClient)
}
