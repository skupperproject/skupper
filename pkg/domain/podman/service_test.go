package podman

import (
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/pkg/domain"
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
	nginxService := &ServicePodman{
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

func compareNginxSvc(t *testing.T, nginxService *ServicePodman, svc domain.Service) {
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
			EnableTls:      true,
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
