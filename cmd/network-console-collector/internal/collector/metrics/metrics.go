package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/skupperproject/skupper/pkg/vanflow"
)

var (
	siteInfoMetricLabels = []string{
		"site_id",
		"name",
		"version",
	}
	routerInfoMetricLabels = []string{
		"site_id",
		"mode",
	}
	linkInfoMetricLabels = []string{
		"site_id",
		"role",
		"status",
	}
	linkErrorMetricLablels = []string{
		"site_id",
		"role",
	}
	siteIDMetricLabels = []string{"site_id"}
)

func New(reg prometheus.Registerer) *Adaptor {
	h := &Adaptor{
		sites:         make(map[siteInfo]prometheus.Gauge),
		routers:       make(map[siteRouters]*gaugeMetricByID),
		links:         make(map[siteLinks]*gaugeMetricByID),
		listeners:     make(map[string]*gaugeMetricByID),
		connectors:    make(map[string]*gaugeMetricByID),
		linkErrors:    make(map[siteLinkErrors]*counterMetricByItem),
		pendingRouter: make(map[string]map[string]vanflow.Record),
	}
	h.siteInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "skupper",
		Name:      "site_info",
		Help:      "Metadata about the active sites that make up the application network",
	}, siteInfoMetricLabels)
	h.routerInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "skupper",
		Name:      "routers_total",
		Help:      "Number of active routers participating in the application network",
	}, routerInfoMetricLabels)
	h.siteListenerInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "skupper",
		Name:      "site_listeners_total",
		Help:      "Number of listeners configured in the application network",
	}, siteIDMetricLabels)
	h.siteConnectorInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "skupper",
		Name:      "site_connectors_total",
		Help:      "Number of connectors configured in the application network",
	}, siteIDMetricLabels)
	h.siteLinkInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "skupper",
		Name:      "site_links_total",
		Help:      "Number of router links configured in the application network",
	}, linkInfoMetricLabels)
	h.siteLinkErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "skupper",
		Name:      "site_link_errors_total",
		Help:      "Count of link connection errors across the application network",
	}, linkErrorMetricLablels)

	reg.MustRegister(
		h.siteInfo,
		h.routerInfo,
		h.siteLinkInfo,
		h.siteListenerInfo,
		h.siteConnectorInfo,
		h.siteLinkErrors,
	)
	return h
}

// Adaptor between vanflow events and prometheus metrics. Maintains a set of
// coarse metrics pertaining to network topology.
type Adaptor struct {
	siteInfo          *prometheus.GaugeVec
	routerInfo        *prometheus.GaugeVec
	siteLinkInfo      *prometheus.GaugeVec
	siteListenerInfo  *prometheus.GaugeVec
	siteConnectorInfo *prometheus.GaugeVec
	siteLinkErrors    *prometheus.CounterVec

	sites      map[siteInfo]prometheus.Gauge
	routers    map[siteRouters]*gaugeMetricByID
	links      map[siteLinks]*gaugeMetricByID
	listeners  map[string]*gaugeMetricByID
	connectors map[string]*gaugeMetricByID
	linkErrors map[siteLinkErrors]*counterMetricByItem

	pendingRouter map[string]map[string]vanflow.Record

	routerSitesCache map[string]string
}

type counterMetricByItem struct {
	Items   map[string]int
	Counter prometheus.Counter
}

func (c *counterMetricByItem) Ensure(id string, ct int) {
	prev, ok := c.Items[id]
	if !ok {
		prev = ct
		c.Items[id] = prev
	}
	if ct <= prev {
		return
	}
	c.Items[id] = ct
	c.Counter.Add(float64(ct - prev))
}

func (m *counterMetricByItem) Remove(id string) {
	if _, ok := m.Items[id]; !ok {
		return
	}
	delete(m.Items, id)
}

type gaugeMetricByID struct {
	IDs   map[string]struct{}
	Gauge prometheus.Gauge
}

func (m *gaugeMetricByID) Ensure(id string) bool {
	if _, ok := m.IDs[id]; ok {
		return false
	}
	m.IDs[id] = struct{}{}
	m.Gauge.Add(1.0)
	return true
}

func (m *gaugeMetricByID) Remove(id string) {
	if _, ok := m.IDs[id]; !ok {
		return
	}
	delete(m.IDs, id)
	m.Gauge.Sub(1.0)
}

func toSiteInfo(site vanflow.SiteRecord) (siteInfo, bool) {
	var info siteInfo
	if site.Name == nil || site.Version == nil {
		return info, false
	}
	info.ID, info.Name, info.Version = site.ID, *site.Name, *site.Version
	return info, true
}

func routerInfo(router vanflow.RouterRecord) (siteRouters, bool) {
	if router.Parent == nil || router.Mode == nil {
		return siteRouters{}, false
	}
	return siteRouters{
		SiteID: *router.Parent,
		Mode:   *router.Mode,
	}, true
}

