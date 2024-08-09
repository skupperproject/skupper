package bundle

import (
	"strings"
	"testing"

	"github.com/skupperproject/skupper/pkg/container"
	"gotest.tools/assert"
)

func TestEscapeCommand(t *testing.T) {
	var tests = []struct {
		argument string
		expected string
	}{
		{
			argument: "standard-argument",
			expected: "standard-argument",
		},
		{
			argument: "spaces in argument",
			expected: "spaces\\ in\\ argument",
		},
		{
			argument: "special {[ch@ract$rs]}; in \\ argument",
			expected: "special\\ \\{\\[ch@ract\\$rs\\]\\}\\;\\ in\\ \\\\\\ argument",
		},
	}
	for _, test := range tests {
		result := escapeArgument(test.argument)
		assert.Equal(t, test.expected, result)
	}
}

func TestContainerToShell(t *testing.T) {
	var tests = []struct {
		description   string
		containers    map[string]container.Container
		expectedParts []string
	}{
		{
			description: "minimal-container",
			containers: map[string]container.Container{
				"container1": container.Container{
					Name:  "container1",
					Image: "image1",
				},
			},
			expectedParts: []string{
				`#!/bin/sh`,
				`{{.ContainerEngine}} run -d --name=container1 --user={{.RunAs}} --userns={{.UserNamespace}}`,
				`--label=application=skupper --restart=always --network=host image1`,
			},
		},
		{
			description: "normal-container",
			containers: map[string]container.Container{
				"container1": container.Container{
					Name:  "container1",
					Image: "image1",
					Env: map[string]string{
						"ENV_VAR1": "VALUE_1",
						"ENV_VAR2": "VALUE_2",
						"ENV_VAR3": "VALUE_3",
						"ENV_VAR4": "VALUE_4",
					},
					Labels: map[string]string{
						"label1": "value1",
						"label2": "value2",
						"label3": "value3",
						"label4": "value4",
					},
					FileMounts: []container.FileMount{
						{
							Source:      "/home/user/directory",
							Destination: "/dest/directory",
							Options:     []string{"z"},
						},
					},
				},
			},
			expectedParts: []string{
				`#!/bin/sh`,
				`{{.ContainerEngine}} run -d --name=container1 --user={{.RunAs}} --userns={{.UserNamespace}} `,
				` --env=ENV_VAR1=VALUE_1 `, ` --env=ENV_VAR2=VALUE_2 `, ` --env=ENV_VAR3=VALUE_3 `, ` --env=ENV_VAR4=VALUE_4 `,
				` --label=application=skupper `, ` --label=label1=value1 `, ` --label=label2=value2 `, ` --label=label3=value3 `,
				` --label=label4=value4 `, ` --restart=always `, `--network=host `, ` image1`,
			},
		},
		{
			description: "multiple-containers",
			containers: map[string]container.Container{
				"container1": container.Container{
					Name:  "container1",
					Image: "image1",
					Env: map[string]string{
						"ENV_VAR1": "VALUE_1",
						"ENV_VAR2": "VALUE_2",
						"ENV_VAR3": "VALUE_3",
						"ENV_VAR4": "VALUE_4",
					},
					Labels: map[string]string{
						"label1": "value1",
						"label2": "value2",
						"label3": "value3",
						"label4": "value4",
					},
					FileMounts: []container.FileMount{
						{
							Source:      "/home/user/directory",
							Destination: "/dest/directory",
							Options:     []string{"z"},
						},
					},
				},
			},
			expectedParts: []string{
				`#!/bin/sh`,
				`{{.ContainerEngine}} run -d --name=container1 --user={{.RunAs}} --userns={{.UserNamespace}} `,
				` --env=ENV_VAR1=VALUE_1 `, ` --env=ENV_VAR2=VALUE_2 `, ` --env=ENV_VAR3=VALUE_3 `, ` --env=ENV_VAR4=VALUE_4 `,
				` --label=application=skupper `, ` --label=label1=value1 `, ` --label=label2=value2 `, ` --label=label3=value3 `,
				` --label=label4=value4 `, ` --restart=always `, `--network=host `, ` image1`,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			scriptData := containersToShell(test.containers)
			script := string(scriptData)
			for _, part := range test.expectedParts {
				assert.Assert(t, strings.Contains(script, part), "%s not found in %s", part, script)
			}
		})
	}
}
