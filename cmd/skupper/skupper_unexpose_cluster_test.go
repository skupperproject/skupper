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

	cmd.SilenceErrors = true
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

func newMockClient(namespace string) *client.VanClient {
	return &client.VanClient{
		Namespace:  namespace,
		KubeClient: fake.NewSimpleClientset(),
	}
}

func testClient(cmd *cobra.Command, args []string) {
	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}
}

func TestUnexposeWithCluster(t *testing.T) {
	testcases := []struct {
		args        []string
		flags       []string
		expectedOut string
		expectedErr string
	}{
		{
			args:        []string{"--help"},
			flags:       []string{},
			expectedOut: "Unexpose a set of pods previously exposed through a Skupper address",
			expectedErr: "",
		},
		{
			args:        []string{},
			flags:       []string{},
			expectedErr: "expose target and name must be specified (e.g. 'skupper expose deployment <name>'",
		},
		{
			args:        []string{"deployment"},
			flags:       []string{},
			expectedErr: "expose target and name must be specified (e.g. 'skupper expose deployment <name>'",
		},
		{
			args:        []string{"deployment", "tcp-not-deployed"},
			flags:       []string{},
			expectedErr: "Unable to unbind skupper service:",
		},
		{
			args:        []string{"deployment/tcp-not-deployed"},
			flags:       []string{},
			expectedErr: "Unable to unbind skupper service:",
		},
		{
			args:        []string{"deployent", "tcp-not-deployed"},
			flags:       []string{},
			expectedErr: "target type must be one of: [deployment, statefulset, pods, service]",
		},
		{
			args:        []string{"pods", "tcp-not-deployed"},
			flags:       []string{},
			expectedErr: "Unable to unbind skupper service: Target type for service interface not yet implemented",
		},
		{
			args:        []string{"deployment", "tcp-not-deployed", "extraArg"},
			flags:       []string{},
			expectedErr: "illegal argument: extraArg",
		},
		{
			args:        []string{"deployment", "tcp-not-deployed", "extraArg", "extraExtraArg"},
			flags:       []string{},
			expectedErr: "illegal argument: extraArg",
		},
	}

	// along with cli these are package vars
	namespace = "cmd-unexpose-cluster-test"
	kubeContext = ""
	kubeConfigPath = ""

	// test cases assume running skupper, set up pre-condition
	if *clusterRun {
		cli = NewClient(namespace, kubeContext, kubeConfigPath)
	} else {
		cli = newMockClient(namespace)
	}

	if c, ok := cli.(*client.VanClient); ok {
		_, err := kube.NewNamespace(namespace, c.KubeClient)
		assert.Check(t, err)
		defer kube.DeleteNamespace(namespace, c.KubeClient)
	}
	initCmd := NewCmdInit(testClient)
	silenceCobra(initCmd)
	_, err := executeCommand(initCmd, []string{"--edge"}...)
	assert.Assert(t, err)

	for _, tc := range testcases {
		cmd := NewCmdUnexpose(testClient)
		silenceCobra(cmd)
		output, err := executeCommand(cmd, tc.args...)

		if tc.expectedErr != "" {
			checkStringContains(t, err.Error(), tc.expectedErr)
		} else {
			assert.Assert(t, err)
		}
		if tc.expectedOut != "" {
			checkStringContains(t, output, tc.expectedOut)
		} else {
			assert.Equal(t, output, "")
		}
	}

}
