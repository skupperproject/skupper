package main

import (
	"github.com/spf13/cobra"
)

func NewCmdNetwork() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network [command]",
		Short: "Show information about the sites and services included in the network.",
	}
	return cmd
}

var verboseNetworkStatus bool
var siteNameNetoworkStatus string

func NewCmdNetworkStatus(skupperClient SkupperNetworkClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status",
		Short:  "Shows information about the current site, and connected sites.",
		Args:   cobra.MaximumNArgs(1),
		PreRun: skupperClient.NewClient,
		RunE:   skupperClient.Status,
	}

	cmd.Flags().BoolVarP(&verboseNetworkStatus, "verbose", "v", false, "More detailed output about the network topology")
	cmd.Flags().StringVarP(&siteNameNetoworkStatus, "site", "s", "", "Filter by a specific site name")
	return cmd

}
