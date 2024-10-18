package diagnose

import (
	"errors"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/cmd/skupper/diagnose/cli"
	"github.com/skupperproject/skupper/internal/cmd/skupper/diagnose/command"
	"github.com/skupperproject/skupper/internal/cmd/skupper/diagnose/kube"
	"github.com/spf13/cobra"
	"k8s.io/utils/ptr"
)

type cmdDiagnose struct {
	cmd          *cobra.Command
	kube         bool
	kubeCommands []*command.Diagnose
}

type empty struct{}

func NewCmdDiagnose() *cobra.Command {

	diagnoseCmd := cmdDiagnose{
		kubeCommands: []*command.Diagnose{},
	}

	diagnoseCmd.cmd = &cobra.Command{
		Use:     "diagnose",
		Short:   "Run diagnostics",
		Long:    `Runs diagnostics to identify potential issues in the environment hosting Skupper`,
		Example: `skupper diagnose --kube`,
		Run: func(cmd *cobra.Command, args []string) {
			utils.HandleError(diagnoseCmd.Run(cmd, args))
		},
	}

	diagnoseCmd.cmd.Flags().BoolVar(&diagnoseCmd.kube, "kube", false, "run all Kubernetes diagnostics")

	diagnoseCmd.kubeCommands = append(diagnoseCmd.kubeCommands, ptr.To(kube.NewCmdDiagnoseK8sAccess()))
	diagnoseCmd.kubeCommands = append(diagnoseCmd.kubeCommands, ptr.To(kube.NewCmdDiagnoseK8sVersion()))
	for i := range diagnoseCmd.kubeCommands {
		subCommand := *diagnoseCmd.kubeCommands[i]
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
		cmd.Example = "skupper diagnose " + subCommand.Name()
		diagnoseCmd.cmd.AddCommand(cmd)
	}

	return diagnoseCmd.cmd
}

func (c cmdDiagnose) Run(cmd *cobra.Command, args []string) error {
	if !c.kube {
		return errors.New("please specify which diagnose sub-command to run, or a subsystem such as --kube")
	}

	// Run all available sub-commands, in dependency order (falling back to declaration order)
	// In the map of processed dependencies, true indicates that the command previously ran successfully,
	// false that it failed previously
	processedDependencies := make(map[string]bool)

	status := cli.NewReporter()
	defer status.End()

	for i := range c.kubeCommands {
		subCommand := *c.kubeCommands[i]
		if _, seen := processedDependencies[subCommand.Name()]; seen {
			// The command has already been run as a dependency, skip it
			continue
		}
		_ = runCommandWithDeps(status, subCommand, processedDependencies, cmd)
	}

	// For UX consistency, errors are handled internally
	return nil
}

func runCommandWithDeps(status cli.Reporter, dc command.Diagnose, processed map[string]bool, cmd *cobra.Command) error {
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
