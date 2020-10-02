package client

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"gotest.tools/assert"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

var fp = fmt.Fprintf

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

func TestConnectorCreateInterior(t *testing.T) {
	testcases := []struct {
		doc             string
		expectedError   string
		connName        string
		connFile        string
		secretsExpected []string
		opts            []cmp.Option
	}{
		{
			doc:             "Expect generated name to be conn1",
			expectedError:   "",
			connName:        "",
			secretsExpected: []string{"conn1"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "conn") }),
			},
		},
		{
			doc:             "Expect secret name to be as provided: conn2",
			expectedError:   "",
			connName:        "conn2",
			secretsExpected: []string{"conn1", "conn2"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "conn") }),
			},
		},
		{
			doc:             "Expect secret name to be as provided: conn3",
			expectedError:   "",
			connName:        "conn3",
			secretsExpected: []string{"conn1", "conn2", "conn3"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "conn") }),
			},
		},
		{
			doc:             "Expect secret already exists: conn2",
			expectedError:   "A connector secret of that name already exist, please choose a different name",
			connName:        "conn2",
			secretsExpected: []string{"conn1", "conn2", "conn3"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "conn") }),
			},
		},
		{
			doc:             "Expect generated name to be conn4",
			expectedError:   "",
			connName:        "",
			secretsExpected: []string{"conn1", "conn2", "conn3", "conn4"},
			opts: []cmp.Option{
				trans,
				cmpopts.IgnoreSliceElements(func(v string) bool { return !strings.HasPrefix(v, "conn") }),
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create and set up the two namespaces that we will be using.
	tokenCreatorNamespace := "van-connector-create-interior"
	tokenUserNamespace := "van-connector-create-edge"
	tokenCreatorClient, tokenUserClient := setupTwoNamespaces(t, ctx, tokenCreatorNamespace, tokenUserNamespace)
	defer kube.DeleteNamespace(tokenCreatorNamespace, tokenCreatorClient.KubeClient)
	defer kube.DeleteNamespace(tokenUserNamespace, tokenUserClient.KubeClient)

	secretsFound := []string{}
	informers := informers.NewSharedInformerFactory(tokenCreatorClient.KubeClient, 0)
	secretsInformer := informers.Core().V1().Secrets().Informer()
	secretsInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			secret := obj.(*corev1.Secret)
			if !strings.HasPrefix(secret.Name, "skupper") {
				secretsFound = append(secretsFound, secret.Name)
			}
		},
	})

	informers.Start(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), secretsInformer.HasSynced)

	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)
	defer os.RemoveAll(testPath)

	for _, c := range testcases {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		err := tokenCreatorClient.ConnectorTokenCreateFile(ctx, c.connName, testPath+c.connName+".yaml")
		assert.Assert(t, err, "Unable to create token")

		_, err = tokenUserClient.ConnectorCreateFromFile(ctx, testPath+c.connName+".yaml", types.ConnectorCreateOptions{
			Name:             c.connName,
			SkupperNamespace: tokenUserNamespace,
			Cost:             1,
		})
		if c.expectedError == "" {
			assert.Assert(t, err, "Unable to create connector")
			// TODO: make more deterministic
			time.Sleep(time.Second * 1)
			if diff := cmp.Diff(c.secretsExpected, secretsFound, c.opts...); diff != "" {
				t.Errorf("TestConnectorCreateInterior "+c.doc+" secrets mismatch (-want +got):\n%s", diff)
			}
			//assert.Assert(t, cmp.Equal(c.secretsExpected, secretsFound, trans), c.doc)
		} else {
			assert.Error(t, err, c.expectedError, c.doc)
			if diff := cmp.Diff(c.secretsExpected, secretsFound, c.opts...); diff != "" {
				t.Errorf("TestConnectorCreateInterior "+c.doc+" secrets mismatch (-want +got):\n%s", diff)
			}
		}

	}
}

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

func setupTwoNamespaces(t *testing.T, ctx context.Context, tokenCreatorNamespace, tokenUserNamespace string) (tokenCreatorClient, tokenUserClient *VanClient) {
	var err error
	if *clusterRun {
		tokenCreatorClient, err = NewClient(tokenCreatorNamespace, "", "")
		tokenUserClient, err = NewClient(tokenUserNamespace, "", "")
	} else {
		tokenCreatorClient, err = newMockClient(tokenCreatorNamespace, "", "")
		tokenUserClient, err = newMockClient(tokenUserNamespace, "", "")
	}
	assert.Assert(t, err)

	_, err = kube.NewNamespace(tokenCreatorNamespace, tokenCreatorClient.KubeClient)
	assert.Assert(t, err)
	_, err = kube.NewNamespace(tokenUserNamespace, tokenUserClient.KubeClient)
	assert.Assert(t, err)

	configureSiteAndCreateRouter(t, ctx, tokenCreatorClient, "tokenCreator")
	configureSiteAndCreateRouter(t, ctx, tokenUserClient, "tokenUser")

	return tokenCreatorClient, tokenUserClient
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
