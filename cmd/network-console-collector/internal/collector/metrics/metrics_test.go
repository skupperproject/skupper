package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	prom_testutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"gotest.tools/assert"
)

func TestSiteInfoMetrics(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	metrics := New(reg)

	metrics.Add(vanflow.SiteRecord{BaseRecord: vanflow.NewBase("s1"), Name: ptrTo("x")})
	metrics.Update(vanflow.SiteRecord{}, vanflow.SiteRecord{BaseRecord: vanflow.NewBase("s1"), Name: ptrTo("x"), Version: ptrTo("1.0")})
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_info"), 1)
	assert.Equal(t, prom_testutil.ToFloat64(metrics.siteInfo.WithLabelValues("s1", "x", "1.0")), 1.0)
	metrics.Remove(vanflow.SiteRecord{BaseRecord: vanflow.NewBase("s1"), Name: ptrTo("x"), Version: ptrTo("1.0")})
	assert.Equal(t, prom_testutil.ToFloat64(metrics.siteInfo.WithLabelValues("s1", "x", "1.0")), 0.0)
}

func TestSiteRouterMetrics(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	metrics := New(reg)

	metrics.Add(vanflow.RouterRecord{BaseRecord: vanflow.NewBase("r1"), Mode: ptrTo("inter-router"), Parent: ptrTo("s1")})
	metrics.Add(vanflow.RouterRecord{BaseRecord: vanflow.NewBase("r2"), Parent: ptrTo("s1")})
	metrics.Update(vanflow.RouterRecord{}, vanflow.RouterRecord{BaseRecord: vanflow.NewBase("r2"), Mode: ptrTo("inter-router"), Parent: ptrTo("s1")})
	metrics.Add(vanflow.RouterRecord{BaseRecord: vanflow.NewBase("r3"), Mode: ptrTo("edge"), Parent: ptrTo("s1")})
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_routers_total"), 2)
	assert.Equal(t, prom_testutil.ToFloat64(metrics.routerInfo.WithLabelValues("s1", "inter-router")), 2.0)
	assert.Equal(t, prom_testutil.ToFloat64(metrics.routerInfo.WithLabelValues("s1", "edge")), 1.0)
	metrics.Remove(vanflow.RouterRecord{BaseRecord: vanflow.NewBase("r3"), Mode: ptrTo("edge"), Parent: ptrTo("s1")})
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_routers_total"), 2)
	assert.Equal(t, prom_testutil.ToFloat64(metrics.routerInfo.WithLabelValues("s1", "inter-router")), 2.0)
	assert.Equal(t, prom_testutil.ToFloat64(metrics.routerInfo.WithLabelValues("s1", "edge")), 0.0)
}

func TestSiteLinkMetrics(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	metrics := New(reg)
	interRouter := ptrTo("inter-router")
	statusUp := ptrTo("up")
	statusDown := ptrTo("down")

	metrics.Add(vanflow.LinkRecord{
		BaseRecord: vanflow.NewBase("l01"),
		Role:       interRouter,
		Status:     statusDown,
		Parent:     ptrTo("r01"),
	})
	metrics.Add(vanflow.LinkRecord{
		BaseRecord: vanflow.NewBase("l02"),
		Role:       interRouter,
		Status:     statusUp,
		Peer:       ptrTo("ap01"),
		Parent:     ptrTo("r01"),
	})
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_links_total"), 0)
	metrics.Add(vanflow.RouterRecord{
		BaseRecord: vanflow.NewBase("r01"),
	})
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_links_total"), 0)
	metrics.Update(vanflow.RouterRecord{
		BaseRecord: vanflow.NewBase("r01"),
	}, vanflow.RouterRecord{
		BaseRecord: vanflow.NewBase("r01"), Parent: ptrTo("s01"), Mode: interRouter,
	})
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_links_total"), 2)
	assert.Equal(t, prom_testutil.ToFloat64(metrics.siteLinkInfo.WithLabelValues("s01", "inter-router", "down")), 1.0)
	assert.Equal(t, prom_testutil.ToFloat64(metrics.siteLinkInfo.WithLabelValues("s01", "inter-router", "up")), 1.0)
	metrics.Update(
		vanflow.LinkRecord{
			BaseRecord: vanflow.NewBase("l01"),
			Role:       interRouter,
			Status:     statusDown,
			Parent:     ptrTo("r01"),
		}, vanflow.LinkRecord{
			BaseRecord: vanflow.NewBase("l01"),
			Role:       interRouter,
			Status:     statusUp,
			Peer:       ptrTo("ap02"),
			Parent:     ptrTo("r01"),
		},
	)
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_links_total"), 2)
	assert.Equal(t, prom_testutil.ToFloat64(metrics.siteLinkInfo.WithLabelValues("s01", "inter-router", "down")), 0.0)
	assert.Equal(t, prom_testutil.ToFloat64(metrics.siteLinkInfo.WithLabelValues("s01", "inter-router", "up")), 2.0)

	metrics.Update(vanflow.LinkRecord{}, vanflow.LinkRecord{
		BaseRecord: vanflow.NewBase("l03"),
		Role:       interRouter,
		Status:     statusUp,
		Peer:       ptrTo("ap01"),
		Parent:     ptrTo("r02"),
	})
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_links_total"), 2)
	metrics.Remove(vanflow.LinkRecord{
		BaseRecord: vanflow.NewBase("l03"),
		Role:       interRouter,
		Status:     statusUp,
		Peer:       ptrTo("ap01"),
		Parent:     ptrTo("r02"),
	})
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_links_total"), 2)
	metrics.Add(vanflow.RouterRecord{
		BaseRecord: vanflow.NewBase("r02"), Parent: ptrTo("s02"), Mode: interRouter,
	})
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_links_total"), 2)
}

