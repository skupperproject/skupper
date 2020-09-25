package client

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"gotest.tools/assert"
)

type Test struct {
	namespaces []string
}

// var fp = fmt.Fprintf

func TestSelfConnect(t *testing.T) {

	if !*clusterRun {
		var red string = "\033[1;31m"
		var resetColor string = "\033[0m"
		t.Skip(fmt.Sprintf("%sSkipping: This test only works in real clusters.%s", string(red), string(resetColor)))
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if !*clusterRun {
		lightRed := "\033[1;31m"
		resetColor := "\033[0m"
		t.Skip(fmt.Sprintf("%sSkipping: This test only works in real clusters.%s", string(lightRed), string(resetColor)))
		return
	}

	var publicClient *VanClient
	var err error

	// Set up Public namespace ----------------------
	publicNamespace := "public"
	publicClient, err = NewClient(publicNamespace, "", "")
	assert.Check(t, err, publicNamespace)

	_, err = kube.NewNamespace(publicNamespace, publicClient.KubeClient)
	assert.Check(t, err, publicNamespace)
	defer kube.DeleteNamespace(publicNamespace, publicClient.KubeClient)

	// Configure the site.
	// It needs to be done this way -- by calling SiteConfigCreate() --
	// when using a real cluster, because that function has a side-effect
	// of creating the config map with the K8S API.
	routerCreateOpts := types.SiteConfigSpec{
		SkupperName:       publicNamespace,
		IsEdge:            false,
		EnableController:  true,
		EnableServiceSync: true,
		EnableConsole:     false,
		AuthMode:          "",
		User:              "",
		Password:          "",
		ClusterLocal:      true,
	}
	siteConfig, err := publicClient.SiteConfigCreate(context.Background(), routerCreateOpts)

	// Create Public Router: interior. ----------------------
	err = publicClient.RouterCreate(ctx, *siteConfig)
	assert.Check(t, err, "Unable to create public router")

	// Here's where we will put the connection token.
	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)
	defer os.RemoveAll(testPath)

	// Create the connection token for Public ---------------------------------
	connectionName := "conn1"
	secretFileName := testPath + connectionName + ".yaml"
	err = publicClient.ConnectorTokenCreateFile(ctx, connectionName, secretFileName)
	assert.Assert(t, err, "Unable to create token")

	// And now try to use it ... to connect to Public!
	// This attempt at self-connection should fail.
	_, err = publicClient.ConnectorCreateFromFile(ctx, secretFileName, types.ConnectorCreateOptions{
		Name:             connectionName,
		SkupperNamespace: publicNamespace,
		Cost:             1,
	})
	assert.Assert(t, err != nil, "Self-connection should fail.")
}
