package flow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/skupperproject/skupper/internal/network"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/eventsource"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"
)

var recordTypes []vanflow.Record = []vanflow.Record{
	vanflow.SiteRecord{},
	vanflow.RouterRecord{},
	vanflow.LinkRecord{},
	vanflow.RouterAccessRecord{},
	vanflow.ConnectorRecord{},
	vanflow.ListenerRecord{},
	vanflow.ProcessRecord{},
}

type StatusSync struct {
	records       store.Interface
	recordMapping eventsource.RecordStoreMap

	session       session.Container
	discovery     *eventsource.Discovery
	client        StatusSyncClient
	configMapName string

	logger *slog.Logger
	ctx    context.Context

	mu      sync.Mutex
	clients map[string]*eventsource.Client

	hasNext    chan struct{}
	purgeQueue chan store.SourceRef
	limit      rate.Limit
	burst      int
}

type StatusSyncClient interface {
	Logger() *slog.Logger
	Get(ctx context.Context) (*corev1.ConfigMap, error)
	Update(ctx context.Context, latest *corev1.ConfigMap) error
}

func NewStatusSync(factory session.ContainerFactory, localSources map[string][]store.Interface, client StatusSyncClient, configMap string) *StatusSync {

	logger := client.Logger()
	sessionCtr := factory.Create()
	sessionCtr.OnSessionError(func(err error) {
		logger.Error("session error on discovery container", slog.Any("error", err))
	})
	discovery := eventsource.NewDiscovery(sessionCtr, eventsource.DiscoveryOptions{})

	s := &StatusSync{
		session:       sessionCtr,
		discovery:     discovery,
		configMapName: configMap,
		client:        client,

		recordMapping: make(eventsource.RecordStoreMap, len(recordTypes)),
		clients:       make(map[string]*eventsource.Client),
		purgeQueue:    make(chan store.SourceRef, 8),
		hasNext:       make(chan struct{}, 1),
		limit:         rate.Every(time.Second),
		burst:         1,

		logger: logger,
	}
	recordsStore := store.NewSyncMapStore(store.SyncMapStoreConfig{
		Handlers: store.EventHandlerFuncs{
			OnAdd:    s.recordAdded,
			OnChange: s.recordChanged,
			OnDelete: s.recordDeleted,
		},
		Indexers: map[string]store.Indexer{
			store.SourceIndex: store.SourceIndexer,
			store.TypeIndex:   store.TypeIndexer,
			byTypeParent:      indexByTypeParent,
			byAddress:         indexByAddress,
			byParentHost:      indexByParentHost,
		},
	})

	for _, record := range recordTypes {
		s.recordMapping[record.GetTypeMeta().String()] = recordsStore
	}

	s.records = recordsStore

	return s
}

func (s *StatusSync) Run(ctx context.Context) {
	s.session.Start(ctx)
	s.ctx = ctx
	discoveryErr := make(chan error, 1)
	go func() {
		discoveryErr <- s.discovery.Run(ctx, eventsource.DiscoveryHandlers{
			Discovered: s.handleDiscovery,
			Forgotten:  s.handleForgotten,
		})
	}()

	var (
		prev  network.NetworkStatusInfo
		later <-chan time.Time
	)

	tryBuildAndPublish := func() {
		var err error
		prev, err = s.buildAndPublish(prev)
		if err != nil {
			s.logger.Error("could not update network status info", slog.Any("error", err))
		}
	}
	var limiter *rate.Limiter
	if s.limit != 0 && s.burst != 0 {
		limiter = rate.NewLimiter(s.limit, s.burst)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-discoveryErr:
			if ctx.Err() != nil {
				return
			}
			s.logger.Error("event source discovery finished unexpectedly", slog.Any("error", err))

		case <-s.hasNext:
			if limiter != nil {
				r := limiter.Reserve()
				if d := r.Delay(); d != 0 {
					// rate limit reached
					if later == nil {
						later = time.After(d) // schedule next update
					} else {
						r.Cancel() // future update already scheduled
					}
					continue
				}
			}
			tryBuildAndPublish()

		case <-later:
			later = nil
			tryBuildAndPublish()
		case source := <-s.purgeQueue:
			ct := s.purge(source)
			s.logger.Info("purged records from forgotten source",
				slog.String("source", source.ID),
				slog.Int("count", ct),
			)
		}
	}
}

