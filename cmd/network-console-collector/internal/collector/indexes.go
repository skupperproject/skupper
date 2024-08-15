package collector

import (
	"fmt"
	"time"

	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

var standardRecordTypes []vanflow.TypeMeta = []vanflow.TypeMeta{
	vanflow.SiteRecord{}.GetTypeMeta(),
	vanflow.RouterRecord{}.GetTypeMeta(),
	vanflow.LinkRecord{}.GetTypeMeta(),
	vanflow.RouterAccessRecord{}.GetTypeMeta(),
	vanflow.ConnectorRecord{}.GetTypeMeta(),
	vanflow.ListenerRecord{}.GetTypeMeta(),
	vanflow.ProcessRecord{}.GetTypeMeta(),
}

const (
	IndexByTypeParent      = "ByTypeAndParent"
	IndexByAddress         = "ByAddress"
	IndexByParentHost      = "ByParentHost"
	IndexByLifecycleStatus = "ByLifecycleStatus"
	IndexByTypeName        = "ByTypeAndName"
)

func indexByTypeName(e store.Entry) []string {
	optionalSingle := func(prefix string, s *string) []string {
		if s != nil {
			return []string{fmt.Sprintf("%s/%s", prefix, *s)}
		}
		return nil
	}
	switch record := e.Record.(type) {
	case ProcessGroupRecord:
		return optionalSingle(record.GetTypeMeta().String(), &record.Name)
	case vanflow.SiteRecord:
		return optionalSingle(record.GetTypeMeta().String(), record.Name)
	case vanflow.RouterRecord:
		return optionalSingle(record.GetTypeMeta().String(), record.Name)
	case vanflow.LinkRecord:
		return optionalSingle(record.GetTypeMeta().String(), record.Name)
	case vanflow.ListenerRecord:
		return optionalSingle(record.GetTypeMeta().String(), record.Name)
	case vanflow.ConnectorRecord:
		return optionalSingle(record.GetTypeMeta().String(), record.Name)
	case vanflow.ProcessRecord:
		return optionalSingle(record.GetTypeMeta().String(), record.Name)
	default:
		return nil
	}
}

func indexByParentHost(e store.Entry) []string {
	if proc, ok := e.Record.(vanflow.ProcessRecord); ok {
		if proc.Parent != nil && proc.SourceHost != nil {
			return []string{fmt.Sprintf("%s/%s", *proc.Parent, *proc.SourceHost)}
		}
	}
	return nil
}
func indexByTypeParent(e store.Entry) []string {
	optionalSingle := func(prefix string, s *string) []string {
		if s != nil {
			return []string{fmt.Sprintf("%s/%s", prefix, *s)}
		}
		return nil
	}
	switch record := e.Record.(type) {
	case vanflow.RouterRecord:
		return optionalSingle(record.GetTypeMeta().String(), record.Parent)
	case vanflow.LinkRecord:
		return optionalSingle(record.GetTypeMeta().String(), record.Parent)
	case vanflow.RouterAccessRecord:
		return optionalSingle(record.GetTypeMeta().String(), record.Parent)
	case vanflow.ConnectorRecord:
		return optionalSingle(record.GetTypeMeta().String(), record.Parent)
	case vanflow.ListenerRecord:
		return optionalSingle(record.GetTypeMeta().String(), record.Parent)
	case vanflow.ProcessRecord:
		return optionalSingle(record.GetTypeMeta().String(), record.Parent)
	default:
		return nil
	}
}
func indexByAddress(e store.Entry) []string {
	optionalSingle := func(s *string) []string {
		if s != nil {
			return []string{*s}
		}
		return nil
	}
	switch record := e.Record.(type) {
	case vanflow.ConnectorRecord:
		return optionalSingle(record.Address)
	case vanflow.ListenerRecord:
		return optionalSingle(record.Address)
	default:
		return nil
	}
}
func indexByLifecycleStatus(e store.Entry) []string {
	lifecycleState := func(b vanflow.BaseRecord) []string {
		var (
			started bool
			ended   bool
		)
		if b.StartTime != nil && b.StartTime.After(time.Unix(0, 0)) {
			started = true
		}
		if b.EndTime != nil && b.EndTime.After(time.Unix(0, 0)) {
			ended = true
		}
		switch {
		case !started && !ended:
			return []string{"INACTIVE"}
		case started && !ended:
			return []string{"ACTIVE"}
		default:
			return []string{"TERMINATED"}
		}
	}
	switch record := e.Record.(type) {
	case vanflow.SiteRecord:
		return lifecycleState(record.BaseRecord)
	case vanflow.RouterRecord:
		return lifecycleState(record.BaseRecord)
	case vanflow.LinkRecord:
		return lifecycleState(record.BaseRecord)
	case vanflow.RouterAccessRecord:
		return lifecycleState(record.BaseRecord)
	case vanflow.ConnectorRecord:
		return lifecycleState(record.BaseRecord)
	case vanflow.ListenerRecord:
		return lifecycleState(record.BaseRecord)
	case vanflow.ProcessRecord:
		return lifecycleState(record.BaseRecord)
	default:
		return nil
	}
}
