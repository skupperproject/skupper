package client

import (
	"context"
	"os"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"
)

func TestConnectorCreateTokenInterior(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := newMockClient("skupper", "", "")

	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
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

	err = cli.ConnectorTokenCreateFile(ctx, "conn1", "./conn1.yaml")
	assert.Check(t, err, "Unable to create connector token")

	os.Remove("./conn1.yaml")
}

func TestConnectorCreateTokenEdge(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := newMockClient("skupper", "", "")

	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "skupper",
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
	assert.Check(t, err, "Unable to create VAN router")

	err = cli.ConnectorTokenCreateFile(ctx, "conn1", "/tmp/conn1.yaml")
	assert.Error(t, err, "Edge configuration cannot accept connections", "Expect error when edge")

}
