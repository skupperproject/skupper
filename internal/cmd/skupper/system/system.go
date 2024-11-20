package system

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/system/kube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/system/nonkube"
	"github.com/skupperproject/skupper/pkg/config"

	"github.com/spf13/cobra"
)

func NewCmdSystem() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "non-kubernetes sites are static and Custom Resources need to be provided.",
		Long: `Non-kubernetes sites can be created using the standard V2 site declaration 
approach, which is based on the new set of Custom Resource Definitions (CRDs).`,
		Example: "system start --path ./my-config-path -n my-namespace",
	}

	cmd.AddCommand(CmdSystemStartFactory(config.GetPlatform()))
	cmd.AddCommand(CmdSystemReloadFactory(config.GetPlatform()))
	cmd.AddCommand(CmdSystemStopFactory(config.GetPlatform()))
	cmd.AddCommand(CmdSystemTeardownFactory(config.GetPlatform()))

	return cmd
}

func CmdSystemStartFactory(configuredPlatform types.Platform) *cobra.Command {

	//This implementation will warn the user that the command is not available for Kubernetes environments.
	kubeCommand := kube.NewCmdSystemStart()
	nonKubeCommand := nonkube.NewCmdSystemStart()

	cmdSystemStartDesc := common.SkupperCmdDescription{
		Use:   "start",
		Short: "Create a non-kube site providing Skupper Custom Resources",
		Long: `
Create a static non-kube site providing Skupper Custom Resources

*** Note for containers: 
When running this command through a container, the /input path must be mapped 
to a directory containing a site definition based on CR files, if an input 
path has been provided. The /output path must be mapped to the Host's 
XDG_DATA_HOME/skupper or $HOME/.local/share/skupper (non-root) and 
/usr/local/share/skupper (root).
`,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSystemStartDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandSystemStartFlags{}

	cmd.Flags().StringVar(&cmdFlags.Path, common.FlagNamePath, "", common.FlagDescPath)
	cmd.Flags().StringVarP(&cmdFlags.Strategy, common.FlagNameStrategy, "b", "", common.FlagDescStrategy)
	cmd.Flags().BoolVarP(&cmdFlags.Force, common.FlagNameForce, "f", false, common.FlagDescForce)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}

func CmdSystemReloadFactory(configuredPlatform types.Platform) *cobra.Command {

	//This implementation will warn the user that the command is not available for Kubernetes environments.
	kubeCommand := kube.NewCmdSystemReload()
	nonKubeCommand := nonkube.NewCmdSystemReload()

	cmdSystemReloadDesc := common.SkupperCmdDescription{
		Use:     "reload",
		Short:   "Forces to overwrite an existing namespace based on input/resources",
		Long:    "Forces to overwrite an existing namespace based on input/resources, if the namespace is not provided, the default one is going to be reloaded",
		Example: "skupper system reload -n my-namespace",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSystemReloadDesc, kubeCommand, nonKubeCommand)

	kubeCommand.CobraCmd = cmd
	nonKubeCommand.CobraCmd = cmd

	return cmd
}

func CmdSystemStopFactory(configuredPlatform types.Platform) *cobra.Command {

	//This implementation will warn the user that the command is not available for Kubernetes environments.
	kubeCommand := kube.NewCmdSystemStop()
	nonKubeCommand := nonkube.NewCmdSystemStop()

	cmdSystemStopDesc := common.SkupperCmdDescription{
		Use:     "stop",
		Short:   "Shut down the Skupper components for the current site",
		Long:    "Shut down the Skupper components for the current site",
		Example: "skupper system stop -n my-namespace",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSystemStopDesc, kubeCommand, nonKubeCommand)

	return cmd
}

func CmdSystemTeardownFactory(configuredPlatform types.Platform) *cobra.Command {

	//This implementation will warn the user that the command is not available for Kubernetes environments.
	kubeCommand := kube.NewCmdSystemTeardown()
	nonKubeCommand := nonkube.NewCmdSystemTeardown()

	cmdSystemTeardownDesc := common.SkupperCmdDescription{
		Use:     "teardown",
		Short:   "Remove the Skupper components and resources from the from the current namespace",
		Long:    "Remove the Skupper components and resources from the current namespace",
		Example: "skupper system teardown -n my-namespace",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSystemTeardownDesc, kubeCommand, nonKubeCommand)

	return cmd
}
