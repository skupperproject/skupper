package collector

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

type processManager struct {
	logger *slog.Logger
	stor   store.Interface
	graph  *graph
	idp    idProvider
	source store.SourceRef

	mu     sync.Mutex
	groups map[string]int

	siteHosts map[string]map[string]bool

	// connectors: connectorID -> host
	connectors map[string]string
	// siteID -> host -> connectorID
	expectedSiteHosts map[string]map[string]string
	// processID -> host
	processHosts map[string]string

	checkGroup        chan string
	rebuildConnectors chan struct{}
	rebuildProcesses  chan struct{}
}

func newProcessManager(logger *slog.Logger, stor store.Interface, graph *graph, idp idProvider) *processManager {
	return &processManager{
		logger: logger,
		idp:    idp,
		stor:   stor,
		graph:  graph,
		source: store.SourceRef{
			Version: "0.1",
			ID:      "self",
		},
		groups:    make(map[string]int),
		siteHosts: make(map[string]map[string]bool),

		connectors:        make(map[string]string),
		processHosts:      make(map[string]string),
		expectedSiteHosts: make(map[string]map[string]string),

		checkGroup:        make(chan string, 32),
		rebuildConnectors: make(chan struct{}, 1),
		rebuildProcesses:  make(chan struct{}, 1),
	}
}

func (m *processManager) run(ctx context.Context) func() error {
	return func() error {
		defer m.logger.Info("process manager shutdown complete")
		rebuild := time.NewTicker(60 * time.Second)
		var selector bool
		defer rebuild.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-rebuild.C:
				if selector {
					m.rebuildConnectors <- struct{}{}
				} else {
					m.rebuildProcesses <- struct{}{}
				}
				selector = !selector
			case <-m.rebuildProcesses:
				func() {
					m.mu.Lock()
					defer m.mu.Unlock()

					actualProcessHosts := make(map[string]string, len(m.processHosts))
					for siteID, hosts := range m.expectedSiteHosts {
						processes := m.stor.Index(IndexByTypeParent, store.Entry{Record: vanflow.ProcessRecord{Parent: &siteID}})
						phosts := make(map[string][]string, len(processes))
						for _, proc := range processes {
							if pr, ok := proc.Record.(vanflow.ProcessRecord); ok && pr.SourceHost != nil {
								sourceHost := *pr.SourceHost
								phosts[sourceHost] = append(phosts[sourceHost], pr.ID)
							}
						}
						expected := make(map[string]string, len(hosts))
						for host, cnctrID := range hosts {
							expected[host] = cnctrID
						}
						for h, cid := range expected {
							host, cnctrID := h, cid
							procIDs := phosts[host]
							switch len(procIDs) {
							case 1: // this is excellent. carry on.
								actualProcessHosts[procIDs[0]] = host
							case 0: // may need a site-process
								procID := m.idp.ID("siteproc", host, siteID)
								name := fmt.Sprintf("site-server-%s-%s", host, shortSite(siteID))
								groupName := fmt.Sprintf("site-servers-%s", shortSite(siteID))
								role := "external"
								m.stor.Add(vanflow.ProcessRecord{
									BaseRecord: vanflow.NewBase(procID, time.Now()),
									Parent:     &siteID,
									Name:       &name,
									Group:      &groupName,
									SourceHost: &host,
									Mode:       &role,
								}, m.source)
								m.logger.Info("Adding site server process for connector without suitable target",
									slog.String("id", procID),
									slog.String("name", name),
									slog.String("site_id", siteID),
									slog.String("host", host),
									slog.String("connector_id", cnctrID),
								)
								actualProcessHosts[procID] = host
							default: // more than one process. see about purging site processes
								var (
									toDelete   string
									replacedBy string
								)
								for _, procID := range procIDs {
									if procEntry, ok := m.stor.Get(procID); ok {
										if procEntry.Source == m.source {
											toDelete = procID
										} else {
											replacedBy = procID
											actualProcessHosts[procID] = host
										}
									}

								}
								if toDelete != "" {
									m.logger.Info("Deleting site server process superceeded by new process",
										slog.String("id", toDelete),
										slog.String("site_id", siteID),
										slog.String("host", host),
										slog.String("replaced_by", replacedBy),
									)
									m.stor.Delete(toDelete)
								}
							}
						}
						for _, proc := range processes {
							if proc.Source == m.source {
								_, ok := actualProcessHosts[proc.Record.Identity()]
								if ok {
									continue
								}
								if _, deleted := m.stor.Delete(proc.Record.Identity()); deleted {
									m.logger.Info("Deleting site server process with no connectors",
										slog.String("id", proc.Record.Identity()),
										slog.String("site_id", siteID),
									)
								}
							}
						}
					}
					m.processHosts = actualProcessHosts
				}()
			case <-m.rebuildConnectors:
				func() {
					m.mu.Lock()
					defer m.mu.Unlock()
					next := make(map[string]string, len(m.connectors))
					nextSiteHosts := make(map[string]map[string]string, len(m.expectedSiteHosts))
					current := m.connectors
					connectors := m.stor.Index(store.TypeIndex, store.Entry{Record: vanflow.ConnectorRecord{}})
					var hasChange bool
					for _, c := range connectors {
						if cr, ok := c.Record.(vanflow.ConnectorRecord); ok && cr.DestHost != nil {
							destHost := *cr.DestHost
							siteID := m.graph.Connector(cr.ID).Parent().Parent().ID()
							if siteID == "" {
								continue
							}
							next[cr.ID] = destHost
							hosts, ok := nextSiteHosts[siteID]
							if !ok {
								hosts = make(map[string]string)
								nextSiteHosts[siteID] = hosts
							}
							hosts[destHost] = cr.ID
							if prev, ok := current[cr.ID]; ok {
								delete(current, cr.ID)
								if prev != destHost {
									hasChange = true
								}
							} else {
								hasChange = true
							}
						}
					}
					if len(current) > 0 {
						hasChange = true
					}
					m.connectors = next
					m.expectedSiteHosts = nextSiteHosts

					if hasChange {
						m.rebuildProcesses <- struct{}{}
					}
				}()
			case groupName := <-m.checkGroup:
				func() {
					m.mu.Lock()
					defer m.mu.Unlock()
					ct := m.groups[groupName]
					groups := m.stor.Index(IndexByTypeName, store.Entry{Record: ProcessGroupRecord{Name: groupName}})
					if ct <= 0 {
						delete(m.groups, groupName)
						for _, g := range groups {
							m.logger.Info("Deleting process group with no processes",
								slog.String("id", g.Record.Identity()),
								slog.String("name", groupName),
							)
							m.stor.Delete(g.Record.Identity())
						}
						return
					}
					if len(groups) > 0 {
						return
					}
					id := m.idp.ID("pg", groupName)
					m.logger.Info("Creating process group",
						slog.String("id", id),
						slog.String("name", groupName),
					)
					m.stor.Add(ProcessGroupRecord{ID: id, Name: groupName, Start: time.Now()}, m.source)
				}()
			}
		}
	}
}

