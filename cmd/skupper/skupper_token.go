package main

import (
	"time"

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

var tokenType string
var password string
var expiry time.Duration
var uses int
var tokenTemplate string

func NewCmdTokenCreate(skupperClient SkupperTokenClient, flag string) *cobra.Command {
	subflag := ""
	if flag == "client-identity" {
		subflag = "i"
	} else if flag == "" {
		flag = "name" // default
	} else {
		panic("flag argument must be \"client-identity\" or \"\"")
	}
	cmd := &cobra.Command{
		Use:    "create <output-token-file>",
		Short:  "Create a token.  The 'link create' command uses the token to establish a link from a remote Skupper site.",
		Args:   cobra.ExactArgs(1),
		PreRun: skupperClient.NewClient,
		RunE:   skupperClient.Create,
	}
	cmd.Flags().StringVarP(&clientIdentity, flag, subflag, types.DefaultVanName, "Provide a specific identity as which connecting skupper installation will be authenticated")
	skupperClient.CreateFlags(cmd)
	return cmd
}
