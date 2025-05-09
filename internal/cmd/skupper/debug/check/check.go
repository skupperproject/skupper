package check

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/check/cli"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/check/command"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/check/kube"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/spf13/cobra"
	"k8s.io/utils/ptr"
)

type cmdCheck struct {
	cmd              *cobra.Command
	platformCommands map[types.Platform][]*command.Check
}

func NewCmdCheck() *cobra.Command {
	checkCmd := cmdCheck{
		platformCommands: map[types.Platform][]*command.Check{},
	}

	checkCmd.cmd = &cobra.Command{
		Use:     "check",
		Short:   "Run diagnostics",
		Long:    `Runs diagnostics to identify potential issues in the environment hosting Skupper`,
		Example: `skupper debug check -p kubernetes`,
		Run: func(cmd *cobra.Command, args []string) {
			utils.HandleError(utils.GenericError, checkCmd.Run(cmd, args))
		},
	}

	checkCmd.registerCommand(types.PlatformKubernetes, ptr.To(kube.NewCmdCheckK8sAccess()))
	checkCmd.registerCommand(types.PlatformKubernetes, ptr.To(kube.NewCmdCheckK8sVersion()))
	for _, cmds := range checkCmd.platformCommands {
		for i := range cmds {
			subCommand := *cmds[i]
			cmd := &cobra.Command{
				Use:   subCommand.Name(),
				Short: "check that " + subCommand.CheckDescription(),
				Run: func(cmd *cobra.Command, args []string) {
					status := cli.NewReporter()
					defer status.End()
					runCommandWithDeps(status, subCommand, map[string]bool{}, cmd)
				},
			}
			// TODO Adjust "skupper" to args[0]
			cmd.Example = "skupper debug check " + subCommand.Name()
			checkCmd.cmd.AddCommand(cmd)
		}
	}

	return checkCmd.cmd
}

func (c *cmdCheck) registerCommand(platform types.Platform, cmd *command.Check) {
	c.platformCommands[platform] = append(c.platformCommands[platform], cmd)
}

func (c cmdCheck) Run(cmd *cobra.Command, args []string) error {
	platform := config.GetPlatform()

	// Run all available sub-commands, in dependency order (falling back to declaration order)
	// In the map of processed dependencies, true indicates that the command previously ran successfully,
	// false that it failed previously
	processedDependencies := make(map[string]bool)

	status := cli.NewReporter()
	defer status.End()

	for i := range c.platformCommands[platform] {
		subCommand := *c.platformCommands[platform][i]
		if _, seen := processedDependencies[subCommand.Name()]; seen {
			// The command has already been run as a dependency, skip it
			continue
		}
		_ = runCommandWithDeps(status, subCommand, processedDependencies, cmd)
	}

	// For UX consistency, errors are handled internally
	return nil
}

func runCommandWithDeps(status cli.Reporter, dc command.Check, processed map[string]bool, cmd *cobra.Command) error {
	dependencies := dc.Dependencies()
	for i := range dependencies {
		dependency := *dependencies[i]
		if succeeded, seen := processed[dependency.Name()]; seen {
			if succeeded {
				// The command previously succeeded, skip it but continue
				continue
			}
			// The command previously failed, stop (assuming that the previous run reported the error)
			return nil
		}
		if err := runCommandWithDeps(status, dependency, processed, cmd); err != nil {
			return err
		}
	}

	status.Start("Checking that " + dc.CheckDescription())
	if err := dc.Run(status, cmd); err != nil {
		processed[dc.Name()] = false
		return err
	}

	processed[dc.Name()] = true
	status.Success("")
	return nil
}
