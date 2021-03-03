package client

import (
	"context"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"
)

func TestConnectorInspectError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create the client
	cli, err := newMockClient("skupper", "", "")
	assert.Check(t, err, "Unabled to create client.")

	_, err = cli.ConnectorInspect(ctx, "conn1")
	assert.Error(t, err, `configmaps "skupper-internal" not found`, "Expect error when VAN is not deployed")
}

func TestConnectorInspectNotFound(t *testing.T) {
	testcases := []struct {
		doc           string
		expectedError string
		connName      string
	}{
		{
			expectedError: `secrets "conn1" not found`,
			doc:           "test one",
			connName:      "conn1",
		},
		{
			expectedError: `secrets "all" not found`,
			doc:           "test two",
			connName:      "all",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := newMockClient("skupper", "", "")
	assert.Check(t, err, "Unabled to create client.")

	err = cli.RouterCreate(ctx, types.SiteConfig{
		Spec: types.SiteConfigSpec{
			SkupperName:       "skupper",
			RouterMode:        string(types.TransportModeInterior),
			EnableController:  true,
			EnableServiceSync: true,
			EnableConsole:     false,
			AuthMode:          "",
			User:              "",
			Password:          "",
			Ingress:           types.IngressNoneString,
		},
	})
	assert.Check(t, err, "Unable to create VAN router")

	for _, c := range testcases {
		_, err := cli.ConnectorInspect(ctx, c.connName)
		assert.Error(t, err, c.expectedError, c.doc)
	}
}
