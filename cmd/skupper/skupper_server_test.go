package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes/fake"

	"github.com/spf13/cobra"
	"gotest.tools/assert"

	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/kube"
)

func executeCommand(cmd *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err = cmd.ExecuteContext(context.Background())
	return buf.String(), err
}

func checkStringContains(t *testing.T, got, expected string) {
	if !strings.Contains(got, expected) {
		t.Errorf("Expected to contain: \n %v\nGot:\n %v\n", expected, got)
	}
}

func checkStringOmits(t *testing.T, got, expected string) {
	if strings.Contains(got, expected) {
		t.Errorf("Expected to not contain: \n %v\nGot: %v", expected, got)
	}
}

func TestUnexposeCommandWithCluster(t *testing.T) {
	testcases := []struct {
		args        []string
		flags       []string
		expectedOut string
	}{
		{
			args:        []string{"--help"},
			flags:       []string{},
			expectedOut: "Unexpose a set of pods previously exposed through a Skupper address",
		},
		{
			args:        []string{},
			flags:       []string{},
			expectedOut: "expose target and name must be specified (e.g. 'skupper expose deployment <name>'",
		},
		{
			args:        []string{"deployment"},
			flags:       []string{},
			expectedOut: "expose target and name must be specified (e.g. 'skupper expose deployment <name>'",
		},
		{
			args:        []string{"deployment", "tcp-not-deployed"},
			flags:       []string{},
			expectedOut: "Unable to unbind skupper service: Could not find entry for service interface tcp-not-deployed",
		},
		{
			args:        []string{"deployment/tcp-not-deployed"},
			flags:       []string{},
			expectedOut: "Unable to unbind skupper service: Could not find entry for service interface tcp-not-deployed",
		},
		{
			args:        []string{"deployent", "tcp-not-deployed"},
			flags:       []string{},
			expectedOut: "expose target type must be one of: [deployment, statefulset, pods, service]",
		},
		{
			args:        []string{"pods", "tcp-not-deployed"},
			flags:       []string{},
			expectedOut: "Error: Unable to unbind skupper service: Target type for service interface not yet implemented",
		},
		{
			args:        []string{"deployment", "tcp-not-deployed", "extraArg"},
			flags:       []string{},
			expectedOut: "Error: illegal argument: extraArg",
		},
		{
			args:        []string{"deployment", "tcp-not-deployed", "extraArg", "extraExtraArg"},
			flags:       []string{},
			expectedOut: "Error: illegal argument: extraArg",
		},
	}

	var cli *client.VanClient
	ns := "cmd-unexpose-cluster-test"

	if *clusterRun {
		cli = NewClient(ns, "", "")
	} else {
		cli = &client.VanClient{
			Namespace:  ns,
			KubeClient: fake.NewSimpleClientset(),
		}
	}

	_, err := kube.NewNamespace(ns, cli.KubeClient)
	assert.Check(t, err)
	defer kube.DeleteNamespace(ns, cli.KubeClient)

	// test case assumes skupper init'ed
	initCmd := NewCmdInit(cli)
	_, err = executeCommand(initCmd, []string{"--edge"}...)
	assert.Assert(t, err)

	for _, tc := range testcases {
		cmd := NewCmdUnexpose(cli)
		cmd.SilenceUsage = true
		output, _ := executeCommand(cmd, tc.args...)
		checkStringContains(t, output, tc.expectedOut)
	}

}
