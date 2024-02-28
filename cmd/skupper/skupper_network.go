package main

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils/formatter"
	"github.com/spf13/cobra"
	"strings"
)

func NewCmdNetwork() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network [command]",
		Short: "Show information about the sites and services included in the network.",
	}
	return cmd
}

var verboseNetworkStatus bool
var siteNetworkStatus string

func NewCmdNetworkStatus(skupperClient SkupperNetworkClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status",
		Short:  "Shows information about the current site, and connected sites.",
		Args:   cobra.MaximumNArgs(1),
		PreRun: skupperClient.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			ctx, cancel := context.WithTimeout(context.Background(), types.DefaultTimeoutDuration)
			defer cancel()

			currentSiteId, err := skupperClient.GetCurrentSiteId(ctx)
			if err != nil && strings.HasPrefix(err.Error(), "Skupper is not enabled") {
				fmt.Println(err.Error())
				return nil
			} else if err != nil {
				return err
			}
			currentNetworkStatus, errStatus := skupperClient.Status(cmd, args, ctx)
			if errStatus != nil && errStatus.Error() == "status not ready" {
				fmt.Println("Status pending...")
				return nil
			} else if errStatus != nil && strings.HasPrefix(errStatus.Error(), "Version incompatibility:") {
				fmt.Println(errStatus.Error())
				return nil
			} else if errStatus != nil {
				return errStatus
			}

			return formatter.PrintNetworkStatus(currentSiteId, currentNetworkStatus, siteNetworkStatus, verboseNetworkStatus)
		},
	}

	cmd.Flags().BoolVarP(&verboseNetworkStatus, "verbose", "v", false, "More detailed output about the network topology")
	cmd.Flags().StringVarP(&siteNetworkStatus, "site", "s", "", "Filter by a specific site name")
	return cmd

}
