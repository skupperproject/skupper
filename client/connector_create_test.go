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

func TestConnectorCreateError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := newMockClient("my-namespace", "", "")
	assert.Assert(t, err)

	_, err = cli.ConnectorCreateFromFile(ctx, "./somefile.yaml", types.ConnectorCreateOptions{
		Name: "",
		Cost: 1,
	})
	assert.ErrorContains(t, err, "open ./somefile.yaml: no such file or directory", "Expect error when file not found")
}

func TestSelfConnect(t *testing.T) {

	if !*clusterRun {
		lightRed := "\033[1;31m"
		resetColor := "\033[0m"
		t.Skip(fmt.Sprintf("%sSkipping: This test only works in real clusters.%s", string(lightRed), string(resetColor)))
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var publicClient *VanClient
	var err error

	// Set up Public namespace ----------------------
	publicNamespace := "public"
	publicClient, err = NewClient(publicNamespace, "", "")
	assert.Check(t, err, publicNamespace)

	_, err = kube.NewNamespace(publicNamespace, publicClient.KubeClient)
	assert.Check(t, err, publicNamespace)
	defer kube.DeleteNamespace(publicNamespace, publicClient.KubeClient)

	// Configuring the site needs to be done by calling SiteConfigCreate()
	// when using a real cluster, because that function has a side-effect
	// of creating the config map with the K8S API.
	configureSiteAndCreateRouter(t, ctx, publicClient, "public")

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

func TestMultipleConnect(t *testing.T) {

	if !*clusterRun {
		lightRed := "\033[1;31m"
		resetColor := "\033[0m"
		t.Skip(fmt.Sprintf("%sSkipping: This test only works in real clusters.%s", string(lightRed), string(resetColor)))
		return
	}

	var err error
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tokenCreatorNamespace := "creator"
	tokenUserNamespace := "user"
	creatorClient, userClient, err := setupTwoNamespaces(t, ctx, tokenCreatorNamespace, tokenUserNamespace)
	assert.Assert(t, err, "Can't set up namespaces")
	defer kube.DeleteNamespace(tokenCreatorNamespace, creatorClient.KubeClient)
	defer kube.DeleteNamespace(tokenUserNamespace, userClient.KubeClient)

	// Here's where we will put the connection token.
	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)
	defer os.RemoveAll(testPath)

	// Create the connection token for Public ---------------------------------
	connectionName := "token1"
	secretFileName := testPath + connectionName + ".yaml"
	err = creatorClient.ConnectorTokenCreateFile(ctx, connectionName, secretFileName)
	assert.Assert(t, err, "Unable to create token")

	// Use the token to make a connector.
	_, err = userClient.ConnectorCreateFromFile(ctx, secretFileName, types.ConnectorCreateOptions{
		Name:             "conn1",
		SkupperNamespace: tokenCreatorNamespace,
		Cost:             1,
	})
	assert.Assert(t, err, "Can't make first connection")

	// Try to make a second connection.
	// This should fail.
	_, err = userClient.ConnectorCreateFromFile(ctx, secretFileName, types.ConnectorCreateOptions{
		Name:             "conn2",
		SkupperNamespace: tokenCreatorNamespace,
		Cost:             1,
	})
	assert.Assert(t, err != nil, "Second connection attempt should fail")
}

func setupTwoNamespaces(t *testing.T, ctx context.Context, tokenCreatorNamespace, tokenUserNamespace string) (tokenCreatorClient, tokenUserClient *VanClient, err error) {
	if *clusterRun {
		tokenCreatorClient, err = NewClient(tokenCreatorNamespace, "", "")
		tokenUserClient, err = NewClient(tokenUserNamespace, "", "")
	} else {
		tokenCreatorClient, err = newMockClient(tokenCreatorNamespace, "", "")
		tokenUserClient, err = newMockClient(tokenUserNamespace, "", "")
	}
	if err != nil {
		return nil, nil, err
	}

	_, err = kube.NewNamespace(tokenCreatorNamespace, tokenCreatorClient.KubeClient)
	if err != nil {
		return nil, nil, err
	}
	_, err = kube.NewNamespace(tokenUserNamespace, tokenUserClient.KubeClient)
	if err != nil {
		return nil, nil, err
	}

	configureSiteAndCreateRouter(t, ctx, tokenCreatorClient, "tokenCreator")
	configureSiteAndCreateRouter(t, ctx, tokenUserClient, "tokenUser")

	return tokenCreatorClient, tokenUserClient, nil
}

func configureSiteAndCreateRouter(t *testing.T, ctx context.Context, cli *VanClient, name string) {
	routerCreateOpts := types.SiteConfigSpec{
		SkupperName:       "skupper",
		IsEdge:            false,
		EnableController:  true,
		EnableServiceSync: true,
		EnableConsole:     false,
		AuthMode:          "",
		User:              "",
		Password:          "",
		ClusterLocal:      true,
	}
	siteConfig, err := cli.SiteConfigCreate(context.Background(), routerCreateOpts)
	assert.Assert(t, err, "Unable to configure %s site", name)
	err = cli.RouterCreate(ctx, *siteConfig)
	assert.Assert(t, err, "Unable to create %s VAN router", name)
}
