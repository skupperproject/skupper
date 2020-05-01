package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/test"
	"github.com/stretchr/testify/assert"
)

func TestHelloWorld(t *testing.T) {
	if os.Getenv(test.INTEGRATION) == "" {
		t.Skipf("skipping test; %s not set", test.INTEGRATION)
	}

	var vanRouterCreateOpts types.VanRouterCreateOptions = types.VanRouterCreateOptions{
		SkupperName:       "theSkupperName",
		IsEdge:            false,
		EnableController:  false,
		EnableServiceSync: false,
		EnableConsole:     false,
		AuthMode:          types.ConsoleAuthModeUnsecured,
		User:              "theUser",
		Password:          "nopasswordd",
		ClusterLocal:      true,
		Replicas:          2,
	}
	cli, _ := client.NewClient("default", "minikube", os.Getenv("KUBECONFIG"))
	err := cli.VanRouterCreate(context.Background(), vanRouterCreateOpts)
	assert.Equal(t, nil, err)

	//TODO assert expected services, pods, etc
}
