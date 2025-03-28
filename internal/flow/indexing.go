package flow

import (
	"fmt"

	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

const (
	byTypeParent = "ByTypeAndParent"
	byAddress    = "ByAddress"
	byParentHost = "ByParentHost"
)

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
