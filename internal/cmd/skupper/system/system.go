package system

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/system/kube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/system/nonkube"
	"github.com/skupperproject/skupper/internal/config"

	"github.com/spf13/cobra"
)

var (
	systemStartDescription   = `Start the Skupper router for the current site. This starts the systemd service for the current namespace.`
	systemInstallDescription = `
Checks the local environment for required resources and configuration.
In some instances, configures the local environment. It starts the Podman/Docker API 
service if it is not already available.`
)

func NewCmdSystem() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "non-kubernetes sites are static and Custom Resources need to be provided.",
		Long: `Non-kubernetes sites can be created using the standard V2 site declaration 
approach, which is based on the new set of Custom Resource Definitions (CRDs).`,
		Example: "system setup --path ./my-config-path -n my-namespace",
	}

	platform := common.Platform(config.GetPlatform())
	cmd.AddCommand(CmdSystemStartFactory(platform))
	cmd.AddCommand(CmdSystemReloadFactory(platform))
	cmd.AddCommand(CmdSystemStopFactory(platform))
	cmd.AddCommand(CmdSystemInstallFactory(platform))
	cmd.AddCommand(CmdSystemUnInstallFactory(platform))
	cmd.AddCommand(CmdSystemGenerateBundleFactory(platform))

	return cmd
}

func CmdSystemStartFactory(configuredPlatform common.Platform) *cobra.Command {

	//This implementation will warn the user that the command is not available for Kubernetes environments.
	kubeCommand := kube.NewCmdSystemStart()
	nonKubeCommand := nonkube.NewCmdSystemStart()

	cmdSystemStartDesc := common.SkupperCmdDescription{
		Use:   "start",
		Short: "Create a non-kube site based on provided Skupper Custom Resources",
		Long:  systemStartDescription,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSystemStartDesc, kubeCommand, nonKubeCommand)

	kubeCommand.CobraCmd = cmd
	nonKubeCommand.CobraCmd = cmd

	return cmd
}

func CmdSystemReloadFactory(configuredPlatform common.Platform) *cobra.Command {

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

func CmdSystemStopFactory(configuredPlatform common.Platform) *cobra.Command {

	//This implementation will warn the user that the command is not available for Kubernetes environments.
	kubeCommand := kube.NewCmdSystemStop()
	nonKubeCommand := nonkube.NewCmdSystemStop()

	cmdSystemStopDesc := common.SkupperCmdDescription{
		Use:     "stop",
		Short:   "Remove the Skupper components and resources from the from the current namespace",
		Long:    "Stop the Skupper router for the current site. This stops the systemd service for the current namespace.",
		Example: "skupper system stop -n my-namespace",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSystemStopDesc, kubeCommand, nonKubeCommand)

	return cmd
}

func CmdSystemInstallFactory(configuredPlatform common.Platform) *cobra.Command {

	//This implementation will warn the user that the command is not available for Kubernetes environments.
	kubeCommand := kube.NewCmdSystemInstall()
	nonKubeCommand := nonkube.NewCmdSystemInstall()

	cmdSystemInstallDesc := common.SkupperCmdDescription{
		Use:   "install",
		Short: "Install local system infrastructure and configure the environment",
		Long:  systemInstallDescription,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSystemInstallDesc, kubeCommand, nonKubeCommand)

	kubeCommand.CobraCmd = cmd
	nonKubeCommand.CobraCmd = cmd

	return cmd
}

func CmdSystemUnInstallFactory(configuredPlatform common.Platform) *cobra.Command {

	//This implementation will warn the user that the command is not available for Kubernetes environments.
	kubeCommand := kube.NewCmdSystemUnInstall()
	nonKubeCommand := nonkube.NewCmdSystemUninstall()

	cmdSystemUninstallDesc := common.SkupperCmdDescription{
		Use:   "uninstall",
		Short: "Remove local system infrastructure",
		Long:  "Remove local system infrastructure, undoing the configuration changes made by skupper system install, by disabling the Podman/Docker API.",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSystemUninstallDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandSystemUninstallFlags{}

	cmd.Flags().BoolVarP(&cmdFlags.Force, common.FlagNameForce, "f", false, common.FlagDescUninstallForce)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}

func CmdSystemGenerateBundleFactory(configuredPlatform common.Platform) *cobra.Command {

	//This implementation will warn the user that the command is not available for Kubernetes environments.
	kubeCommand := kube.NewCmdCmdSystemGenerateBundle()
	nonKubeCommand := nonkube.NewCmdCmdSystemGenerateBundle()

	cmdSystemGenerateBundleDesc := common.SkupperCmdDescription{
		Use:   "generate-bundle <bundle-file>",
		Short: "Generate a bundle",
		Long:  "Generate a self-contained site bundle for use on another machine.",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSystemGenerateBundleDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandSystemGenerateBundleFlags{}

	cmd.Flags().StringVar(&cmdFlags.Input, common.FlagNameInput, "", common.FlagDescInput)
	cmd.Flags().StringVarP(&cmdFlags.Type, common.FlagNameType, "", "tarball", common.FlagDescType)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

	return cmd
}