func (a *Adaptor) linkInfo(link vanflow.LinkRecord) (info siteLinks, wants string, ok bool) {
	if link.Parent == nil || link.Role == nil || link.Status == nil {
		return siteLinks{}, "", false
	}
	siteID, ok := a.siteIDForRotuer(*link.Parent)
	if !ok {
		return siteLinks{}, *link.Parent, false
	}
	return siteLinks{
		SiteID: siteID,
		Role:   *link.Role,
		Status: *link.Status,
	}, "", true
}

func (a *Adaptor) listenerInfo(listener vanflow.ListenerRecord) (siteID, wants string, ok bool) {
	if listener.Parent == nil {
		return "", "", false
	}
	siteID, ok = a.siteIDForRotuer(*listener.Parent)
	if !ok {
		return "", *listener.Parent, false
	}
	return siteID, "", true
}

func (a *Adaptor) connectorInfo(connector vanflow.ConnectorRecord) (siteID, wants string, ok bool) {
	if connector.Parent == nil {
		return "", "", false
	}
	siteID, ok = a.siteIDForRotuer(*connector.Parent)
	if !ok {
		return "", *connector.Parent, false
	}
	return siteID, "", true
}

func (a *Adaptor) siteIDForRotuer(rID string) (siteID string, ok bool) {
	if a.routerSitesCache == nil {
		a.routerSitesCache = make(map[string]string)
	}
	if siteID, ok := a.routerSitesCache[rID]; ok {
		return siteID, true
	}
	for site, routers := range a.routers {
		_, found := routers.IDs[rID]
		if found {
			ok = true
			siteID = site.SiteID
			break
		}
	}
	if ok {
		a.routerSitesCache[rID] = siteID
	}
	return siteID, ok
}

func (a *Adaptor) Add(record vanflow.Record) {
	switch record := record.(type) {
	case vanflow.SiteRecord:
		info, ok := toSiteInfo(record)
		if !ok {
			return
		}
		if _, ok := a.sites[info]; ok {
			return
		}
		a.sites[info] = a.siteInfo.With(info.asLabels())
		a.sites[info].Set(1.0)
	case vanflow.RouterRecord:
		siteRouter, ok := routerInfo(record)
		if !ok {
			return
		}
		metric, ok := a.routers[siteRouter]
		if !ok {
			metric = a.newSiteRouterMetrics(siteRouter)
			a.routers[siteRouter] = metric
		}
		if added := metric.Ensure(record.ID); added {
			dependencies, ok := a.pendingRouter[record.ID]
			if ok {
				delete(a.pendingRouter, record.ID)
			}
			for _, record := range dependencies {
				a.Add(record)
			}
		}
	case vanflow.LinkRecord:
		siteLink, wants, ok := a.linkInfo(record)
		if !ok {
			if wants != "" {
				a.addPendingRouter(wants, record)
			}
			return
		}
		metric, ok := a.links[siteLink]
		if !ok {
			metric = a.newSiteLinkMetrics(siteLink)
			a.links[siteLink] = metric
		}
		metric.Ensure(record.ID)

		if errCount := record.DownCount; errCount != nil {
			linkErrorKey := siteLinkErrors{
				SiteID: siteLink.SiteID,
				Role:   siteLink.Role,
			}
			counter, ok := a.linkErrors[linkErrorKey]
			if !ok {
				counter = a.newLinkErrorMetrics(linkErrorKey)
				a.linkErrors[linkErrorKey] = counter
			}
			counter.Ensure(record.ID, int(*errCount))
		}
	case vanflow.ListenerRecord:
		site, wants, ok := a.listenerInfo(record)
		if !ok {
			if wants != "" {
				a.addPendingRouter(wants, record)
			}
			return
		}
		metric, ok := a.listeners[site]
		if !ok {
			metric = a.newSiteListenerMetrics(site)
			a.listeners[site] = metric
		}
		metric.Ensure(record.ID)
	case vanflow.ConnectorRecord:
		site, wants, ok := a.connectorInfo(record)
		if !ok {
			if wants != "" {
				a.addPendingRouter(wants, record)
			}
			return
		}
		metric, ok := a.connectors[site]
		if !ok {
			metric = a.newSiteConnectorMetrics(site)
			a.connectors[site] = metric
		}
		metric.Ensure(record.ID)
	}
}

func (a *Adaptor) addPendingRouter(rID string, record vanflow.Record) {
	pending, ok := a.pendingRouter[rID]
	if !ok {
		pending = make(map[string]vanflow.Record)
		a.pendingRouter[rID] = pending
	}
	pending[record.Identity()] = record
}

func (a *Adaptor) clearPendingRouter(rID string, record vanflow.Record) {
	pending, ok := a.pendingRouter[rID]
	if !ok {
		return
	}
	delete(pending, record.Identity())
}