func (s *StatusSync) buildAndPublish(prev network.NetworkStatusInfo) (network.NetworkStatusInfo, error) {
	next := s.build()
	if cmp.Equal(prev, next) {
		s.logger.Debug("no change since last publish")
		return prev, nil
	}
	s.logger.Info("updating network status info", slog.String("configmap", s.configMapName))
	return next, s.publish(next)
}

func (s *StatusSync) build() network.NetworkStatusInfo {
	var (
		info         network.NetworkStatusInfo
		siteExemplar = store.Entry{Record: vanflow.SiteRecord{}}
	)

	sorted := func(entries []store.Entry) []store.Entry {
		slices.SortFunc(entries, func(a, b store.Entry) int {
			return strings.Compare(a.Record.Identity(), b.Record.Identity())
		})
		return entries
	}

	siteEntries := sorted(s.records.Index(store.TypeIndex, siteExemplar))
	for _, entry := range siteEntries {
		var siteInfo network.SiteStatusInfo
		site := entry.Record.(vanflow.SiteRecord)
		siteID := entry.Record.Identity()
		siteInfo.Site = asSiteInfo(site)

		exemplar := store.Entry{Record: vanflow.RouterRecord{
			Parent: &siteID,
		}}
		routerEntries := sorted(s.records.Index(byTypeParent, exemplar))
		for _, routerEnt := range routerEntries {
			routerID := routerEnt.Record.Identity()
			routerRecord := routerEnt.Record.(vanflow.RouterRecord)
			var routerInfo network.RouterStatusInfo
			routerInfo.Router = asRouterInfo(routerRecord)

			linkExemplar := store.Entry{Record: vanflow.LinkRecord{
				Parent: &routerID,
			}}
			linkEntries := sorted(s.records.Index(byTypeParent, linkExemplar))
			routerInfo.Links = make([]network.LinkInfo, len(linkEntries))
			for i, linkEnt := range linkEntries {
				routerInfo.Links[i] = asLinkInfo(linkEnt.Record.(vanflow.LinkRecord))
			}

			accessExemplar := store.Entry{Record: vanflow.RouterAccessRecord{
				Parent: &routerID,
			}}
			accessEntries := sorted(s.records.Index(byTypeParent, accessExemplar))
			routerInfo.AccessPoints = make([]network.RouterAccessInfo, len(accessEntries))
			for i, accessEnt := range accessEntries {
				routerInfo.AccessPoints[i] = asRouterAccessInfo(accessEnt.Record.(vanflow.RouterAccessRecord))
			}

			connExemplar := store.Entry{Record: vanflow.ConnectorRecord{
				Parent: &routerID,
			}}
			connEntries := sorted(s.records.Index(byTypeParent, connExemplar))
			routerInfo.Connectors = make([]network.ConnectorInfo, len(connEntries))
			for i, connEnt := range connEntries {
				connector := connEnt.Record.(vanflow.ConnectorRecord)
				connectorInfo := asConnectorInfo(connector)

				var matchedProc bool
				if connector.ProcessID != nil {
					if procEntry, ok := s.records.Get(*connector.ProcessID); ok {
						matchedProc = true
						proc := procEntry.Record.(vanflow.ProcessRecord)
						if proc.Name != nil {
							connectorInfo.Target = *proc.Name
						}
					}
				}
				if !matchedProc {
					procExemplar := store.Entry{Record: vanflow.ProcessRecord{
						Parent:     &siteID,
						SourceHost: connector.DestHost,
					}}
					procs := s.records.Index(byParentHost, procExemplar)
					if len(procs) > 0 {
						first := procs[0].Record.(vanflow.ProcessRecord)
						connectorInfo.Process = first.ID
						if first.Name != nil {
							connectorInfo.Target = *first.Name
						}
					}
				}
				routerInfo.Connectors[i] = connectorInfo
			}

			lstExemplar := store.Entry{Record: vanflow.ListenerRecord{
				Parent: &routerID,
			}}
			lstEntries := sorted(s.records.Index(byTypeParent, lstExemplar))
			routerInfo.Listeners = make([]network.ListenerInfo, len(lstEntries))
			for i, lstEnt := range lstEntries {
				routerInfo.Listeners[i] = asListenerInfo(lstEnt.Record.(vanflow.ListenerRecord))
			}

			siteInfo.RouterStatus = append(siteInfo.RouterStatus, routerInfo)
		}

		info.SiteStatus = append(info.SiteStatus, siteInfo)
	}

	addresses := s.records.IndexValues(byAddress)
	slices.Sort(addresses)
	for _, address := range addresses {
		var (
			connectors []vanflow.ConnectorRecord
			listeners  []vanflow.ListenerRecord
		)
		entries := sorted(s.records.Index(byAddress, store.Entry{
			Record: vanflow.ConnectorRecord{Address: &address},
		}))

		for _, entry := range entries {
			switch record := entry.Record.(type) {
			case vanflow.ConnectorRecord:
				connectors = append(connectors, record)
			case vanflow.ListenerRecord:
				listeners = append(listeners, record)
			}
		}

		info.Addresses = append(info.Addresses, asAddressInfo(address, connectors, listeners))
	}

	return info
}

