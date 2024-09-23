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
		refresh := time.NewTicker(time.Second * 20)
		defer refresh.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-refresh.C:
				go func() {
					processPairs := m.stor.Index(store.TypeIndex, store.Entry{Record: ProcPairRecord{}})
					for _, entry := range processPairs {
						if pair, ok := entry.Record.(ProcPairRecord); ok {
							m.updatePairs <- pair
						}
					}
				}()
			case pair := <-m.updatePairs:
				func() {
					m.mu.Lock()
					defer m.mu.Unlock()
					var (
						hasSitePair  bool
						hasGroupPair bool
					)
					if _, ok := m.pairs[pair.ID]; ok {
						return
					}

					sourceN := m.graph.Process(pair.Source).Parent()
					destN := m.graph.Process(pair.Dest).Parent()

					// add site and group pairs
					if sourceID, destID := sourceN.ID(), destN.ID(); sourceID != "" && destID != "" {
						hasSitePair = true
						spid := m.idp.ID("sitepair", sourceID, destID, pair.Protocol)
						added := m.stor.Add(SitePairRecord{
							ID:       spid,
							Start:    time.Now(),
							Protocol: pair.Protocol,
							Source:   sourceID,
							Dest:     destID,
						},
							m.source,
						)
						if added {
							m.logger.Info("Added site pair", slog.String("id", spid), slog.String("source", sourceID), slog.String("dest", destID))
						}
					}

					pgIDByProcess := func(node Process) (string, string, bool) {
						record, ok := node.GetRecord()
						if !ok {
							return "", "", false
						}
						groups := m.stor.Index(IndexByTypeName, store.Entry{Record: ProcessGroupRecord{Name: dref(record.Group)}})
						if len(groups) > 0 {
							if pg, ok := groups[0].Record.(ProcessGroupRecord); ok {
								return pg.ID, pg.Name, true
							}
						}
						return "", "", false
					}
					sourcePG, sourcePGName, sourceFound := pgIDByProcess(m.graph.Process(pair.Source))
					destPG, destPGName, destFound := pgIDByProcess(m.graph.Process(pair.Dest))
					if sourceFound && destFound {
						hasGroupPair = true
						pgpid := m.idp.ID("grouppair", sourcePG, destPG, pair.Protocol)
						added := m.stor.Add(ProcGroupPairRecord{
							ID:         pgpid,
							Start:      time.Now(),
							Protocol:   pair.Protocol,
							Source:     sourcePG,
							SourceName: sourcePGName,
							Dest:       destPG,
							DestName:   destPGName,
						},
							m.source,
						)
						if added {
							m.logger.Info(
								"Added process group pair",
								slog.String("id", pgpid),
								slog.String("source", sourcePG),
								slog.String("dest", destPG))
						}

					}
					if hasSitePair && hasGroupPair {
						m.pairs[pair.ID] = struct{}{}
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
