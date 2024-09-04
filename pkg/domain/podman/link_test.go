//go:build podman
// +build podman

package podman

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/skupperproject/skupper/pkg/utils"
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
	assert.Assert(t, siteHandler.Create(context.Background(), newBasicSite()))
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

			// On some clouds, it may take a while for the service DNS name to be externally
			// resolvable.  So, we extract that URL and wait for the name resolution to work
			// before creating the link.  If anything fails, we may mark the test as failed, but
			// keep going, as that's not the focus of the test; the whole thing  may fail down
			// the road, but with additional information for debugging.
			skupperUrl := token.Annotations["skupper.io/url"]
			if skupperUrl != "" {
				parsed, err := url.Parse(skupperUrl)
				if err != nil {
					t.Errorf("The skupper.io/url annotation did not parse as an URL (%q): %v", skupperUrl, err)
				} else {
					err = utils.RetryError(time.Second*2, 60, func() error {
						_, err := net.ResolveIPAddr("ip", parsed.Hostname())
						return err
					})
					if err != nil {
						log.Printf("Name resolution for skupper.io/url (%q) still failing after 2 minutes: %v", parsed.Hostname(), err)
					}
				}
			}
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
