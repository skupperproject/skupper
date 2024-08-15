package collector

import (
	"fmt"

	"github.com/heimdalr/dag"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
)

type Graph struct {
	dag  *dag.DAG
	stor store.Interface
}

func NewGraph(stor store.Interface) *Graph {
	return &Graph{
		dag:  dag.NewDAG(),
		stor: stor,
	}
}

func (g *Graph) Reset() {
	g.dag = dag.NewDAG()
	for _, e := range g.stor.List() {
		g.Reindex(e.Record)
	}
}

func (g *Graph) Site(id string) Site {
	return vertexByType[Site](g.dag, id)
}
func (g *Graph) Process(id string) Process {
	return vertexByType[Process](g.dag, id)
}
func (g *Graph) Link(id string) Link {
	return vertexByType[Link](g.dag, id)
}
func (g *Graph) RouterAccess(id string) RouterAccess {
	return vertexByType[RouterAccess](g.dag, id)
}
func (g *Graph) Connector(id string) Connector {
	return vertexByType[Connector](g.dag, id)
}
func (g *Graph) Listener(id string) Listener {
	return vertexByType[Listener](g.dag, id)
}
func (g *Graph) Address(id string) Address {
	return vertexByType[Address](g.dag, id)
}
func (g *Graph) ConnectorTarget(id string) ConnectorTarget {
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

func (g *Graph) Unindex(in vanflow.Record) {
	g.dag.DeleteVertex(in.Identity())
}

func (g *Graph) Reindex(in vanflow.Record) {
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

func (g *Graph) ensureParents(id string, nodes []Node) {
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

func (g *Graph) newBase(id string) baseNode {
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

func (n Router) Parent() Site            { return parentOfType[Site](n.dag, n.identity) }
func (n Router) Listeners() []Listener   { return childrenByType[Listener](n.dag, n.identity) }
func (n Router) Connectors() []Connector { return childrenByType[Connector](n.dag, n.identity) }

type Link struct {
	baseNode
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

func (n Listener) Parent() Router { return parentOfType[Router](n.dag, n.identity) }
func (n Listener) Address() Node  { return nil }

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

func (n Address) Connectors() []Connector { return childrenByType[Connector](n.dag, n.identity) }
func (n Address) Listeners() []Listener   { return childrenByType[Listener](n.dag, n.identity) }
