//go:build podman
// +build podman

package podman

import (
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
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
			PrometheusOpts: types.PrometheusServerOptions{
				ExternalServer: "http://10.0.0.1:8080/v1",
				AuthMode:       "internal",
				User:           "admin",
				Password:       "admin",
			},
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
			expDeployments := 2
			if scenarioSite.EnableFlowCollector {
				expDeployments += 2
			}
			assert.Assert(t, len(podmanSite.IngressHosts) == expIngHosts)
			assert.Equal(t, len(podmanSite.Deployments), expDeployments)
			for _, dep := range podmanSite.GetDeployments() {
				assert.Assert(t, len(dep.GetComponents()) > 0, "no components found for %s", dep.GetName())
				for _, cmp := range dep.GetComponents() {
					var cmpContainer *container.Container
					err = utils.Retry(time.Second*6, 10, func() (bool, error) {
						cmpContainer, err = siteHandler.cli.ContainerInspect(cmp.Name())
						if err != nil {
							return true, err
						}
						return cmpContainer.Running, nil
					})
					assert.Assert(t, err, "error retrieving container info")
					assert.Assert(t, cmpContainer.Running, "component %s is not running - exit code: %d "+
						"- restarts: %d", cmpContainer.Name, cmpContainer.ExitCode, cmpContainer.RestartCount)
				}
			}
			assert.Assert(t, len(podmanSite.Credentials) > 0)
			assert.Assert(t, len(podmanSite.CertAuthorities) > 0)
			assert.Equal(t, scenarioSite.PrometheusOpts.ExternalServer, podmanSite.PrometheusOpts.ExternalServer)
			assert.Equal(t, scenarioSite.PrometheusOpts.AuthMode, podmanSite.PrometheusOpts.AuthMode)
			assert.Equal(t, scenarioSite.PrometheusOpts.User, podmanSite.PrometheusOpts.User)
			assert.Equal(t, scenarioSite.PrometheusOpts.Password, podmanSite.PrometheusOpts.Password)
		})
	}
}
