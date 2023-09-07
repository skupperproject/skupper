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

var selectedService string

func NewCmdNetworkStatus(skupperClient SkupperNetworkClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status",
		Short:  "Shows information about the current site, and connected sites.",
		Args:   cobra.MaximumNArgs(1),
		PreRun: skupperClient.NewClient,
		RunE:   skupperClient.Status,
	}

	cmd.Flags().StringVarP(&selectedService, "service", "s", "", "Service name")

	return cmd

}
