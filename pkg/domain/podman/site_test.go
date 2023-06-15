//go:build podman
// +build podman

package podman

import (
	"testing"

	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/utils"
	"gotest.tools/assert"
)

func TestSiteHandler(t *testing.T) {
	siteHandler, err := NewSitePodmanHandler(getEndpoint())
	assert.Assert(t, err)

	scenarios := []struct {
		name string
		site domain.Site
	}{{
		name: "basic-ingress-localhost",
		site: newBasicSite(),
	}, {
		name: "basic-ingress-none",
		site: &Site{
			SiteCommon: &domain.SiteCommon{
				Name: "site-podman-no-ingress",
			},
		},
	}, {
		name: "flow-collector-internal-auth-ingress-localhost",
		site: &Site{
			SiteCommon: &domain.SiteCommon{
				Name: "site-podman-fc-ingress",
			},
			IngressHosts:        []string{"127.0.0.1"},
			EnableFlowCollector: true,
			EnableConsole:       true,
			AuthMode:            "internal",
			ConsoleUser:         "internal",
			ConsolePassword:     "internal",
		},
	}, {
		name: "flow-collector-internal-auth-ingress-none",
		site: &Site{
			SiteCommon: &domain.SiteCommon{
				Name: "site-podman-fc-no-ingress",
			},
			EnableFlowCollector: true,
			EnableConsole:       true,
			AuthMode:            "internal",
			ConsoleUser:         "internal",
			ConsolePassword:     "internal",
		},
	}}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Logf("creating site")
			err = siteHandler.Create(scenario.site)
			assert.Assert(t, err)

			// remove site
			defer func() {
				t.Logf("removing site")
				err = siteHandler.Delete()
				assert.Assert(t, err)
				site, err := siteHandler.Get()
				assert.Assert(t, err != nil)
				assert.Assert(t, site == nil)
			}()

			// Verifying site
			t.Logf("retrieving site")
			site, err := siteHandler.Get()
			assert.Assert(t, err)
			podmanSite := site.(*Site)

			t.Logf("validating site info")
			scenarioSite := scenario.site.(*Site)
			assert.Equal(t, podmanSite.GetName(), scenarioSite.GetName())
			assert.Equal(t, podmanSite.GetMode(), utils.DefaultStr(scenarioSite.GetMode(), "interior"))
			assert.Equal(t, podmanSite.ContainerNetwork, utils.DefaultStr(scenarioSite.ContainerNetwork, container.ContainerNetworkName))
			// number of expected ingress hosts
			expIngHosts := 1 + len(scenarioSite.IngressHosts)
			expDeployments := 1
			if scenarioSite.EnableFlowCollector {
				expDeployments += 1
			}
			assert.Assert(t, len(podmanSite.IngressHosts) == expIngHosts)
			assert.Assert(t, len(podmanSite.Deployments) == expDeployments)
			for _, dep := range podmanSite.GetDeployments() {
				assert.Assert(t, len(dep.GetComponents()) > 0, "no components found for %s", dep.GetName())
				for _, cmp := range dep.GetComponents() {
					cmpContainer, err := siteHandler.cli.ContainerInspect(cmp.Name())
					assert.Assert(t, err, "error retrieving container info")
					assert.Assert(t, cmpContainer.Running)
				}
			}
			assert.Assert(t, len(podmanSite.Credentials) > 0)
			assert.Assert(t, len(podmanSite.CertAuthorities) > 0)
		})
	}
}