func (s *StatusSync) publish(info network.NetworkStatusInfo) error {
	ctx, cancel := context.WithTimeout(s.ctx, time.Second*10)
	defer cancel()
	bs, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal network info: %s", err)
	}
	networkStatus := string(bs)
	data := map[string]string{"NetworkStatus": networkStatus}
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.client.Get(ctx)
		if err != nil {
			return err
		}
		current.Data = data
		err = s.client.Update(ctx, current)
		if err != nil {
			s.logger.Error("updating network status", slog.Any("error", err))
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to update configmap: %s", err)
	}
	return nil
}

func (s *StatusSync) recordAdded(entry store.Entry) {
	s.recordChanged(store.Entry{}, entry)
}

func (s *StatusSync) recordChanged(prev, entry store.Entry) {
	switch record := entry.Record.(type) {
	case vanflow.SiteRecord:
		if isEndTimeSet(record.EndTime) {
			s.records.Delete(record.Identity())
		}
	case vanflow.RouterRecord:
		if isEndTimeSet(record.EndTime) {
			s.records.Delete(record.Identity())
		}
	case vanflow.LinkRecord:
		if isEndTimeSet(record.EndTime) {
			s.records.Delete(record.Identity())
		}
	case vanflow.RouterAccessRecord:
		if isEndTimeSet(record.EndTime) {
			s.records.Delete(record.Identity())
		}
	case vanflow.ConnectorRecord:
		if isEndTimeSet(record.EndTime) {
			s.records.Delete(record.Identity())
		}
	case vanflow.ListenerRecord:
		if isEndTimeSet(record.EndTime) {
			s.records.Delete(record.Identity())
		}
	case vanflow.ProcessRecord:
		if isEndTimeSet(record.EndTime) {
			s.records.Delete(record.Identity())
		}
	}

	s.notify()
}
func (s *StatusSync) recordDeleted(entry store.Entry) {
	s.notify()
}

func (s *StatusSync) notify() {
	select {
	case s.hasNext <- struct{}{}:
	default:
	}
}

func (s *StatusSync) purge(source store.SourceRef) int {
	matching := s.records.Index(store.SourceIndex, store.Entry{Metadata: store.Metadata{Source: source}})
	for _, record := range matching {
		s.records.Delete(record.Record.Identity())
	}
	return len(matching)
}

