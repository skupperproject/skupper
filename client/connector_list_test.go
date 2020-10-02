package client

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"gotest.tools/assert"
)

func TestConnectorListInterior(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	connNames := []string{"conn1", "conn2", "conn3"}
	var err error

	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)
	defer os.RemoveAll(testPath)

	// Create and set up the two namespaces that we will be using.
	tokenCreatorNamespace := "van-connector-list-interior"
	tokenUserNamespace := "van-connector-list-edge"
	tokenCreatorClient, tokenUserClient := setupTwoNamespaces(t, ctx, tokenCreatorNamespace, tokenUserNamespace)
	defer kube.DeleteNamespace(tokenCreatorNamespace, tokenCreatorClient.KubeClient)
	defer kube.DeleteNamespace(tokenUserNamespace, tokenUserClient.KubeClient)

	for _, connName := range connNames {
		// TODO: make more deterministic
		time.Sleep(time.Second * 1)
		err = tokenCreatorClient.ConnectorTokenCreateFile(ctx, connName, testPath+connName+".yaml")
		assert.Check(t, err, "Unable to create connector token:"+connName)
	}
	for _, connName := range connNames {
		// TODO: make more deterministic
		time.Sleep(time.Second * 1)
		_, err = tokenUserClient.ConnectorCreateFromFile(ctx, testPath+connName+".yaml", types.ConnectorCreateOptions{
			Name:             connName,
			SkupperNamespace: tokenUserNamespace,
			Cost:             1,
		})
		assert.Check(t, err, "Unable to create connector for "+connName)
	}
	connectors, err := tokenUserClient.ConnectorList(ctx)
	actualNames := []string{}
	for _, connector := range connectors {
		actualNames = append(actualNames, connector.Name)
	}
	assert.Check(t, err, "Unable to get connector list")
	if diff := cmp.Diff(connNames, actualNames, trans); diff != "" {
		t.Errorf("TestConnectorListInterior connectors mismatch (-want +got):\n%s", diff)
	}
}
