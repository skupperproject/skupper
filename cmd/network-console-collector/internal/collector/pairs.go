package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

type pairManager struct {
	logger *slog.Logger
	stor   store.Interface
	graph  *graph
	idp    idProvider
	source store.SourceRef

	mu          sync.Mutex
	pairs       map[string]struct{}
	updatePairs chan ProcPairRecord
}

func newPairManager(logger *slog.Logger, stor store.Interface, graph *graph) *pairManager {
	return &pairManager{
		logger:      logger,
		stor:        stor,
		graph:       graph,
		idp:         newStableIdentityProvider(),
		updatePairs: make(chan ProcPairRecord, 8),
		pairs:       make(map[string]struct{}),
		source: store.SourceRef{
			Version: "0.1",
			ID:      "self",
		},
	}
}

func (m *pairManager) run(ctx context.Context) func() error {
	return func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case pair := <-m.updatePairs:
				func() {
					m.mu.Lock()
					defer m.mu.Unlock()
					if _, ok := m.pairs[pair.ID]; ok {
						return
					}

					sourceN := m.graph.Process(pair.Source).Parent()
					destN := m.graph.Process(pair.Dest).Parent()

					// add site and group pairs
					if sourceID, destID := sourceN.ID(), destN.ID(); sourceID != "" && destID != "" {
						spid := m.idp.ID("sp", sourceID, destID, pair.Protocol)
						m.stor.Add(SitePairRecord{
							ID:       spid,
							Start:    time.Now(),
							Protocol: pair.Protocol,
							Source:   sourceID,
							Dest:     destID,
						},
							m.source,
						)
					}

					pgIDByProcess := func(node Process) (string, bool) {
						record, ok := node.GetRecord()
						if !ok {
							return "", false
						}
						groups := m.stor.Index(IndexByTypeName, store.Entry{Record: ProcessGroupRecord{Name: dref(record.Name)}})
						if len(groups) > 0 {
							return groups[0].Record.Identity(), true
						}
						return "", false
					}
					sourcePG, sourceFound := pgIDByProcess(m.graph.Process(pair.Source))
					destPG, destFound := pgIDByProcess(m.graph.Process(pair.Dest))
					if sourceFound && destFound {
						pgpid := m.idp.ID("sp", sourcePG, destPG, pair.Protocol)
						m.stor.Add(SitePairRecord{
							ID:       pgpid,
							Start:    time.Now(),
							Protocol: pair.Protocol,
							Source:   sourcePG,
							Dest:     destPG,
						},
							m.source,
						)

					}
				}()
			}
		}
	}
}

func (m *pairManager) handleChangeEvent(event changeEvent, stor readonly) {
	if _, ok := event.(deleteEvent); ok {
		return
	}
	entry, ok := stor.Get(event.ID())
	if !ok {
		return
	}
	switch r := entry.Record.(type) {
	case ProcPairRecord:
		m.mu.Lock()
		defer m.mu.Unlock()
		if _, ok := m.pairs[r.ID]; ok {
			return
		}
		m.updatePairs <- r
	default:
	}

}