func (s *StatusSync) handleDiscovery(source eventsource.Info) {
	client := eventsource.NewClient(s.session, eventsource.ClientOptions{
		Source: source,
	})

	// register client with discovery to update lastseen, and monitor for staleness
	err := s.discovery.NewWatchClient(s.ctx, eventsource.WatchConfig{
		Client:      client,
		ID:          source.ID,
		Timeout:     time.Second * 30,
		GracePeriod: time.Second * 30,
	})

	if err != nil {
		s.logger.Error("error creating watcher for discoverd source", slog.Any("error", err))
		s.discovery.Forget(source.ID)
		return
	}

	router := eventsource.RecordStoreRouter{
		Stores: s.recordMapping,
		Source: sourceRef(source),
	}
	client.OnRecord(router.Route)
	client.Listen(s.ctx, eventsource.FromSourceAddress())
	if source.Type == "CONTROLLER" {
		client.Listen(s.ctx, eventsource.FromSourceAddressHeartbeats())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[source.ID] = client

	go func() {
		ctx, cancel := context.WithTimeout(s.ctx, time.Second*5)
		defer cancel()
		if err := eventsource.FlushOnFirstMessage(ctx, client); err != nil {
			if errors.Is(err, ctx.Err()) {
				s.logger.Info("timed out waiting for first message. sending flush anyways")
				err = client.SendFlush(s.ctx)
			}
			if err != nil {
				s.logger.Error("error sending flush", slog.Any("error", err))
			}
		}
	}()
}

func (s *StatusSync) handleForgotten(source eventsource.Info) {
	s.mu.Lock()
	defer s.mu.Unlock()
	client, ok := s.clients[source.ID]
	if ok {
		client.Close()
		delete(s.clients, source.ID)
	}
	s.purgeQueue <- sourceRef(source)
}

func sourceRef(source eventsource.Info) store.SourceRef {
	return store.SourceRef{
		Version: fmt.Sprint(source.Version),
		ID:      source.ID,
	}
}

func isEndTimeSet(endtime *vanflow.Time) bool {
	return endtime != nil && endtime.After(time.Unix(1, 0))
}

func asAddressInfo(address string, connectors []vanflow.ConnectorRecord, listeners []vanflow.ListenerRecord) network.AddressInfo {
	var addressInfo network.AddressInfo
	addressInfo.Name = address
	for _, connector := range connectors {
		if connector.Protocol != nil {
			addressInfo.Protocol = *connector.Protocol
			break
		}
	}
	addressInfo.ConnectorCount = len(connectors)
	addressInfo.ListenerCount = len(listeners)
	return addressInfo
}

func asLinkInfo(link vanflow.LinkRecord) network.LinkInfo {
	return network.LinkInfo{
		Name:     dref(link.Name),
		Status:   dref(link.Status),
		LinkCost: dref(link.LinkCost),
		Role:     dref(link.Role),
		Peer:     dref(link.Peer),
	}
}

func asRouterAccessInfo(link vanflow.RouterAccessRecord) network.RouterAccessInfo {
	return network.RouterAccessInfo{
		Identity: link.ID,
	}
}

func asConnectorInfo(connector vanflow.ConnectorRecord) network.ConnectorInfo {
	return network.ConnectorInfo{
		DestHost: dref(connector.DestHost),
		DestPort: dref(connector.DestPort),
		Address:  dref(connector.Address),
		Process:  dref(connector.ProcessID),
	}
}

func asListenerInfo(listener vanflow.ListenerRecord) network.ListenerInfo {
	return network.ListenerInfo{
		//TODO(ck) Name not in spec? Name: dref(listener.Name),
		DestHost: dref(listener.DestHost),
		DestPort: dref(listener.DestPort),
		Protocol: dref(listener.Protocol),
		Address:  dref(listener.Address),
		Name:     dref(listener.Name),
	}
}

func asSiteInfo(site vanflow.SiteRecord) network.SiteInfo {
	return network.SiteInfo{
		Identity:  site.ID,
		Name:      dref(site.Name),
		Namespace: dref(site.Namespace),
		Platform:  dref(site.Platform),
		Version:   dref(site.Version),
	}
}

func asRouterInfo(router vanflow.RouterRecord) network.RouterInfo {
	return network.RouterInfo{
		Name:         dref(router.Name),
		Namespace:    dref(router.Namespace),
		Mode:         dref(router.Mode),
		ImageName:    dref(router.ImageName),
		ImageVersion: dref(router.ImageVersion),
		Hostname:     dref(router.Hostname),
	}
}

func dref[T any, R *T](ptr R) T {
	var out T
	if ptr != nil {
		out = *ptr
	}
	return out
}
