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

func TestVanConnectorListInterior(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	connNames := []string{"conn1", "conn2", "conn3"}

	var namespace string = "van-connector-list-interior"

	var cli *VanClient
	var err error
	if *clusterRun {
		cli, err = NewClient(namespace, "", "")
	} else {
		cli, err = newMockClient(namespace, "", "")
	}
	assert.Assert(t, err)

	_, err = kube.NewNamespace(namespace, cli.KubeClient)
	defer kube.DeleteNamespace(namespace, cli.KubeClient)

	testPath := "./tmp/"
	os.Mkdir(testPath, 0755)

	err = cli.VanRouterCreate(ctx, types.VanSiteConfig{
		Spec: types.VanSiteConfigSpec{
			SkupperName:       "skupper",
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
	assert.Check(t, err, "Unable to create VAN router")

	for _, connName := range connNames {
		// TODO: make more deterministic
		time.Sleep(time.Second * 1)
		err = cli.VanConnectorTokenCreateFile(ctx, connName, testPath+connName+".yaml")
		assert.Check(t, err, "Unable to create connector token:"+connName)
	}
	for _, connName := range connNames {
		// TODO: make more deterministic
		time.Sleep(time.Second * 1)
		_, err = cli.VanConnectorCreateFromFile(ctx, testPath+connName+".yaml", types.VanConnectorCreateOptions{
			Name:             connName,
			SkupperNamespace: namespace,
			Cost:             1,
		})
		assert.Check(t, err, "Unable to create connector for "+connName)
	}
	connectors, err := cli.VanConnectorList(ctx)
	actualNames := []string{}
	for _, connector := range connectors {
		actualNames = append(actualNames, connector.Name)
	}
	assert.Check(t, err, "Unable to get connector list")
	if diff := cmp.Diff(connNames, actualNames, trans); diff != "" {
		t.Errorf("TestVanConnectorListInterior connectors mismatch (-want +got):\n%s", diff)
	}
	os.RemoveAll(testPath)

}
