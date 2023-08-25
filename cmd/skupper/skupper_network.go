package main

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
	"time"
)

func NewCmdNetwork() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network [command]",
		Short: "Show information about the sites and services included in the network.",
	}
	return cmd
}

var selectedSite string
var networkStatusTimeout time.Duration

func NewCmdNetworkStatus(skupperClient SkupperNetworkClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status",
		Short:  "Shows information about the current site, and connected sites.",
		Args:   cobra.MaximumNArgs(1),
		PreRun: skupperClient.NewClient,
		RunE:   skupperClient.Status,
	}

	cmd.Flags().StringVarP(&selectedSite, "site", "s", "all", "Site identifier")
	cmd.Flags().DurationVar(&networkStatusTimeout, "timeout", types.DefaultTimeoutDuration, "Configurable timeout for retrieving remote information")

	return cmd

}
