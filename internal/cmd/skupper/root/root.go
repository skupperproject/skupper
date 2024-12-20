package root

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/connector"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug"
	"github.com/skupperproject/skupper/internal/cmd/skupper/diagnose"
	"github.com/skupperproject/skupper/internal/cmd/skupper/link"
	"github.com/skupperproject/skupper/internal/cmd/skupper/listener"
	"github.com/skupperproject/skupper/internal/cmd/skupper/site"
	"github.com/skupperproject/skupper/internal/cmd/skupper/system"
	"github.com/skupperproject/skupper/internal/cmd/skupper/token"
	"github.com/skupperproject/skupper/internal/cmd/skupper/version"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/spf13/cobra"
)

var SelectedNamespace string
var SelectedContext string
var KubeConfigPath string

var rootCmd = &cobra.Command{
	Use:   "skupper",
	Short: "Skupper is a tool for secure, cross-cluster Kubernetes communication",
	Long: `Skupper is an open-source tool that enables secure communication across clusters with no VPNs or special firewall rules.
For more information visit https://skupperproject.github.io/refdog/index.html`,
}

func NewSkupperRootCommand() *cobra.Command {

	rootCmd.AddCommand(site.NewCmdSite())
	rootCmd.AddCommand(token.NewCmdToken())
	rootCmd.AddCommand(listener.NewCmdListener())
	rootCmd.AddCommand(link.NewCmdLink())
	rootCmd.AddCommand(connector.NewCmdConnector())
	rootCmd.AddCommand(version.NewCmdVersion())
	rootCmd.AddCommand(debug.NewCmdDebug())
	rootCmd.AddCommand(system.NewCmdSystem())
	rootCmd.AddCommand(diagnose.NewCmdDiagnose())

	return rootCmd
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&config.Platform, common.FlagNamePlatform, "p", "", common.FlagDescPlatform)
	rootCmd.PersistentFlags().StringVarP(&SelectedNamespace, common.FlagNameNamespace, "n", "", common.FlagDescNamespace)
	rootCmd.PersistentFlags().StringVarP(&SelectedContext, common.FlagNameContext, "c", "", common.FlagDescContext)
	rootCmd.PersistentFlags().StringVarP(&KubeConfigPath, common.FlagNameKubeconfig, "", "", common.FlagDescKubeconfig)

	if config.GetPlatform() != types.PlatformKubernetes {
		rootCmd.PersistentFlags().MarkHidden(common.FlagNameContext)
		rootCmd.PersistentFlags().MarkHidden(common.FlagNameKubeconfig)
	}
}
