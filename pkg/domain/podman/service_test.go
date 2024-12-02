//go:build podman
// +build podman

package podman

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/qdr"
	"gotest.tools/assert"
)

func TestPodmanServiceInterfaceHandler(t *testing.T) {
	// creating basic site
	assert.Assert(t, createBasicSite(), "unable to create basic podman site")
	// teardown
	defer func() {
		assert.Assert(t, teardownBasicSite())
	}()

	// run nginx as 'nginx-container'
	assert.Assert(t, runNginxContainer())

	// service handler instance
	svcHandler := NewServiceHandlerPodman(cli)

	// service used in this test
	nginxService := &Service{
		ServiceCommon: &domain.ServiceCommon{
			Address:  "nginx",
			Protocol: "tcp",
			Ports:    []int{8080},
			EgressResolvers: []domain.EgressResolver{
				&domain.EgressResolverHost{
					Host: "nginx-container",
				},
			},
		},
	}

	t.Run("service-create", func(t *testing.T) {
		assert.Assert(t, svcHandler.Create(nginxService))
	})
	t.Run("service-get", func(t *testing.T) {
		svc, err := svcHandler.Get(nginxService.Address)
		assert.Assert(t, err)
		compareNginxSvc(t, nginxService, svc)
	})
	t.Run("service-list", func(t *testing.T) {
		svcs, err := svcHandler.List()
		assert.Assert(t, err)
		assert.Equal(t, 1, len(svcs))
		compareNginxSvc(t, nginxService, svcs[0])
	})
	t.Run("service-remove-egress-resolver", func(t *testing.T) {
		assert.Assert(t, svcHandler.RemoveEgressResolver(nginxService.Address, nginxService.EgressResolvers[0]))
		svc, err := svcHandler.Get(nginxService.Address)
		assert.Assert(t, err)
		assert.Equal(t, 0, len(svc.GetEgressResolvers()))
	})
	t.Run("service-add-egress-resolver", func(t *testing.T) {
		assert.Assert(t, svcHandler.AddEgressResolver(nginxService.Address, nginxService.EgressResolvers[0]))
		svc, err := svcHandler.Get(nginxService.Address)
		assert.Assert(t, err)
		assert.Equal(t, 1, len(svc.GetEgressResolvers()))
	})
	t.Run("service-remove-all-egress-resolvers", func(t *testing.T) {
		assert.Assert(t, svcHandler.RemoveAllEgressResolvers(nginxService.Address))
		svc, err := svcHandler.Get(nginxService.Address)
		assert.Assert(t, err)
		assert.Equal(t, 0, len(svc.GetEgressResolvers()))
	})
	t.Run("service-delete", func(t *testing.T) {
		assert.Assert(t, svcHandler.Delete(nginxService.Address))
	})
}

func compareNginxSvc(t *testing.T, nginxService *Service, svc domain.Service) {
	assert.Equal(t, nginxService.GetAddress(), svc.GetAddress())
	assert.Equal(t, nginxService.GetProtocol(), svc.GetProtocol())
	assert.DeepEqual(t, nginxService.GetPorts(), svc.GetPorts())
	assert.Equal(t, len(nginxService.GetEgressResolvers()), len(svc.GetEgressResolvers()))
	assert.Equal(t, nginxService.GetEgressResolvers()[0].String(), svc.GetEgressResolvers()[0].String())
}

