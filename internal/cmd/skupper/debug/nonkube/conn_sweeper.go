package nonkube

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/sweeper"
	"github.com/skupperproject/skupper/internal/nonkube/client/runtime"
	nonkubecommon "github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/spf13/cobra"
)

// Cert paths inside the router container (see compat.SiteStateRenderer, which
// mounts the runtime certs at /etc/skupper-router/runtime/certs).
const containerCertsPath = "/etc/skupper-router/runtime/certs/skupper-local-client"

type CmdConnSweeper struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandConnSweeperFlags
	namespace string
	platform  string
	url       string
	sslArgs   []string
	exec      sweeper.Execer
}

func NewCmdConnSweeper() *CmdConnSweeper {
	return &CmdConnSweeper{}
}

func (cmd *CmdConnSweeper) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.namespace = cobraCommand.Flag(common.FlagNameNamespace).Value.String()
}

func (cmd *CmdConnSweeper) ValidateInput(args []string) error {
	if cmd.Flags.IdleThreshold <= 0 {
		return fmt.Errorf("--idle-threshold must be a positive number of seconds")
	}
	return nil
}

func (cmd *CmdConnSweeper) InputToOptions() {
	if cmd.namespace == "" {
		cmd.namespace = "default"
	}
	cmd.url = cmd.Flags.URL

	// Detect the platform from the namespace's site config.
	platformLoader := &nonkubecommon.NamespacePlatformLoader{}
	platform, err := platformLoader.Load(cmd.namespace)
	if err != nil {
		return
	}
	cmd.platform = platform

	switch platform {

	case "podman", "docker":
		// exec inside it so skmanage and the socket
		// query see the router's network namespace.
		containerName := cmd.namespace + "-skupper-router"
		cmd.exec = containerExecer(platform, containerName)
		if !cmd.urlOverridden() {
			// The site's local management listener is amqps with client-cert
			// auth; certs are mounted inside the container.
			if url, err := runtime.GetLocalRouterAddress(cmd.namespace); err == nil {
				cmd.url = url
				cmd.sslArgs = sslArgs(containerCertsPath+"/tls.crt", containerCertsPath+"/tls.key", containerCertsPath+"/ca.crt")
			}
		}
	default: // linux
		if !cmd.urlOverridden() {
			if url, err := runtime.GetLocalRouterAddress(cmd.namespace); err == nil {
				certs := runtime.GetRuntimeTlsCert(cmd.namespace, "skupper-local-client")
				cmd.url = url
				cmd.sslArgs = sslArgs(certs.CertPath, certs.KeyPath, certs.CaPath)
			}
		}
	}
}

func (cmd *CmdConnSweeper) Run() error {
	if cmd.exec != nil {
		fmt.Printf("running against %s container %s-skupper-router\n", cmd.platform, cmd.namespace)
	}
	_, err := sweeper.Run(sweeper.Config{
		URL:               cmd.url,
		Skmanage:          cmd.Flags.Skmanage,
		IdleThresholdSecs: cmd.Flags.IdleThreshold,
		DryRun:            cmd.Flags.DryRun,
		Exec:              cmd.exec,
		SkmanageExtraArgs: cmd.sslArgs,
	})
	return err
}

func (cmd *CmdConnSweeper) WaitUntil() error { return nil }

// urlOverridden reports whether the user set --url.
func (cmd *CmdConnSweeper) urlOverridden() bool {
	return cmd.CobraCmd != nil && cmd.CobraCmd.Flags().Changed("url")
}

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
