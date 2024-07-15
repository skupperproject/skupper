package bundle

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	internal "github.com/skupperproject/skupper/internal/non_kube"
	"github.com/skupperproject/skupper/pkg/container"
)

var (
	shellEscape = regexp.MustCompile(`([^a-zA-Z0-9,._+:@%/-])`)
)

const (
	shellReplace = "\\$1"
)

func escapeArgument(argument string) string {
	if strings.Contains(argument, "{{.") {
		return argument
	}
	noNewLineArgument := strings.Replace(argument, "\n", "", -1)
	return shellEscape.ReplaceAllString(strings.Trim(noNewLineArgument, "\n"), shellReplace)
}

func containersToShell(containers map[string]container.Container) []byte {
	buf := new(bytes.Buffer)

	for _, c := range containers {
		var createCmd []string
		createCmd = append(createCmd, "{{.ContainerEngine}}", "run", "-d", "--name", escapeArgument(c.Name))
		createCmd = append(createCmd, "-u", "{{.RunAs}}")
		createCmd = append(createCmd, "--userns", "{{.UserNamespace}}")
		for envName, envVal := range c.Env {
			createCmd = append(createCmd, "--env", fmt.Sprintf("%s=%s", envName, escapeArgument(envVal)))
		}
		createCmd = append(createCmd, "--label", "application=skupper")
		for labelName, labelVal := range c.Labels {
			createCmd = append(createCmd, "--label", fmt.Sprintf("%s=%s", labelName, escapeArgument(labelVal)))
		}
		for _, mount := range c.FileMounts {
			options := ""
			if len(mount.Options) > 0 {
				options = fmt.Sprintf(":%s", strings.Join(mount.Options, ""))
			}
			createCmd = append(createCmd, "--volume", fmt.Sprintf("%s:%s%s", mount.Source, mount.Destination, options))
		}
		createCmd = append(createCmd, "--restart", "always")
		createCmd = append(createCmd, "--network", "host")
		createCmd = append(createCmd, c.Image)
		prettyCreateCmd := internal.PrettyPrintCommand(createCmd[0], createCmd[1:])
		buf.WriteString(prettyCreateCmd)
	}

	return buf.Bytes()
}
