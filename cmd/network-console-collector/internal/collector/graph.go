package collector

import (
	"fmt"

	"github.com/heimdalr/dag"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

// Graph exposes more complex relations between record types than can be
// represented by store indexes and is more concice than would be possible
// fetching each individual record from a store along a chain of relations.
//
// Each node type has a set of exposed relations, none of which are guaranteed
// to exist or be present in the backing store. To keep error checking to a
// minimum, all relations will return a valid node regardless if the relation
// exists or is present in the store. As an example, with an empty graph
// `graph.Link("doesnotexist").Parent().Parent() will return a Site node, but
// calling Get() on that node will return false.
type Graph interface {
	Address(id string) Address
	Connector(id string) Connector
	ConnectorTarget(id string) ConnectorTarget
	Link(id string) Link
	Listener(id string) Listener
	Process(id string) Process
	RouterAccess(id string) RouterAccess
	Site(id string) Site
}

type graph struct {
	dag  *dag.DAG
	stor store.Interface
}

func NewGraph(stor store.Interface) Graph {
	return &graph{
		dag:  dag.NewDAG(),
		stor: stor,
	}
}

func (g *graph) Reset() {
	g.dag = dag.NewDAG()
	for _, e := range g.stor.List() {
		g.Reindex(e.Record)
	}
}

func (g *graph) Site(id string) Site {
	return vertexByType[Site](g.dag, id)
}
func (g *graph) Process(id string) Process {
	return vertexByType[Process](g.dag, id)
}
func (g *graph) Link(id string) Link {
	return vertexByType[Link](g.dag, id)
}
func (g *graph) RouterAccess(id string) RouterAccess {
	return vertexByType[RouterAccess](g.dag, id)
}
func (g *graph) Connector(id string) Connector {
	return vertexByType[Connector](g.dag, id)
}
func (g *graph) Listener(id string) Listener {
	return vertexByType[Listener](g.dag, id)
}
func (g *graph) Address(id string) Address {
	return vertexByType[Address](g.dag, id)
}
func (g *graph) ConnectorTarget(id string) ConnectorTarget {
	return vertexByType[ConnectorTarget](g.dag, id)
}

func vertexByType[T Node](dag *dag.DAG, id string) T {
	var out T
	v, err := dag.GetVertex(id)
	if err != nil {
		return out
	}
	if vtx, ok := v.(T); ok {
		out = vtx
	}
	return out
}

func (g *graph) Unindex(in vanflow.Record) {
	g.dag.DeleteVertex(in.Identity())
}

func (g *graph) Reindex(in vanflow.Record) {
	id := in.Identity()
	vertex, _ := g.dag.GetVertex(id)
	switch record := in.(type) {
	case vanflow.SiteRecord:
		if vertex != nil {
			return
		}
		g.dag.AddVertex(Site{g.newBase(id)})
	case vanflow.RouterRecord:
		g.dag.AddVertex(Router{g.newBase(id)})
		var edges []Node
		if record.Parent != nil {
			edges = []Node{Site{g.newBase(*record.Parent)}}
		}
		g.ensureParents(id, edges)
	case vanflow.LinkRecord:
		g.dag.AddVertex(Link{g.newBase(id)})
		var edges []Node
		if record.Parent != nil {
			edges = append(edges, Router{g.newBase(*record.Parent)})
		}
		if record.Peer != nil {
			edges = append(edges, RouterAccess{g.newBase(*record.Peer)})
		}
		g.ensureParents(id, edges)
	case vanflow.RouterAccessRecord:
		g.dag.AddVertex(RouterAccess{g.newBase(id)})
		var edges []Node
		if record.Parent != nil {
			edges = []Node{Router{g.newBase(*record.Parent)}}
		}
		g.ensureParents(id, edges)
	case vanflow.ListenerRecord:
		g.dag.AddVertex(Listener{g.newBase(id)})
		var edges []Node
		if record.Parent != nil {
			edges = append(edges, Router{g.newBase(*record.Parent)})
		}
		if record.Address != nil && record.Protocol != nil {
			edges = append(edges, RoutingKey{g.newBase(RoutingKeyID(*record.Address, *record.Protocol))})
		}
		g.ensureParents(id, edges)
	case vanflow.ProcessRecord:
		processVtx := Process{g.newBase(id)}
		g.dag.AddVertex(processVtx)
		var edges []Node
		if record.Parent != nil {
			edges = append(edges, Site{g.newBase(*record.Parent)})
		}
		g.ensureParents(id, edges)

		if siteID, sourceHost := record.Parent, record.SourceHost; siteID != nil && sourceHost != nil {
			ctID := ConnectorTargetID(*siteID, *sourceHost)
			g.dag.AddVertex(ConnectorTarget{g.newBase(ctID)})
			g.ensureParents(ctID, []Node{processVtx})
		}
	case vanflow.ConnectorRecord:
		connectorVtx := Connector{g.newBase(id)}
		g.dag.AddVertex(connectorVtx)
		var edges []Node
		if record.Parent != nil {
			edges = append(edges, Router{g.newBase(*record.Parent)})
		}
		if record.Address != nil && record.Protocol != nil {
			edges = append(edges, RoutingKey{g.newBase(RoutingKeyID(*record.Address, *record.Protocol))})
		}
		if record.ProcessID != nil {
			edges = append(edges, Process{g.newBase(*record.ProcessID)})
		} else if record.DestHost != nil && record.Parent != nil {
			// TODO(ck) Fix race - if a connector came in before its router parent record we'd miss this relation
			snode := vertexByType[Router](g.dag, *record.Parent).Parent()
			if snode.IsKnown() {
				edges = append(edges, ConnectorTarget{g.newBase(ConnectorTargetID(snode.ID(), *record.DestHost))})
			}
		}
		g.ensureParents(id, edges)
	case AddressRecord:
		addressVtx := Address{g.newBase(id)}
		g.dag.AddVertex(addressVtx)
		routingKeyID := RoutingKeyID(record.Name, record.Protocol)
		g.dag.AddVertex(RoutingKey{g.newBase(routingKeyID)})
		g.ensureParents(routingKeyID, []Node{addressVtx})
	}
}

func (g *graph) ensureParents(id string, nodes []Node) {
	nm := make(map[string]Node, len(nodes))
	for _, n := range nodes {
		nm[n.ID()] = n
	}
	parents, _ := g.dag.GetParents(id)
	for pID := range parents {
		if _, ok := nm[pID]; ok {
			delete(nm, pID)
			continue
		}
		g.dag.DeleteEdge(pID, id)
	}

	for nID, node := range nm {
		g.dag.AddVertex(node)
		g.dag.AddEdge(nID, id)
	}
}

func (g *graph) newBase(id string) baseNode {
	return baseNode{
		dag:      g.dag,
		identity: id,
		stor:     g.stor,
	}
}

type Node interface {
	ID() string
	IsKnown() bool
	Get() (store.Entry, bool)
}

type baseNode struct {
	dag      *dag.DAG
	stor     store.Interface
	identity string
}

func (n baseNode) ID() string {
	return n.identity
}
func (n baseNode) IsKnown() bool {
	return n.identity != ""
}

func (b baseNode) Get() (entry store.Entry, found bool) {
	if b.identity == "" {
		return entry, false
	}
	return b.stor.Get(b.identity)
}

type Site struct {
	baseNode
}

func (n Site) GetRecord() (record vanflow.SiteRecord, found bool) {
	return getrecord[vanflow.SiteRecord](n)
}

func getrecord[R vanflow.Record, N Node](n N) (record R, found bool) {
	e, ok := n.Get()
	if !ok {
		return record, false
	}
	record, found = e.Record.(R)
	return record, found
}

func (n Site) Routers() []Router {
	return childrenByType[Router](n.dag, n.identity)
}

func (n Site) Links() []Link {
	var out []Link
	for _, router := range n.Routers() {
		out = append(out, childrenByType[Link](n.dag, router.ID())...)
	}
	return out
}

func (n Site) RouterAccess() []RouterAccess {
	var out []RouterAccess
	for _, router := range n.Routers() {
		out = append(out, childrenByType[RouterAccess](n.dag, router.ID())...)
	}
	return out
}

func parentOfType[T Node](g *dag.DAG, id string) T {
	var t T
	if g == nil {
		return t
	}
	parents, _ := g.GetParents(id)
	for _, parent := range parents {
		if pt, ok := parent.(T); ok {
			return pt
		}
	}
	return t
}

func childrenByType[T Node](g *dag.DAG, id string) []T {
	var results []T
	if g == nil {
		return results
	}
	children, _ := g.GetChildren(id)
	for _, child := range children {
		if childNode, ok := child.(T); ok {
			results = append(results, childNode)
		}
	}
	return results
}

type Router struct {
	baseNode
}

func (n Router) GetRecord() (record vanflow.RouterRecord, found bool) {
	return getrecord[vanflow.RouterRecord](n)
}

func (n Router) Parent() Site            { return parentOfType[Site](n.dag, n.identity) }
func (n Router) Listeners() []Listener   { return childrenByType[Listener](n.dag, n.identity) }
func (n Router) Connectors() []Connector { return childrenByType[Connector](n.dag, n.identity) }

type Link struct {
	baseNode
}

func (n Link) GetRecord() (record vanflow.LinkRecord, found bool) {
	return getrecord[vanflow.LinkRecord](n)
}

func (n Link) Parent() Router { return parentOfType[Router](n.dag, n.identity) }

func (n Link) Peer() RouterAccess {
	return parentOfType[RouterAccess](n.dag, n.identity)
}

type RouterAccess struct {
	baseNode
}

func (n RouterAccess) Parent() Router { return parentOfType[Router](n.dag, n.identity) }

func (n RouterAccess) Peers() []Link { return childrenByType[Link](n.dag, n.identity) }

type Listener struct {
	baseNode
}

func (n Listener) GetRecord() (record vanflow.ListenerRecord, found bool) {
	return getrecord[vanflow.ListenerRecord](n)
}

func (n Listener) Parent() Router { return parentOfType[Router](n.dag, n.identity) }
func (n Listener) Address() Node  { return parentOfType[RoutingKey](n.dag, n.identity).Parent() }

type Connector struct {
	baseNode
}

func (n Connector) Parent() Router { return parentOfType[Router](n.dag, n.identity) }

func (n Connector) Address() Address {
	return parentOfType[RoutingKey](n.dag, n.identity).Parent()
}

func (n Connector) Target() Process {
	target := parentOfType[Process](n.dag, n.identity)
	if target.IsKnown() {
		return target
	}
	return parentOfType[ConnectorTarget](n.dag, n.identity).Process()
}

type Process struct {
	baseNode
}

func (n Process) GetRecord() (record vanflow.ProcessRecord, found bool) {
	return getrecord[vanflow.ProcessRecord](n)
}
func (n Process) Parent() Site         { return parentOfType[Site](n.dag, n.identity) }
func (n Process) Addresses() []Address { return nil }
func (n Process) Connectors() []Connector {
	connectors := childrenByType[Connector](n.dag, n.identity)
	for _, target := range childrenByType[ConnectorTarget](n.dag, n.identity) {
		for _, cn := range target.Connectors() {
			if cn.IsKnown() {
				connectors = append(connectors, cn)
			}
		}
	}
	return connectors
}

func RoutingKeyID(address, protocol string) string {
	return fmt.Sprintf("%s:%s", protocol, address)
}

type RoutingKey struct {
	baseNode
}

func (n RoutingKey) Parent() Address { return parentOfType[Address](n.dag, n.identity) }
func (n RoutingKey) Connectors() []Connector {
	return childrenByType[Connector](n.dag, n.identity)
}
func (n RoutingKey) Listeners() []Listener { return childrenByType[Listener](n.dag, n.identity) }

func ConnectorTargetID(site, host string) string {
	return fmt.Sprintf("%s:%s", site, host)
}

type ConnectorTarget struct {
	baseNode
}

func (n ConnectorTarget) Connectors() []Connector {
	return childrenByType[Connector](n.dag, n.identity)
}
func (n ConnectorTarget) Process() Process { return parentOfType[Process](n.dag, n.identity) }

type Address struct {
	baseNode
}

func (n Address) GetRecord() (record AddressRecord, found bool) {
	return getrecord[AddressRecord](n)
}
func (n Address) RoutingKey() RoutingKey {
	rks := childrenByType[RoutingKey](n.dag, n.identity)
	if len(rks) == 1 {
		return rks[0]
	}
	return RoutingKey{}
}
