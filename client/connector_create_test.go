package client

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/version"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var lightRed string = "\033[1;31m"
var resetColor string = "\033[0m"

func TestConnectorCreateError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cli *VanClient
	var err error
	ns := "namespace-for-testconnectorcreateerror"
	if *clusterRun {
		cli, err = NewClient(ns, "", "")
	} else {
		cli, err = newMockClient(ns, "", "")
	}
	assert.Assert(t, err)

	_, err = kube.NewNamespace(ns, cli.KubeClient)
	assert.Check(t, err, ns)
	defer kube.DeleteNamespace(ns, cli.KubeClient)
	configureSiteAndCreateRouter(t, ctx, cli, "public")

	// We forget to actually create the token...
	// ... so the connector creation should fail.
	secretFileName := "last-night-i-met-upon-the-stair.yaml"
	_, err = cli.ConnectorCreateFromFile(ctx, secretFileName, types.ConnectorCreateOptions{
		Name:             "a-token-file-that-wasnt-there",
		SkupperNamespace: ns,
		Cost:             1,
	})
	assert.Assert(t, strings.Contains(err.Error(), "no such file or directory"))
}

func TestSelfConnect(t *testing.T) {

	if !*clusterRun {
		lightRed := "\033[1;31m"
		resetColor := "\033[0m"
		t.Skip(fmt.Sprintf("%sSkipping: This test only works in real clusters.%s", string(lightRed), string(resetColor)))
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
	connectionName := "link1"
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
		Name:             "link1",
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
		RouterMode:        string(types.TransportModeInterior),
		EnableController:  true,
		EnableServiceSync: true,
		EnableConsole:     false,
		AuthMode:          "",
		User:              "",
		Password:          "",
		Ingress:           types.IngressNoneString,
	}
	siteConfig, err := cli.SiteConfigCreate(context.Background(), routerCreateOpts)
	assert.Assert(t, err, "Unable to configure %s site", name)
	err = cli.RouterCreate(ctx, *siteConfig)
	assert.Assert(t, err, "Unable to create %s VAN router", name)
}

func TestVerifySiteCompatibility(t *testing.T) {

	testTable := []struct {
		siteVersion       string
		minimumCompatible string
		clientSiteVersion string
		accept            bool
	}{
		{
			siteVersion:       "0.8.0",
			minimumCompatible: "0.8.0",
			clientSiteVersion: "",
			accept:            false,
		},
		{
			siteVersion:       "0.8.0",
			minimumCompatible: "0.8.0",
			clientSiteVersion: "0.7.0",
			accept:            false,
		},
		{
			siteVersion:       "nodeport-338-g6558216-modified",
			minimumCompatible: "0.8.0",
			clientSiteVersion: "0.7.0",
			accept:            true,
		},
		{
			siteVersion:       "nodeport-338-g6558216",
			minimumCompatible: "0.8.0",
			clientSiteVersion: "0.8.0",
			accept:            true,
		},
		{
			siteVersion:       "0.8.0",
			minimumCompatible: "0.8.0",
			clientSiteVersion: "nodeport-338-g6558216",
			accept:            true,
		},
		{
			siteVersion:       "nodeport-338-g6558216",
			minimumCompatible: "0.8.0",
			clientSiteVersion: "nodeport-338-g6558216",
			accept:            true,
		},
		{
			siteVersion:       "0.8.0",
			minimumCompatible: "0.8.0",
			clientSiteVersion: "0.8.0",
			accept:            true,
		},
		{
			siteVersion:       "0.8.0",
			minimumCompatible: "0.8.0",
			clientSiteVersion: "0.8.1",
			accept:            true,
		},
		{
			siteVersion:       "0.8.0",
			minimumCompatible: "0.8.0",
			clientSiteVersion: "0.9.0",
			accept:            true,
		},
		{
			siteVersion:       "0.9.0",
			minimumCompatible: "0.8.0",
			clientSiteVersion: "0.8.0",
			accept:            true,
		},
		{
			siteVersion:       "0.10.0",
			minimumCompatible: "0.8.0",
			clientSiteVersion: "0.8.0",
			accept:            true,
		},
		{
			siteVersion:       "0.10.0",
			minimumCompatible: "0.9.1",
			clientSiteVersion: "0.9.0",
			accept:            false,
		},
		{
			siteVersion:       "0.10.0",
			minimumCompatible: "0.9.1",
			clientSiteVersion: "0.9.1",
			accept:            true,
		},
	}

	var cli *VanClient
	var err error

	cli, err = newMockClient("fakens", "", "")
	assert.Assert(t, err)

	// Generates a fake client that simulates a skupper-internal using a specific version
	initKubeClient := func(siteVersion string) {
		cli.KubeClient = fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "fakens",
				Name:      types.TransportConfigMapName,
			},
			Data: map[string]string{
				"skrouterd.json": `
    [
        [
            "router",
            {
                "id": "skupper-fakens",
                "mode": "interior",
                "helloMaxAgeSeconds": "3",
                "metadata": "{\"id\":\"my-fake-site-id\",\"version\":\"` + siteVersion + `\"}"
            }
        ]
    ]
`,
			},
		})
	}

	originalMinimumCompatible := version.MinimumCompatibleVersion
	for _, test := range testTable {
		name := fmt.Sprintf("site-%s-minimum-%s-provided-%s", test.siteVersion, version.MinimumCompatibleVersion, test.clientSiteVersion)
		t.Run(name, func(t *testing.T) {
			version.MinimumCompatibleVersion = test.minimumCompatible
			initKubeClient(test.siteVersion)
			compatErr := cli.VerifySiteCompatibility(test.clientSiteVersion)
			assert.Assert(t, test.accept == (compatErr == nil))
			if !test.accept {
				assert.Assert(t, compatErr.Error() == fmt.Sprintf("minimum version required %s", test.minimumCompatible))
			}
		})
	}
	version.MinimumCompatibleVersion = originalMinimumCompatible
}
