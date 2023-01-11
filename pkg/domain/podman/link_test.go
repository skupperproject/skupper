//go:build podman
// +build podman

package podman

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestLinkHandlerPodman(t *testing.T) {
	if !*clusterRun {
		t.Skip("Test to run against real cluster")
	}

	// Create a basic podman site
	siteHandler, err := NewSitePodmanHandler(getEndpoint())
	assert.Assert(t, err)
	assert.Assert(t, siteHandler.Create(newBasicSite()))
	defer func() {
		assert.Assert(t, siteHandler.Delete())
	}()
	site, err := siteHandler.Get()
	assert.Assert(t, err)
	podmanSite := site.(*Site)

	// Testing link creation
	linkHandler := NewLinkHandlerPodman(podmanSite, cli)
	var token *corev1.Secret
	for _, tokenType := range []string{"cert", "claim"} {
		linkName := "link-" + tokenType
		t.Run("link-create-type-"+tokenType, func(t *testing.T) {
			if tokenType == "cert" {
				token, _, err = cliKube.ConnectorTokenCreate(context.Background(), "", cliKube.Namespace)
			} else {
				token, _, err = cliKube.TokenClaimCreate(context.Background(), "", []byte("password"), time.Minute*5, 1)
			}
			assert.Assert(t, err)
			assert.Assert(t, token != nil)
			err = linkHandler.Create(token, linkName, 2)
			assert.Assert(t, err)
		})
		t.Run("link-list-type-"+tokenType, func(t *testing.T) {
			secrets, err := linkHandler.List()
			assert.Assert(t, err)
			assert.Assert(t, len(secrets) == 1)
			for _, secret := range secrets {
				assert.Assert(t, linkName == secret.Name)
			}
		})
		t.Run(fmt.Sprintf("link-create-type-%s-dup", tokenType), func(t *testing.T) {
			err = linkHandler.Create(token, linkName+"-dup", 2)
			assert.ErrorContains(t, err, "Already connected to")
		})
		t.Run("link-delete-type-"+tokenType, func(t *testing.T) {
			err = linkHandler.Delete(linkName)
			assert.Assert(t, err)
		})
	}
}