func (a *Adaptor) Update(prev, curr vanflow.Record) {
	switch record := curr.(type) {
	case vanflow.SiteRecord:
		a.Add(curr)
	case vanflow.RouterRecord:
		a.Add(curr)
	case vanflow.ListenerRecord:
		a.Add(curr)
	case vanflow.ConnectorRecord:
		a.Add(curr)
	case vanflow.LinkRecord:
		siteLink, wants, ok := a.linkInfo(record)
		if !ok {
			if wants != "" {
				a.addPendingRouter(wants, record)
			}
			return
		}
		prevLink, _, ok := a.linkInfo(prev.(vanflow.LinkRecord))
		if ok && prevLink != siteLink {
			a.Remove(prev)
		}
		metric, ok := a.links[siteLink]
		if !ok {
			metric = a.newSiteLinkMetrics(siteLink)
			a.links[siteLink] = metric
		}
		metric.Ensure(record.ID)
		if errCount := record.DownCount; errCount != nil {
			linkErrorKey := siteLinkErrors{
				SiteID: siteLink.SiteID,
				Role:   siteLink.Role,
			}
			counter, ok := a.linkErrors[linkErrorKey]
			if !ok {
				counter = a.newLinkErrorMetrics(linkErrorKey)
				a.linkErrors[linkErrorKey] = counter
			}
			counter.Ensure(record.ID, int(*errCount))
		}
	}
}

func (a *Adaptor) Remove(record vanflow.Record) {
	switch record := record.(type) {
	case vanflow.SiteRecord:
		info, ok := toSiteInfo(record)
		if !ok {
			return
		}
		m, ok := a.sites[info]
		if !ok {
			return
		}
		m.Set(0.0)
		a.sites[info] = nil
		delete(a.sites, info)
	case vanflow.RouterRecord:
		siteRouter, ok := routerInfo(record)
		if !ok {
			return
		}
		metric, ok := a.routers[siteRouter]
		if !ok {
			return
		}
		metric.Remove(record.ID)
	case vanflow.LinkRecord:
		siteLink, wants, ok := a.linkInfo(record)
		if !ok {
			if wants != "" {
				a.clearPendingRouter(wants, record)
			}
			return
		}
		metric, ok := a.links[siteLink]
		if !ok {
			return
		}
		metric.Remove(record.ID)
		linkErrorKey := siteLinkErrors{
			SiteID: siteLink.SiteID,
			Role:   siteLink.Role,
		}
		counter, ok := a.linkErrors[linkErrorKey]
		if !ok {
			return
		}
		counter.Remove(record.ID)
	case vanflow.ListenerRecord:
		site, wants, ok := a.listenerInfo(record)
		if !ok {
			if wants != "" {
				a.clearPendingRouter(wants, record)
			}
			return
		}
		metric, ok := a.listeners[site]
		if !ok {
			return
		}
		metric.Remove(record.ID)
	case vanflow.ConnectorRecord:
		site, wants, ok := a.connectorInfo(record)
		if !ok {
			if wants != "" {
				a.clearPendingRouter(wants, record)
			}
			return
		}
		metric, ok := a.connectors[site]
		if !ok {
			return
		}
		metric.Remove(record.ID)
	}
}

func (a *Adaptor) newSiteConnectorMetrics(siteID string) *gaugeMetricByID {
	return &gaugeMetricByID{
		IDs:   make(map[string]struct{}),
		Gauge: a.siteConnectorInfo.WithLabelValues(siteID),
	}
}
func (a *Adaptor) newSiteListenerMetrics(siteID string) *gaugeMetricByID {
	return &gaugeMetricByID{
		IDs:   make(map[string]struct{}),
		Gauge: a.siteListenerInfo.WithLabelValues(siteID),
	}
}
func (a *Adaptor) newSiteLinkMetrics(labels siteLinks) *gaugeMetricByID {
	return &gaugeMetricByID{
		IDs:   make(map[string]struct{}),
		Gauge: a.siteLinkInfo.With(labels.asLabels()),
	}
}
func (a *Adaptor) newSiteRouterMetrics(labels siteRouters) *gaugeMetricByID {
	return &gaugeMetricByID{
		IDs:   make(map[string]struct{}),
		Gauge: a.routerInfo.With(labels.asLabels()),
	}
}
func (a *Adaptor) newLinkErrorMetrics(labels siteLinkErrors) *counterMetricByItem {
	return &counterMetricByItem{
		Items:   make(map[string]int),
		Counter: a.siteLinkErrors.With(labels.asLabels()),
	}
}

type siteInfo struct {
	ID      string
	Name    string
	Version string
}

func (i siteInfo) asLabels() prometheus.Labels {
	return prometheus.Labels{
		"site_id": i.ID,
		"name":    i.Name,
		"version": i.Version,
	}
}

type siteRouters struct {
	SiteID string
	Mode   string
}

func (i siteRouters) asLabels() prometheus.Labels {
	return prometheus.Labels{
		"site_id": i.SiteID,
		"mode":    i.Mode,
	}
}

type siteLinks struct {
	SiteID string
	Role   string
	Status string
}

func (i siteLinks) asLabels() prometheus.Labels {
	return prometheus.Labels{
		"site_id": i.SiteID,
		"role":    i.Role,
		"status":  i.Status,
	}
}

type siteLinkErrors struct {
	SiteID string
	Role   string
}

func (i siteLinkErrors) asLabels() prometheus.Labels {
	return prometheus.Labels{
		"site_id": i.SiteID,
		"role":    i.Role,
	}
}
