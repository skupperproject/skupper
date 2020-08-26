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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if !*clusterRun {
		lightRed := "\033[1;31m"
		resetColor := "\033[0m"
		t.Skip(fmt.Sprintf("%sSkipping: This test only works in real clusters.%s", string(lightRed), string(resetColor)))
		return
	}

	var public_client, private_client *VanClient
	var err error

	// Set up Public namespace ----------------------
	public_Namespace := "public"
	public_client, err = NewClient(public_Namespace, "", "")
	assert.Check(t, err, public_Namespace)

	_, err = kube.NewNamespace(public_Namespace, public_client.KubeClient)
	assert.Check(t, err, public_Namespace)
	defer kube.DeleteNamespace(public_Namespace, public_client.KubeClient)

	// Set up Private namespace ----------------------
	privateNamespace := "private"
	private_client, err = NewClient(privateNamespace, "", "")
	assert.Check(t, err, privateNamespace)

	_, err = kube.NewNamespace(privateNamespace, private_client.KubeClient)
	assert.Check(t, err, privateNamespace)
	defer kube.DeleteNamespace(privateNamespace, private_client.KubeClient)

	// Create Public Router: interior. ----------------------
	err = public_client.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       public_Namespace,
			IsEdge:            false,
			EnableController:  true,
			EnableServiceSync: true,
			EnableConsole:     false,
			AuthMode:          "",
			User:              "",
			Password:          "",
			ClusterLocal:      true,
		},
	})
	assert.Check(t, err, "Unable to create public router")

	// Create Private Router: edge. ----------------------
	err = private_client.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       privateNamespace,
			IsEdge:            true,
			EnableController:  true,
			EnableServiceSync: true,
			EnableConsole:     false,
			AuthMode:          "",
			User:              "",
			Password:          "",
			ClusterLocal:      true,
		},
	})
	assert.Check(t, err, "Unable to create private router")

	// Here's where we will put the connection token.
	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)
	defer os.RemoveAll(testPath)

	// Create the connection token for Public ---------------------------------
	connectionName := "conn1"
	secretFileName := testPath + connectionName + ".yaml"
	err = public_client.ConnectorTokenCreateFile(ctx, connectionName, secretFileName)
	assert.Assert(t, err, "Unable to create token")

	// And now try to use it ... to connect to Public!
	// This attempt at self-connection should fail.
	_, err = public_client.ConnectorCreateFromFile(ctx, secretFileName, types.ConnectorCreateOptions{
		Name:             connectionName,
		SkupperNamespace: public_Namespace,
		Cost:             1,
	})
	assert.Assert(t, err != nil, "Self-connection should fail.")
}
