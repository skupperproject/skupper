package link

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/link/kube"
	"github.com/spf13/cobra"
)

func NewCmdLink() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "link",
		Short: "A site-to-site communication channel",
		Long:  `A site-to-site communication channel. Links serve as a transport for application connections and requests. A set of linked sites constitute a network.`,
		Example: `skupper link export link1 ~/link1.yaml
skupper link status`,
	}

	linkExportCommand := kube.NewCmdLinkGenerate()
	linkUpdateCommand := kube.NewCmdLinkUpdate()
	linkStatusCommand := kube.NewCmdLinkStatus()
	linkDeleteCommand := kube.NewCmdLinkDelete()

	cmd.AddCommand(&linkExportCommand.CobraCmd)
	cmd.AddCommand(&linkUpdateCommand.CobraCmd)
	cmd.AddCommand(&linkDeleteCommand.CobraCmd)
	cmd.AddCommand(&linkStatusCommand.CobraCmd)

	return cmd
}
