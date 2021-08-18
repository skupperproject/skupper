package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

func NewCmdNetwork() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network status",
		Short: "Check incoming and outgoing links.",
	}
	return cmd
}

func NewCmdNetworkStatus(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status",
		Short:  "Check incoming and outgoing links.",
		Args:   cobra.MaximumNArgs(0),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			connectionMap, err := cli.BreakDownConnectionsList()
			if err != nil {
				fmt.Println(err)
			}

			printExistentLinks(connectionMap["in"], "incoming")
			printExistentLinks(connectionMap["out"], "outgoing")

			return nil
		},
	}

	return cmd

}

func printExistentLinks(number int, direction string) {
	if number == 0 {
		fmt.Printf("There are no %v links.\n", direction)
	} else if number == 1 {
		fmt.Printf("There is %v %v link.\n", number, direction)
	} else {
		fmt.Printf("There are %v %v links.\n", number, direction)
	}
}