func TestSiteListenerConnectorMetrics(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	metrics := New(reg)
	interRouter := ptrTo("inter-router")

	metrics.Add(vanflow.ListenerRecord{
		BaseRecord: vanflow.NewBase("l01"),
		Parent:     ptrTo("r01"),
	})
	metrics.Add(vanflow.ListenerRecord{
		BaseRecord: vanflow.NewBase("l02"),
		Parent:     ptrTo("r01"),
	})
	metrics.Add(vanflow.ConnectorRecord{
		BaseRecord: vanflow.NewBase("c01"),
		Parent:     ptrTo("r01"),
	})
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_listeners_total"), 0)
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_connectors_total"), 0)
	metrics.Add(vanflow.RouterRecord{
		BaseRecord: vanflow.NewBase("r01"),
	})
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_listeners_total"), 0)
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_connectors_total"), 0)
	metrics.Update(vanflow.RouterRecord{
		BaseRecord: vanflow.NewBase("r01"),
	}, vanflow.RouterRecord{
		BaseRecord: vanflow.NewBase("r01"), Parent: ptrTo("s01"), Mode: interRouter,
	})
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_listeners_total"), 1)
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_connectors_total"), 1)
	assert.Equal(t, prom_testutil.ToFloat64(metrics.siteListenerInfo.WithLabelValues("s01")), 2.0)
	assert.Equal(t, prom_testutil.ToFloat64(metrics.siteConnectorInfo.WithLabelValues("s01")), 1.0)
	metrics.Remove(vanflow.ListenerRecord{
		BaseRecord: vanflow.NewBase("l02"),
		Parent:     ptrTo("r01"),
	})
	assert.Equal(t, prom_testutil.ToFloat64(metrics.siteListenerInfo.WithLabelValues("s01")), 1.0)
	metrics.Remove(vanflow.ConnectorRecord{
		BaseRecord: vanflow.NewBase("c01"),
		Parent:     ptrTo("r01"),
	})
	assert.Equal(t, prom_testutil.ToFloat64(metrics.siteConnectorInfo.WithLabelValues("s01")), 0.0)
}

func TestSiteLinkErrorMetrics(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	metrics := New(reg)
	interRouter := ptrTo("inter-router")
	statusDown := ptrTo("down")

	metrics.Add(vanflow.RouterRecord{
		BaseRecord: vanflow.NewBase("r01"), Parent: ptrTo("s01"), Mode: interRouter,
	})
	metrics.Add(vanflow.LinkRecord{
		BaseRecord: vanflow.NewBase("l01"),
		Role:       interRouter,
		Status:     statusDown,
		Parent:     ptrTo("r01"),
		DownCount:  ptrTo(uint64(12)),
	})
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_link_errors_total"), 1)
	assert.Equal(t, prom_testutil.ToFloat64(metrics.siteLinkErrors.WithLabelValues("s01", "inter-router")), 0.0)
	metrics.Update(
		vanflow.LinkRecord{
			BaseRecord: vanflow.NewBase("l01"),
			Role:       interRouter,
			Status:     statusDown,
			Parent:     ptrTo("r01"),
			DownCount:  ptrTo(uint64(12)),
		}, vanflow.LinkRecord{
			BaseRecord: vanflow.NewBase("l01"),
			Role:       interRouter,
			Status:     statusDown,
			Peer:       ptrTo("ap02"),
			Parent:     ptrTo("r01"),
			DownCount:  ptrTo(uint64(15)),
		},
	)
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_link_errors_total"), 1)
	assert.Equal(t, prom_testutil.ToFloat64(metrics.siteLinkErrors.WithLabelValues("s01", "inter-router")), 3.0)
	metrics.Remove(vanflow.LinkRecord{
		BaseRecord: vanflow.NewBase("l01"),
		Role:       interRouter,
		Status:     statusDown,
		Parent:     ptrTo("r01"),
		DownCount:  ptrTo(uint64(15)),
	})
	// cannot decrease
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_link_errors_total"), 1)
	assert.Equal(t, prom_testutil.ToFloat64(metrics.siteLinkErrors.WithLabelValues("s01", "inter-router")), 3.0)
	metrics.Add(vanflow.LinkRecord{
		BaseRecord: vanflow.NewBase("l01"),
		Role:       interRouter,
		Status:     statusDown,
		Parent:     ptrTo("r01"),
		DownCount:  ptrTo(uint64(0)),
	})
	metrics.Update(
		vanflow.LinkRecord{
			BaseRecord: vanflow.NewBase("l01"),
			Role:       interRouter,
			Status:     statusDown,
			Parent:     ptrTo("r01"),
			DownCount:  ptrTo(uint64(0)),
		}, vanflow.LinkRecord{
			BaseRecord: vanflow.NewBase("l01"),
			Role:       interRouter,
			Status:     statusDown,
			Peer:       ptrTo("ap02"),
			Parent:     ptrTo("r01"),
			DownCount:  ptrTo(uint64(1)),
		},
	)
	// after delete, can still increment
	assert.Equal(t, prom_testutil.CollectAndCount(reg, "skupper_site_link_errors_total"), 1)
	assert.Equal(t, prom_testutil.ToFloat64(metrics.siteLinkErrors.WithLabelValues("s01", "inter-router")), 4.0)
}

func ptrTo[T any](s T) *T {
	return &s
}
