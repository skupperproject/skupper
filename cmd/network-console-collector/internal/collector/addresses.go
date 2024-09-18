package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

type addressManager struct {
	logger          *slog.Logger
	stor            store.Interface
	mu              sync.Mutex
	idp             idProvider
	addresses       map[string]struct{}
	updateAddresses chan AddressRecord
	source          store.SourceRef
}

func newAddressManager(logger *slog.Logger, stor store.Interface) *addressManager {
	return &addressManager{
		logger:          logger,
		stor:            stor,
		addresses:       make(map[string]struct{}),
		updateAddresses: make(chan AddressRecord, 16),
		idp:             newStableIdentityProvider(),
		source: store.SourceRef{
			Version: "0.1",
			ID:      "self",
		},
	}
}

func (m *addressManager) run(ctx context.Context) func() error {
	return func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case addr := <-m.updateAddresses:
				func() {
					m.mu.Lock()
					defer m.mu.Unlock()
					if _, ok := m.addresses[addr.ID]; ok {
						return
					}
					m.stor.Add(addr, m.source)
					m.addresses[addr.ID] = struct{}{}
					m.logger.Info("Creating address record",
						slog.String("id", addr.ID),
						slog.String("name", addr.Name),
					)
				}()
			}
		}
	}
}

func (m *addressManager) handleChangeEvent(event changeEvent, stor readonly) {
	if _, ok := event.(deleteEvent); ok {
		return
	}
	entry, ok := stor.Get(event.ID())
	if !ok {
		return
	}
	var (
		address   string
		startTime time.Time
		protocol  string = "tcp"
	)
	switch r := entry.Record.(type) {
	case vanflow.ListenerRecord:
		if r.Address != nil {
			address = *r.Address
		}
		if r.StartTime != nil {
			startTime = r.StartTime.Time
		}
		if r.Protocol != nil {
			protocol = *r.Protocol
		}
	case vanflow.ConnectorRecord:
		if r.Address != nil {
			address = *r.Address
		}
		if r.StartTime != nil {
			startTime = r.StartTime.Time
		}
		if r.Protocol != nil {
			protocol = *r.Protocol
		}
	default:
	}
	if address == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	addressID := m.idp.ID("adr", address, protocol)
	if _, ok := m.addresses[addressID]; ok {
		return
	}
	m.updateAddresses <- AddressRecord{ID: addressID, Name: address, Protocol: protocol, Start: startTime}
}
