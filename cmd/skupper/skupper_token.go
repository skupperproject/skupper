package main

import (
	"context"
	"fmt"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/spf13/cobra"

	"github.com/skupperproject/skupper/api/types"
)

func NewCmdToken() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token create <output-token-file> [--name <name>] or link delete ...",
		Short: "Manage skupper tokens",
	}
	return cmd
}

func NewCmdTokenCreate(newClient cobraFunc, flag string) *cobra.Command {
	subflag := "i"
	if flag == "" {
		flag = "name"
		subflag = "n"
	}
	cmd := &cobra.Command{
		Use:    "create <output-token-file>",
		Short:  "Create a connection token.  The 'link create' command uses the token to establish a link from a remote Skupper site.",
		Args:   cobra.ExactArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			err := cli.ConnectorTokenCreateFile(context.Background(), clientIdentity, args[0])
			if err != nil {
				return fmt.Errorf("Failed to create connection token: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&clientIdentity, flag, subflag, types.DefaultVanName, "Provide a specific identity as which connecting skupper installation will be authenticated")

	return cmd
}
