package nonkube

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/sweeper"
	"github.com/skupperproject/skupper/internal/nonkube/client/runtime"
	nonkubecommon "github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/spf13/cobra"
)

const containerCertsPath = "/etc/skupper-router/runtime/certs/skupper-local-client"
const (
	containerSkmanage = "/bin/skmanage"
	systemdSkmanage   = "/usr/bin/skmanage"
)

type CmdConnSweeper struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandConnSweeperFlags
	namespace string
	platform  string
	url       string
	skmanage  string
	sslArgs   []string
	exec      sweeper.Execer
}

func NewCmdConnSweeper() *CmdConnSweeper {
	return &CmdConnSweeper{}
}

func (cmd *CmdConnSweeper) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.CobraCmd = cobraCommand
	cmd.namespace, _ = cobraCommand.Flags().GetString(common.FlagNameNamespace)
}

func (cmd *CmdConnSweeper) ValidateInput(args []string) error {
	var validationErrors []error
	numberValidator := validator.NewNumberValidator()
	numberValidator.IncludeZero = false
	if ok, err := numberValidator.Evaluate(cmd.Flags.IdleThreshold); !ok {
		validationErrors = append(validationErrors, fmt.Errorf("idle-threshold is not valid: %s", err))
	}
	return errors.Join(validationErrors...)
}

func (cmd *CmdConnSweeper) InputToOptions() {
	if cmd.namespace == "" {
		cmd.namespace = "default"
	}

	platformLoader := &nonkubecommon.NamespacePlatformLoader{}
	platform, err := platformLoader.Load(cmd.namespace)
	if err != nil {
		return
	}
	cmd.platform = platform

	url, err := runtime.GetLocalRouterAddress(cmd.namespace)
	if err != nil {
		return
	}
	cmd.url = url

	switch platform {
	case "podman", "docker":
		cmd.skmanage = containerSkmanage
		cmd.exec = containerExecer(platform, cmd.namespace+"-skupper-router")
		cmd.sslArgs = sslArgs(containerCertsPath+"/tls.crt", containerCertsPath+"/tls.key", containerCertsPath+"/ca.crt")
	default:
		cmd.skmanage = systemdSkmanage
		certs := runtime.GetRuntimeTlsCert(cmd.namespace, "skupper-local-client")
		cmd.sslArgs = sslArgs(certs.CertPath, certs.KeyPath, certs.CaPath)
	}
}

func (cmd *CmdConnSweeper) Run() error {
	if cmd.url == "" || cmd.skmanage == "" {
		return fmt.Errorf("could not determine router management address for namespace %q", cmd.namespace)
	}
	if cmd.exec != nil {
		fmt.Printf("running against %s container %s-skupper-router\n", cmd.platform, cmd.namespace)
	}
	_, err := sweeper.Run(sweeper.Config{
		URL:               cmd.url,
		Skmanage:          cmd.skmanage,
		IdleThresholdSecs: cmd.Flags.IdleThreshold,
		DryRun:            cmd.Flags.DryRun,
		Exec:              cmd.exec,
		SkmanageExtraArgs: cmd.sslArgs,
	})
	return err
}

func (cmd *CmdConnSweeper) WaitUntil() error { return nil }

func sslArgs(cert, key, ca string) []string {
	return []string{"--ssl-certificate", cert, "--ssl-key", key, "--ssl-trustfile", ca}
}

// containerExecer returns a sweeper.Execer that runs argv in the router
// container via `<engine> exec` so that skmanage and python3 are
// available.
func containerExecer(engine, containerName string) sweeper.Execer {
	return func(argv []string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		full := append([]string{"exec", containerName}, argv...)
		out, err := exec.CommandContext(ctx, engine, full...).Output()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
				return nil, fmt.Errorf("%s exec %q failed: %w (stderr: %s)", engine, argv[0], err, ee.Stderr)
			}
			return nil, fmt.Errorf("%s exec %q failed: %w", engine, argv[0], err)
		}
		return out, nil
	}
}
