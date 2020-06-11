package client

import (
	"flag"
	"fmt"
	"testing"

	"gotest.tools/assert"
)

func TestNewClient(t *testing.T) {
	testcases := []struct {
		doc             string
		namespace       string
		context         string
		kubeConfigPath  string
		expectedError   string
		expectedVersion string
	}{
		{
			namespace:      "skupper",
			context:        "",
			kubeConfigPath: "",
			expectedError:  "",
			doc:            "test one",
		},
	}

	for _, c := range testcases {
		_, err := newMockClient(c.namespace, c.context, c.kubeConfigPath)
		assert.Check(t, err, c.doc)
	}
}

var runOnRealCluster = flag.Bool("use-real-cluster", false, "client package tests will use KUBECONFIG configured cluster")

func getVanClient(short bool, namespace string) (*VanClient, error) {

	if *runOnRealCluster {
		fmt.Println("Using real Client!")
		return NewClient(namespace, "", "")
	}
	fmt.Println("Using mock client.")
	return newMockClient(namespace, "", "")
}
