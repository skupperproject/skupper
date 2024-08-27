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

	Address  string
	Protocol string

	Connector  NamedReference
	Listener   NamedReference
	Source     NamedReference
	Dest       NamedReference
	SourceSite NamedReference
	DestSite   NamedReference

	stor    store.Interface
	metrics transportMetrics
}

func (cr *ConnectionRecord) GetFlow() (vanflow.TransportBiflowRecord, bool) {
	var record vanflow.TransportBiflowRecord
	ent, ok := cr.stor.Get(cr.ID)
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
		APIVersion: "v1alpha1",
	}
}

var _ vanflow.Record = (*RequestRecord)(nil)

type RequestRecord struct {
	ID          string
	TransportID string
	StartTime   time.Time
	EndTime     time.Time

	Address  string
	Protocol string

	Connector  NamedReference
	Listener   NamedReference
	Source     NamedReference
	Dest       NamedReference
	SourceSite NamedReference
	DestSite   NamedReference

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

func (r RequestRecord) Identity() string {
	return r.ID
}
func (r RequestRecord) GetTypeMeta() vanflow.TypeMeta {
	return vanflow.TypeMeta{
		Type:       "RequestRecord",
		APIVersion: "v1alpha1",
	}
}

type NamedReference struct {
	ID   string
	Name string
}
