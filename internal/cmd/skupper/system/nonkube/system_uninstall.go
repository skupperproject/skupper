package nonkube

import (
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/spf13/cobra"
)

type CmdSystemUninstall struct {
	CobraCmd         *cobra.Command
	Namespace        string
	SystemUninstall  func(string) error
	CheckActiveSites func() (bool, error)
	Flags            *common.CommandSystemUninstallFlags
	forceUninstall   bool
	TearDown         func(namespace string) error
}

func NewCmdSystemUninstall() *CmdSystemUninstall {

	skupperCmd := CmdSystemUninstall{}

	return &skupperCmd
}

func (cmd *CmdSystemUninstall) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.SystemUninstall = bootstrap.Uninstall
	cmd.CheckActiveSites = bootstrap.CheckActiveSites
	cmd.Namespace = cobraCommand.Flag("namespace").Value.String()
	cmd.TearDown = bootstrap.Teardown
}

func (cmd *CmdSystemUninstall) ValidateInput(args []string) error {
	var validationErrors []error
	namespaceStringValidator := validator.NamespaceStringValidator()

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not accept arguments"))
	}

	if config.GetPlatform() != types.PlatformPodman && config.GetPlatform() != types.PlatformDocker {
		validationErrors = append(validationErrors, fmt.Errorf("the selected platform is not supported by this command. There is nothing to uninstall"))
	}

	if cmd.Namespace != "" {
		ok, err := namespaceStringValidator.Evaluate(cmd.Namespace)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("namespace is not valid: %s", err))
		}
	}

	if cmd.Flags != nil && !cmd.Flags.Force {
		activeSites, err := cmd.CheckActiveSites()
		if err != nil {
			return err
		}
		if activeSites {
			validationErrors = append(validationErrors, fmt.Errorf("Uninstallation halted: Active sites detected. Use --force flag to stop and remove active sites"))
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdSystemUninstall) InputToOptions() {

	cmd.forceUninstall = cmd.Flags.Force
}

func (cmd *CmdSystemUninstall) Run() error {
	if cmd.forceUninstall {
		entries, err := os.ReadDir(path.Join(api.GetHostDataHome(), "namespaces/"))
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				runtimeDir := "namespaces/" + entry.Name() + "/runtime/"
				_, err := os.ReadDir(path.Join(api.GetHostDataHome(), runtimeDir))
				if err == nil {
					err := cmd.TearDown(entry.Name())
					if err != nil {
						return fmt.Errorf("failed to remove site \"%s\": %s", entry.Name(), err)
					}
				} else {
					// site not active so just remove directory
					err := os.RemoveAll(api.GetHostNamespaceHome(entry.Name()))
					if err == nil {
						fmt.Printf("Namespace \"%s\" has been removed\n", entry.Name())
					} else {
						return fmt.Errorf("failed to remove site \"%s\": %s", entry.Name(), err)
					}
				}
			}
		}
	}

	err := cmd.SystemUninstall(string(config.GetPlatform()))

	if err != nil {
		return fmt.Errorf("failed to uninstall : %s", err)
	}

	return nil
}

func (cmd *CmdSystemUninstall) WaitUntil() error { return nil }
