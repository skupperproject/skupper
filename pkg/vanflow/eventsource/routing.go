package eventsource

import (
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

type RecordStoreMap map[string]store.Interface

type RecordStoreRouter struct {
	Source store.SourceRef
	Stores RecordStoreMap
}

func (r RecordStoreRouter) Route(msg vanflow.RecordMessage) {
	for _, record := range msg.Records {
		typ := record.GetTypeMeta().String()
		acc, found := r.Stores[typ]
		if !found {
			continue
		}
		acc.Patch(record, r.Source)
	}
}
