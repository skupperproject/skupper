package main

import (
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/spf13/cobra"

	"github.com/skupperproject/skupper/api/types"
)

func NewCmdLink() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link create <input-token-file> [--name <name>] or link delete ...",
		Short: "Manage skupper links definitions",
	}
	return cmd
}

var connectorCreateOpts types.ConnectorCreateOptions

func NewCmdLinkCreate(skupperClient SkupperLinkClient, flag string) *cobra.Command {

	if flag == "" { // hack for backwards compatibility
		flag = "name"
	}

	cmd := &cobra.Command{
		Use:    "create <input-token-file>",
		Short:  "Links this skupper site to the site that issued the token",
		Args:   cobra.ExactArgs(1),
		PreRun: skupperClient.NewClient,
		RunE:   skupperClient.Create,
	}
	cmd.Flags().StringVarP(&connectorCreateOpts.Name, flag, "", "", "Provide a specific name for the link (used when deleting it)")
	cmd.Flags().Int32VarP(&connectorCreateOpts.Cost, "cost", "", 1, "Specify a cost for this link.")

	return cmd
}

var connectorRemoveOpts types.ConnectorRemoveOptions

func NewCmdLinkDelete(skupperClient SkupperLinkClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "delete <name>",
		Short:  "Remove specified link",
		Args:   cobra.ExactArgs(1),
		PreRun: skupperClient.NewClient,
		RunE:   skupperClient.Delete,
	}

	return cmd
}

var waitFor int

func allConnected(links []types.LinkStatus) bool {
	for _, l := range links {
		if !l.Connected {
			return false
		}
	}
	return true
}

func NewCmdLinkStatus(skupperClient SkupperLinkClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status [<link-name>]",
		Short:  "Check whether a link to another Skupper site is active",
		Args:   cobra.MaximumNArgs(1),
		PreRun: skupperClient.NewClient,
		RunE:   skupperClient.Status,
	}
	cmd.Flags().IntVar(&waitFor, "wait", 0, "The number of seconds to wait for links to become active")

	return cmd

}
