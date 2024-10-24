package collector

import (
	"time"

	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

var _ vanflow.Record = (*ConnectionRecord)(nil)

type ConnectionRecord struct {
	ID        string
	StartTime time.Time
	EndTime   time.Time

	Connector     NamedReference
	RoutingKey    string
	Protocol      string
	ConnectorHost string
	ConnectorPort string

	Listener     NamedReference
	Source       NamedReference
	Dest         NamedReference
	SourceSite   NamedReference
	DestSite     NamedReference
	SourceGroup  NamedReference
	DestGroup    NamedReference
	SourceRouter NamedReference
	DestRouter   NamedReference

	// FlowStore is the backing store containing the Biflow records. This was
	// split from the main record store to keep high volume flow producers from
	// affecting the rest of the event sources.
	FlowStore store.Interface
	metrics   transportMetrics
}

func (cr *ConnectionRecord) GetFlow() (vanflow.TransportBiflowRecord, bool) {
	var record vanflow.TransportBiflowRecord
	ent, ok := cr.FlowStore.Get(cr.ID)
	if !ok {
		return record, false
	}
	record, ok = ent.Record.(vanflow.TransportBiflowRecord)
	return record, ok
}

func (r ConnectionRecord) Identity() string {
	return r.ID
}
func (r ConnectionRecord) GetTypeMeta() vanflow.TypeMeta {
	return vanflow.TypeMeta{
		Type:       "ConnectionRecord",
		APIVersion: "v2alpha1",
	}
}

func (r ConnectionRecord) toLabelSet() labelSet {
	return labelSet{
		SourceSiteID:        r.SourceSite.ID,
		DestSiteID:          r.DestSite.ID,
		SourceSiteName:      r.SourceSite.Name,
		DestSiteName:        r.DestSite.Name,
		RoutingKey:          r.RoutingKey,
		Protocol:            r.Protocol,
		SourceProcess:       r.Source.Name,
		DestProcess:         r.Dest.Name,
		SourceComponentID:   r.SourceGroup.ID,
		SourceComponentName: r.SourceGroup.Name,
		DestComponentID:     r.DestGroup.ID,
		DestComponentName:   r.DestGroup.Name,
	}
}

var _ vanflow.Record = (*RequestRecord)(nil)

type RequestRecord struct {
	ID          string
	TransportID string
	StartTime   time.Time
	EndTime     time.Time

	RoutingKey string
	Protocol   string

	Connector    NamedReference
	Listener     NamedReference
	Source       NamedReference
	Dest         NamedReference
	SourceSite   NamedReference
	SourceRouter NamedReference
	DestSite     NamedReference
	DestRouter   NamedReference
	SourceGroup  NamedReference
	DestGroup    NamedReference
	Trace        string

	stor    store.Interface
	metrics appMetrics
}

func (cr *RequestRecord) GetFlow() (vanflow.AppBiflowRecord, bool) {
	var record vanflow.AppBiflowRecord
	ent, ok := cr.stor.Get(cr.ID)
	if !ok {
		return record, false
	}
	record, ok = ent.Record.(vanflow.AppBiflowRecord)
	return record, ok
}
func (cr *RequestRecord) GetTransport() (vanflow.TransportBiflowRecord, bool) {
	var record vanflow.TransportBiflowRecord
	ent, ok := cr.stor.Get(cr.TransportID)
	if !ok {
		return record, false
	}
	record, ok = ent.Record.(vanflow.TransportBiflowRecord)
	return record, ok
}

func (r RequestRecord) Identity() string {
	return r.ID
}
func (r RequestRecord) GetTypeMeta() vanflow.TypeMeta {
	return vanflow.TypeMeta{
		Type:       "RequestRecord",
		APIVersion: "v2alpha1",
	}
}

func (r RequestRecord) toLabelSet() labelSet {
	return labelSet{
		SourceSiteID:        r.SourceSite.ID,
		DestSiteID:          r.DestSite.ID,
		SourceSiteName:      r.SourceSite.Name,
		DestSiteName:        r.DestSite.Name,
		RoutingKey:          r.RoutingKey,
		SourceProcess:       r.Source.Name,
		DestProcess:         r.Dest.Name,
		SourceComponentID:   r.SourceGroup.ID,
		SourceComponentName: r.SourceGroup.Name,
		DestComponentID:     r.DestGroup.ID,
		DestComponentName:   r.DestGroup.Name,
		Protocol:            r.Protocol,
	}
}

type NamedReference struct {
	ID   string
	Name string
}
