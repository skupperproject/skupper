package collector

import (
	"time"

	"github.com/skupperproject/skupper/pkg/vanflow"
)

var _ vanflow.Record = AddressRecord{}

type AddressRecord struct {
	ID       string
	Name     string
	Protocol string
	Start    time.Time
}

func (r AddressRecord) Identity() string {
	return r.ID
}

func (r AddressRecord) GetTypeMeta() vanflow.TypeMeta {
	return vanflow.TypeMeta{
		Type:       "AddressRecord",
		APIVersion: "v1alpha1",
	}
}

type ProcessGroupRecord struct {
	ID    string
	Name  string
	Start time.Time
}

func (r ProcessGroupRecord) Identity() string {
	return r.ID
}

func (r ProcessGroupRecord) GetTypeMeta() vanflow.TypeMeta {
	return vanflow.TypeMeta{
		Type:       "ProcessGroupRecord",
		APIVersion: "v1alpha1",
	}
}

type SitePairRecord struct {
	ID       string
	Protocol string
	Source   string
	Dest     string
	Start    time.Time
}

func (r SitePairRecord) Identity() string {
	return r.ID
}
func (r SitePairRecord) GetTypeMeta() vanflow.TypeMeta {
	return vanflow.TypeMeta{
		Type:       "SitePairRecord",
		APIVersion: "v1alpha1",
	}
}

type ProcGroupPairRecord struct {
	ID       string
	Protocol string
	Source   string
	Dest     string
	Start    time.Time
}

func (r ProcGroupPairRecord) Identity() string {
	return r.ID
}

func (r ProcGroupPairRecord) GetTypeMeta() vanflow.TypeMeta {
	return vanflow.TypeMeta{
		Type:       "ProcGroupPairRecord",
		APIVersion: "v1alpha1",
	}
}

var _ vanflow.Record = (*ProcGroupPairRecord)(nil)

type ProcPairRecord struct {
	ID       string
	Start    time.Time
	Source   string
	Dest     string
	Protocol string
}

func (r ProcPairRecord) Identity() string {
	return r.ID
}
func (r ProcPairRecord) GetTypeMeta() vanflow.TypeMeta {
	return vanflow.TypeMeta{
		Type:       "ProcPairRecord",
		APIVersion: "v1alpha1",
	}
}

var _ vanflow.Record = (*ProcPairRecord)(nil)

type FlowSourceRecord struct {
	ID    string
	Site  string
	Host  string
	Start time.Time
}

func (r FlowSourceRecord) Identity() string {
	return r.ID
}

func (r FlowSourceRecord) GetTypeMeta() vanflow.TypeMeta {
	return vanflow.TypeMeta{
		Type:       "FlowSourceRecord",
		APIVersion: "v1alpha1",
	}
}