func TestPodmanServiceHandler(t *testing.T) {
	// creating a dummy skupper-services volume
	_, err := cli.VolumeCreate(&container.Volume{
		Name: "skupper-services",
	})
	assert.Assert(t, err)

	// defer removal of dummy volume
	defer func() {
		assert.Assert(t, cli.VolumeRemove("skupper-services"))
	}()

	// testing skupper-services handler
	svcIfaceHandler := NewServiceInterfaceHandlerPodman(cli)

	// service definitions
	services := []*types.ServiceInterface{
		{
			Address:      "nginx",
			Protocol:     "tcp",
			Ports:        []int{8080},
			EventChannel: true,
			Aggregate:    "json",
			Labels: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},
			Targets: []types.ServiceInterfaceTarget{
				{
					TargetPorts: map[int]int{8080: 8080},
					Service:     "192.168.122.1",
				},
			},
			TlsCredentials: "nginx-tls-credentials",
		},
	}

	t.Run("service-iface-create", func(t *testing.T) {
		for _, svc := range services {
			assert.Assert(t, svcIfaceHandler.Create(svc))
		}
	})
	t.Run("service-iface-get", func(t *testing.T) {
		for i, svc := range services {
			svcGet, err := svcIfaceHandler.Get(svc.Address)
			assert.Assert(t, err)
			assert.DeepEqual(t, services[i], svcGet)
		}
	})
	t.Run("service-iface-list", func(t *testing.T) {
		svcs, err := svcIfaceHandler.List()
		assert.Assert(t, err)
		assert.Assert(t, len(svcs) == len(services))
		for _, svc := range services {
			svcMap := svcs[svc.Address]
			assert.DeepEqual(t, svcMap, svc)
		}
	})
	t.Run("service-iface-update", func(t *testing.T) {
		svc := services[0]
		svc.Protocol = "http"
		assert.Assert(t, svcIfaceHandler.Update(svc))
		svcGet, err := svcIfaceHandler.Get(svc.Address)
		assert.Assert(t, err)
		assert.DeepEqual(t, svc, svcGet)
	})
	t.Run("service-iface-delete", func(t *testing.T) {
		for i, svc := range services {
			assert.Assert(t, svcIfaceHandler.Delete(svc.Address))
			svcList, err := svcIfaceHandler.List()
			assert.Assert(t, err)
			newLength := len(services) - (i + 1)
			assert.Equal(t, newLength, len(svcList))
		}
	})
}

