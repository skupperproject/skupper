package podman

import (
	"testing"

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

	/*
		Create(service Service) error
		Delete(address string) error
		Get(address string) (Service, error)
		List() ([]Service, error)
		AddEgressResolver(address string, egressResolver EgressResolver) error
		RemoveEgressResolver(address string, egressResolver EgressResolver) error
		RemoveAllEgressResolvers(address string) error
	*/

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
}
