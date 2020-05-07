// +build integration

package tcp_echo

import (
	"context"
	"os"
	"path"
	"testing"

	"gotest.tools/assert"
)

func TestTcpEcho(t *testing.T) {
	testRunner := &TcpEchoClusterTestRunner{}

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		homedir, err := os.UserHomeDir()
		assert.Check(t, err)
		kubeconfig = path.Join(homedir, ".kube/config")
	}

	//TODO: accept 4 different kubeconfigs
	//For now for simplicity we use the same single kubeconfig. Multicluster
	//support can be enabled easily whe needed.
	testRunner.Build(t, kubeconfig, kubeconfig, kubeconfig, kubeconfig)
	ctx := context.Background()
	testRunner.Run(ctx)
}