func TestCreateRouterServiceConfig(t *testing.T) {

	fakeService := func(address string, ports []int, containerName string) *Service {
		svc := &Service{
			ServiceCommon: &domain.ServiceCommon{
				Address:  address,
				Ports:    ports,
				Protocol: "tcp",
			},
			ContainerName: containerName,
		}
		return svc
	}
	site := &Site{
		SiteCommon: &domain.SiteCommon{
			Name: "my-site-name",
			Id:   "my-site-id",
		},
	}
	parentConfig := &qdr.RouterConfig{
		LogConfig: map[string]qdr.LogConfig{
			"DEFAULT": qdr.LogConfig{
				Module: "DEFAULT",
				Enable: "info+",
			},
		},
	}
	egressResolvers := func(service *Service) {
		service.AddEgressResolver(&domain.EgressResolverHost{
			Host:  "10.0.0.1",
			Ports: map[int]int{8080: 8080},
		})
		service.AddEgressResolver(&domain.EgressResolverHost{
			Host:  "10.0.0.2",
			Ports: map[int]int{8080: 8081},
		})
	}
	scenarios := []struct {
		name     string
		service  *Service
		modifier func(*Service)
		expError bool
	}{
		{
			name:    "basic-service-tcp",
			service: fakeService("address", []int{8080}, ""),
		},
		{
			name:    "basic-service-tcp-custom-container",
			service: fakeService("address", []int{8080}, "custom-address"),
		},
		{
			name:    "basic-service-tcp-multiple-ports",
			service: fakeService("address", []int{8080, 8081, 8082}, ""),
		},
		{
			name:    "basic-service-http",
			service: fakeService("address", []int{8080}, ""),
			modifier: func(service *Service) {
				service.Protocol = "http"
			},
		},
		{
			name:    "basic-service-http2",
			service: fakeService("address", []int{8080}, ""),
			modifier: func(service *Service) {
				service.Protocol = "http2"
			},
		},
		{
			name:    "basic-service-tls",
			service: fakeService("address", []int{8080}, ""),
			modifier: func(service *Service) {
				service.TlsCredentials = "my-credentials"
			},
		},
		{
			name:     "basic-service-tcp-with-targets",
			service:  fakeService("address", []int{8080}, ""),
			modifier: egressResolvers,
		},
		{
			name:    "basic-service-http-with-targets",
			service: fakeService("address", []int{8080}, ""),
			modifier: func(service *Service) {
				service.Protocol = "http"
				egressResolvers(service)
			},
		},
		{
			name:    "basic-service-http2-with-targets",
			service: fakeService("address", []int{8080}, ""),
			modifier: func(service *Service) {
				service.Protocol = "http2"
				egressResolvers(service)
			},
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			if scenario.modifier != nil {
				scenario.modifier(scenario.service)
			}
			routerConfig, routerConfigStr, err := domain.CreateRouterServiceConfig(site, parentConfig, scenario.service, scenario.service.GetContainerName())
			if scenario.expError {
				assert.Assert(t, err != nil)
				return
			}
			assert.Assert(t, err == nil)
			assert.Assert(t, routerConfig != nil)
			assert.Assert(t, routerConfigStr != "")

			var expectedTcpListeners int
			var expectedHttpListeners int
			var expectedPorts = len(scenario.service.Ports)
			switch scenario.service.Protocol {
			case "tcp":
				expectedTcpListeners = expectedPorts
			case "http":
				expectedHttpListeners = expectedPorts
			case "http2":
				expectedHttpListeners = expectedPorts
			}
			assert.Equal(t, len(routerConfig.Bridges.TcpListeners), expectedTcpListeners)
			assert.Equal(t, len(routerConfig.Bridges.HttpListeners), expectedHttpListeners)

			var sslProfilesExpected = 1
			if scenario.service.IsTls() {
				sslProfilesExpected++
			}
			assert.Equal(t, sslProfilesExpected, len(routerConfig.SslProfiles))

			for _, port := range scenario.service.Ports {
				addressIndex := fmt.Sprintf("%s:%d", scenario.service.Address, port)
				if expectedTcpListeners > 0 {
					tcpListener := routerConfig.Bridges.TcpListeners[addressIndex]
					if scenario.service.ContainerName == "" {
						assert.Equal(t, tcpListener.Host, scenario.service.Address)
					} else {
						assert.Equal(t, tcpListener.Host, scenario.service.ContainerName)
					}
					assert.Equal(t, tcpListener.Address, addressIndex)
					assert.Equal(t, tcpListener.Port, strconv.Itoa(port))

					assert.Equal(t, len(scenario.service.GetEgressResolvers()), len(routerConfig.Bridges.TcpConnectors))
					for _, egressResolver := range scenario.service.GetEgressResolvers() {
						egresses, err := egressResolver.Resolve()
						assert.Assert(t, err)
						for _, egress := range egresses {
							targetHost := egress.GetHost()
							for _, targetPort := range egress.GetPorts() {
								connectorName := fmt.Sprintf("%s@%s:%d:%d", scenario.service.Address, targetHost, port, targetPort)
								connector, ok := routerConfig.Bridges.TcpConnectors[connectorName]
								assert.Assert(t, ok)
								assert.Equal(t, strconv.Itoa(targetPort), connector.Port)
							}
						}
					}
				} else {
					httpListener := routerConfig.Bridges.HttpListeners[addressIndex]
					if scenario.service.ContainerName == "" {
						assert.Equal(t, httpListener.Host, scenario.service.Address)
					} else {
						assert.Equal(t, httpListener.Host, scenario.service.ContainerName)
					}
					assert.Equal(t, httpListener.Address, addressIndex)
					assert.Equal(t, httpListener.Port, strconv.Itoa(port))

					assert.Equal(t, len(scenario.service.GetEgressResolvers()), len(routerConfig.Bridges.HttpConnectors))
					for _, egressResolver := range scenario.service.GetEgressResolvers() {
						egresses, err := egressResolver.Resolve()
						assert.Assert(t, err)
						for _, egress := range egresses {
							targetHost := egress.GetHost()
							for _, targetPort := range egress.GetPorts() {
								connectorName := fmt.Sprintf("%s@%s:%d:%d", scenario.service.Address, targetHost, port, targetPort)
								connector, ok := routerConfig.Bridges.HttpConnectors[connectorName]
								assert.Assert(t, ok)
								assert.Equal(t, strconv.Itoa(targetPort), connector.Port)
							}
						}
					}
				}
			}
		})
	}
}
