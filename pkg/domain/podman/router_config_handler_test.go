//go:build podman
// +build podman

package podman

import (
	"os"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/qdr"
	"gotest.tools/assert"
)

func TestPodmanRouterConfigHandler(t *testing.T) {
	var err error

	// router config handler for podman
	os.Setenv(types.ENV_PLATFORM, "podman")
	configHandler := NewRouterConfigHandlerPodman(cli)

	// saving initial router config
	config := new(qdr.RouterConfig)
	*config = qdr.InitialConfigSkupperRouter("test-site", "test-site-id", "undefined", false, 1000, types.RouterOptions{
		Logging: []types.RouterLogConfig{
			{
				Module: "default",
				Level:  "trace+",
			},
		},
		DebugMode:        "gdb",
		MaxFrameSize:     types.RouterMaxFrameSizeDefault,
		MaxSessionFrames: types.RouterMaxSessionFramesDefault,
		IngressHost:      "127.0.0.1",
	})
	assert.Assert(t, configHandler.SaveRouterConfig(config))
	defer func() {
		assert.Assert(t, configHandler.RemoveRouterConfig())
	}()

	// retrieving router config
	config, err = configHandler.GetRouterConfig()
	assert.Assert(t, err)
	assert.Equal(t, config.Metadata.Id, "test-site")
	assert.Equal(t, config.GetSiteMetadata().Id, "test-site-id")
	assert.Equal(t, config.Metadata.Mode, qdr.ModeInterior)
	assert.Equal(t, config.GetSiteMetadata().Platform, "podman")
	assert.Assert(t, len(config.Listeners) > 0)
	assert.Assert(t, len(config.SslProfiles) > 0)

	// modifying and updating
	assert.Assert(t, config.Metadata.HelloMaxAgeSeconds != "9999")
	config.Metadata.HelloMaxAgeSeconds = "9999"
	assert.Assert(t, configHandler.SaveRouterConfig(config))
	config, err = configHandler.GetRouterConfig()
	assert.Assert(t, err)
	assert.Equal(t, config.Metadata.HelloMaxAgeSeconds, "9999")
}
