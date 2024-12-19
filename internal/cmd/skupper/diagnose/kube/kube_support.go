package kube

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/diagnose/cli"
	"github.com/skupperproject/skupper/internal/cmd/skupper/diagnose/command"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/spf13/cobra"
)

type KubeDiagnose struct {
	command.BaseDiagnose
	diagnostic func(cli.Reporter, *client.KubeClient) error
}

func newKubeDiagnoseCommand(
	name,
	shortDescription string,
	cmd func(cli.Reporter, *client.KubeClient) error,
	dependencies ...*command.Diagnose,
) command.Diagnose {
	return &KubeDiagnose{
		BaseDiagnose: command.NewBaseDiagnoseCommand(name, shortDescription, dependencies...),
		diagnostic:   cmd,
	}
}

func (kd *KubeDiagnose) Run(status cli.Reporter, cmd *cobra.Command) error {
	newClient, err := client.NewClient(cmd.Flag("namespace").Value.String(), cmd.Flag("context").Value.String(), cmd.Flag("kubeconfig").Value.String())
	if err != nil {
		return status.Error(err, "failed to obtain a Kubernetes client")
	}

	return kd.diagnostic(status, newClient)
}