func (m *processManager) handleChangeEvent(event changeEvent, stor readonly) {
	var entry store.Entry
	var isDelete bool
	if d, ok := event.(deleteEvent); ok {
		entry = store.Entry{Record: d.Record}
		isDelete = true
	} else {
		entry, ok = stor.Get(event.ID())
		if !ok {
			return
		}
	}
	switch record := entry.Record.(type) {
	case vanflow.ConnectorRecord:
		if record.DestHost != nil {
			m.ensureConnector(record.ID, *record.DestHost, isDelete)
			// m.processPresent(m.graph.Connector(record.ID).Parent().Parent().ID(), *record.DestHost, !isDelete)
		}
	case vanflow.ProcessRecord:
		if record.Group != nil {
			m.ensureGroup(*record.Group, !isDelete)
		}
		if record.SourceHost != nil && record.Parent != nil {
			m.ensureProcessHost(record.ID, *record.SourceHost, isDelete)
			// m.processPresent(*record.Parent, *record.SourceHost, !isDelete)
		}
	}
}

func (m *processManager) ensureProcessHost(id, host string, deleted bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var rebuild bool
	if expected, ok := m.processHosts[id]; ok && deleted {
		rebuild = true
	} else if expected != host {
		rebuild = true
	}
	if !rebuild {
		return
	}
	select {
	case m.rebuildProcesses <- struct{}{}:
	default: // skip if full
	}
}

func (m *processManager) ensureConnector(id, host string, deleted bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var rebuild bool
	if expected, ok := m.connectors[id]; ok && deleted {
		rebuild = true
	} else if expected != host {
		rebuild = true
	}
	if !rebuild {
		return
	}
	select {
	case m.rebuildConnectors <- struct{}{}:
	default: // skip if full
	}
}

func (m *processManager) ensureGroup(name string, add bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var added bool
	if _, ok := m.groups[name]; !ok {
		m.groups[name] = 0
		added = true
	}
	if add {
		m.groups[name] = m.groups[name] + 1
	} else {
		m.groups[name] = m.groups[name] - 1
	}
	if m.groups[name] <= 0 || added {
		m.checkGroup <- name
	}
}

func shortSite(s string) string {
	return strings.Split(s, "-")[0]
}
