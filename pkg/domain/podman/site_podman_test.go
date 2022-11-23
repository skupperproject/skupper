//go:build system || podman
// +build system podman

package podman

import (
	"testing"

	"github.com/skupperproject/skupper/client/container"
	"gotest.tools/assert"
)

func TestSiteHandlerCreate(t *testing.T) {
	siteHandler, err := NewSitePodmanHandler(getEndpoint())
	assert.Assert(t, err)

	err = siteHandler.Create(siteBasic)
	assert.Assert(t, err)
}

func TestSiteHandlerGet(t *testing.T) {
	siteHandler, err := NewSitePodmanHandler(getEndpoint())
	assert.Assert(t, err)

	site, err := siteHandler.Get()
	assert.Assert(t, err)

	podmanSite := site.(*SitePodman)
	assert.Assert(t, podmanSite.GetName() == siteBasic.GetName())
	assert.Assert(t, podmanSite.GetMode() == "interior")
	assert.Assert(t, podmanSite.ContainerNetwork == container.ContainerNetworkName)
	assert.Assert(t, len(podmanSite.IngressHosts) > 0)
	assert.Assert(t, len(podmanSite.Deployments) > 0)
	for _, dep := range podmanSite.GetDeployments() {
		assert.Assert(t, len(dep.GetComponents()) > 0, "no components found for %s", dep.GetName())
	}
	assert.Assert(t, len(podmanSite.Credentials) > 0)
	assert.Assert(t, len(podmanSite.CertAuthorities) > 0)
}

func TestSiteHandlerDelete(t *testing.T) {
	siteHandler, err := NewSitePodmanHandler(getEndpoint())
	assert.Assert(t, err)
	err = siteHandler.Delete()
	assert.Assert(t, err)
	site, err := siteHandler.Get()
	assert.Assert(t, err != nil)
	assert.Assert(t, site == nil)
}
