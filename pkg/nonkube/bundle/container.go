package bundle

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	internal "github.com/skupperproject/skupper/internal/nonkube"
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

	if len(containers) > 0 {
		buf.WriteString("#!/bin/sh\n\n")
	}

	for _, c := range containers {
		var createCmd []string
		createCmd = append(createCmd, "{{.ContainerEngine}}", "run", "-d")
		createCmd = append(createCmd, fmt.Sprintf("--name=%s", escapeArgument(c.Name)))
		createCmd = append(createCmd, "--user={{.RunAs}}")
		createCmd = append(createCmd, "--userns={{.UserNamespace}}")
		for envName, envVal := range c.Env {
			createCmd = append(createCmd, fmt.Sprintf("--env=%s=%s", envName, escapeArgument(envVal)))
		}
		createCmd = append(createCmd, "--label=application=skupper")
		for labelName, labelVal := range c.Labels {
			createCmd = append(createCmd, fmt.Sprintf("--label=%s=%s", labelName, escapeArgument(labelVal)))
		}
		for _, mount := range c.FileMounts {
			options := ""
			if len(mount.Options) > 0 {
				options = fmt.Sprintf(":%s", strings.Join(mount.Options, ""))
			}
			createCmd = append(createCmd, fmt.Sprintf("--volume=%s:%s%s", mount.Source, mount.Destination, options))
		}
		createCmd = append(createCmd, "--restart=always")
		createCmd = append(createCmd, "--network=host")
		createCmd = append(createCmd, c.Image)
		prettyCreateCmd := internal.PrettyPrintCommand(createCmd[0], createCmd[1:])
		buf.WriteString(prettyCreateCmd)
	}

	return buf.Bytes()
}
