package version

import (
	"fmt"

	"github.com/skupperproject/skupper/internal/version"
	"github.com/spf13/cobra"
)

func NewCmdVersion() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display the Skupper CLI version.",
		Long:  "Report the version of the Skupper CLI binary.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version.Version)
		},
	}

	return cmd
}
