package root

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/connector"
	"github.com/skupperproject/skupper/internal/cmd/skupper/link"
	"github.com/skupperproject/skupper/internal/cmd/skupper/listener"
	"github.com/skupperproject/skupper/internal/cmd/skupper/site"
	"github.com/skupperproject/skupper/internal/cmd/skupper/token"
	"github.com/spf13/cobra"
)

type SkupperCommand interface {
	NewClient(cobraCommand *cobra.Command, args []string)
	AddFlags()
	ValidateFlags() []error
	FlagsToOptions() error
	Run() error
	WaitUntilReady() bool
}

func NewSkupperRootCommand() *cobra.Command {

	rootCmd := &cobra.Command{
		Use:   "skupper",
		Short: "Skupper is a tool for secure, cross-cluster Kubernetes communication",
		Long: `Skupper is an open-source tool that enables secure communication across clusters with no VPNs or special firewall rules.
For more information visit https://skupper.io`,
	}

	rootCmd.AddCommand(site.NewCmdSite())
	rootCmd.AddCommand(token.NewCmdToken())
	rootCmd.AddCommand(listener.NewCmdListener())
	rootCmd.AddCommand(link.NewCmdLink())
	rootCmd.AddCommand(connector.NewCmdConnector())

	return rootCmd
}
