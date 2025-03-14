package system

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/system/kube"
	"github.com/skupperproject/skupper/internal/cmd/skupper/system/nonkube"
	"github.com/skupperproject/skupper/internal/config"

	"github.com/spf13/cobra"
)

var (
	systemSetupDescription = `
Bootstraps a nonkube Skupper site base on the provided flags.

When the path (--path) flag is provided, it will be used as the source
directory containing the Skupper custom resources to be processed,
generating a local Skupper site using the "default" namespace, unless
a namespace is set in the custom resources, or if the namespace (-n)
flag is provided.

A namespace is just a directory in the file system where all site specific
files are stored, like certificates, configurations, the original sources
(original custom resources used to bootstrap the nonkube site) and
the runtime files generated during initialization.

Namespaces are stored under ${XDG_DATA_HOME}/skupper/namespaces
for regular users when XDG_DATA_HOME environment variable is set, or under
${HOME}/.local/share/skupper/namespaces when it is not set.

As the root user, namespaces are stored under: /var/lib/skupper/namespaces.
In case the path (--path) flag is omitted, Skupper will try to process
custom resources stored at the input/resources directory of the default namespace,
or from the namespace provided through the namespace (-n) flag.

If the respective namespace already exists and you want to bootstrap it
over, you must provide the force (-f) flag. When you do that, the existing
Certificate Authorities (CAs) are preserved, so eventual existing incoming
links should be able to reconnect.

To produce a bundle, instead of rendering a site, the bundle strategy (-b)
flag must be set to "bundle" or "tarball".
`
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
	cmd.AddCommand(CmdSystemSetupFactory(platform))
	cmd.AddCommand(CmdSystemReloadFactory(platform))
	cmd.AddCommand(CmdSystemStartFactory(platform))
	cmd.AddCommand(CmdSystemStopFactory(platform))
	cmd.AddCommand(CmdSystemTeardownFactory(platform))
	cmd.AddCommand(CmdSystemInstallFactory(platform))
	cmd.AddCommand(CmdSystemUnInstallFactory(platform))
	cmd.AddCommand(CmdSystemGenerateBundleFactory(platform))

	return cmd
}

func CmdSystemSetupFactory(configuredPlatform common.Platform) *cobra.Command {

	//This implementation will warn the user that the command is not available for Kubernetes environments.
	kubeCommand := kube.NewCmdSystemSetup()
	nonKubeCommand := nonkube.NewCmdSystemSetup()

	cmdSystemStartDesc := common.SkupperCmdDescription{
		Use:   "setup",
		Short: "Create a non-kube site based on provided Skupper Custom Resources",
		Long:  systemSetupDescription,
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSystemStartDesc, kubeCommand, nonKubeCommand)

	cmdFlags := common.CommandSystemSetupFlags{}

	cmd.Flags().StringVar(&cmdFlags.Path, common.FlagNamePath, "", common.FlagDescPath)
	cmd.Flags().StringVarP(&cmdFlags.Strategy, common.FlagNameStrategy, "b", "", common.FlagDescStrategy)
	cmd.Flags().BoolVarP(&cmdFlags.Force, common.FlagNameForce, "f", false, common.FlagDescForce)

	kubeCommand.CobraCmd = cmd
	kubeCommand.Flags = &cmdFlags
	nonKubeCommand.CobraCmd = cmd
	nonKubeCommand.Flags = &cmdFlags

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

func CmdSystemStartFactory(configuredPlatform common.Platform) *cobra.Command {

	//This implementation will warn the user that the command is not available for Kubernetes environments.
	kubeCommand := kube.NewCmdCmdSystemStart()
	nonKubeCommand := nonkube.NewCmdCmdSystemStart()

	cmdSystemStartDesc := common.SkupperCmdDescription{
		Use:     "start",
		Short:   "Start the Skupper components for the current site",
		Long:    "Start down the Skupper components for the current site",
		Example: "skupper system start -n my-namespace",
	}

	cmd := common.ConfigureCobraCommand(configuredPlatform, cmdSystemStartDesc, kubeCommand, nonKubeCommand)

	return cmd
}

func CmdSystemStopFactory(configuredPlatform common.Platform) *cobra.Command {

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

func CmdSystemTeardownFactory(configuredPlatform common.Platform) *cobra.Command {

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
		Use:   "generate-bundle",
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
