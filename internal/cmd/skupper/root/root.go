package root

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/connector"
	"github.com/skupperproject/skupper/internal/cmd/skupper/link"
	"github.com/skupperproject/skupper/internal/cmd/skupper/listener"
	"github.com/skupperproject/skupper/internal/cmd/skupper/site"
	"github.com/skupperproject/skupper/internal/cmd/skupper/token"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/spf13/cobra"
)

type SkupperCommand interface {
	NewClient(cobraCommand *cobra.Command, args []string)
	AddFlags()
	ValidateInput(args []string) []error
	InputToOptions()
	Run() error
	WaitUntilReady() error
}

var SelectedNamespace string
var SelectedContext string

func NewSkupperRootCommand() *cobra.Command {

	rootCmd := &cobra.Command{
		Use:   "skupper",
		Short: "Skupper is a tool for secure, cross-cluster Kubernetes communication",
		Long: `Skupper is an open-source tool that enables secure communication across clusters with no VPNs or special firewall rules.
For more information visit https://skupperproject.github.io/refdog/index.html`,
	}

	rootCmd.AddCommand(site.NewCmdSite())
	rootCmd.AddCommand(token.NewCmdToken())
	rootCmd.AddCommand(listener.NewCmdListener())
	rootCmd.AddCommand(link.NewCmdLink())
	rootCmd.AddCommand(connector.NewCmdConnector())

	rootCmd.PersistentFlags().StringVarP(&SelectedNamespace, "namespace", "n", "", "Set the namespace")
	rootCmd.PersistentFlags().StringVarP(&SelectedContext, "context", "c", "", "Set the kubeconfig context")
	rootCmd.PersistentFlags().StringVarP(&config.Platform, "platform", "p", "", "Set the platform type to use [kubernetes, podman]")

	return rootCmd
}
