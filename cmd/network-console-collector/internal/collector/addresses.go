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
	logger              *slog.Logger
	stor                store.Interface
	mu                  sync.Mutex
	idp                 idProvider
	addresses           map[string]struct{}
	updateAddresses     chan AddressRecord
	maybePurgeAddresses chan AddressRecord
	source              store.SourceRef
}

func newAddressManager(logger *slog.Logger, stor store.Interface) *addressManager {
	return &addressManager{
		logger:              logger,
		stor:                stor,
		addresses:           make(map[string]struct{}),
		updateAddresses:     make(chan AddressRecord, 16),
		maybePurgeAddresses: make(chan AddressRecord, 16),
		idp:                 newStableIdentityProvider(),
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
			case addr := <-m.maybePurgeAddresses:
				func() {
					m.mu.Lock()
					defer m.mu.Unlock()
					if _, ok := m.addresses[addr.ID]; !ok {
						return
					}
					addressName := addr.Name
					entries := m.stor.Index(IndexByAddress, store.Entry{Record: vanflow.ConnectorRecord{Address: &addressName}})
					if len(entries) > 0 {
						return
					}
					m.stor.Delete(addr.ID)
					delete(m.addresses, addr.ID)
					m.logger.Info("Deleted address record",
						slog.String("id", addr.ID),
						slog.String("name", addr.Name),
					)
				}()
			}
		}
	}
}

func (m *addressManager) handleChangeEvent(event changeEvent, stor readonly) {
	if delEvent, ok := event.(deleteEvent); ok {
		address, ok := m.addressFromRecord(delEvent.Record)
		if !ok {
			return
		}
		if !m.hasAddress(address) {
			return
		}
		m.maybePurgeAddresses <- address
		return
	}
	entry, ok := stor.Get(event.ID())
	if !ok {
		return
	}
	address, ok := m.addressFromRecord(entry.Record)
	if !ok {
		return // incomplete
	}
	if !m.hasAddress(address) {
		m.updateAddresses <- address
	}
}

func (m *addressManager) hasAddress(address AddressRecord) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	addressID := m.idp.ID("adr", address.Name, address.Protocol)
	_, ok := m.addresses[addressID]
	return ok
}

func (m *addressManager) addressFromRecord(record vanflow.Record) (AddressRecord, bool) {
	var addr AddressRecord
	addr.Start = time.Now()
	switch r := record.(type) {
	case vanflow.ListenerRecord:
		if r.Address != nil {
			addr.Name = *r.Address
		}
		if r.StartTime != nil {
			addr.Start = r.StartTime.Time
		}
		if r.Protocol != nil {
			addr.Protocol = *r.Protocol
		}
	case vanflow.ConnectorRecord:
		if r.Address != nil {
			addr.Name = *r.Address
		}
		if r.StartTime != nil {
			addr.Start = r.StartTime.Time
		}
		if r.Protocol != nil {
			addr.Protocol = *r.Protocol
		}
	default:
		return addr, false
	}
	if addr.Name == "" || addr.Protocol == "" {
		return addr, false
	}
	addr.ID = m.idp.ID("adr", addr.Name, addr.Protocol)
	return addr, true
}
