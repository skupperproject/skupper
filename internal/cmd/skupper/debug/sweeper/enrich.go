package sweeper

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	TcpListenerType  = "io.skupper.router.tcpListener"
	TcpConnectorType = "io.skupper.router.tcpConnector"

	listenerNamePrefix  = "listener/"
	connectorNamePrefix = "connector/"
)

// tcpEndpointInfo is a bridge listener or connector as returned by skmanage
// QUERY --type=io.skupper.router.tcp{Listener,Connector}. Address is the
// routing key; Port is the router-side (or backend) port used for joins.
type tcpEndpointInfo struct {
	Name    string `json:"name"`
	Port    string `json:"port"`
	Address string `json:"address"`
}

type endpointRef struct {
	Kind       string
	Resource   string
	RoutingKey string
}

func gatherTcpEndpoints(execFn Execer, skmanageBin, url, typeName string, extraArgs ...string) ([]tcpEndpointInfo, error) {
	raw, err := runSkmanage(execFn, skmanageBin, url, extraArgs, "QUERY", "--type="+typeName)
	if err != nil {
		return nil, fmt.Errorf("could not query %s: %w", typeName, err)
	}
	var endpoints []tcpEndpointInfo
	if err := json.Unmarshal(raw, &endpoints); err != nil {
		return nil, fmt.Errorf("failed to parse %s list: %w", typeName, err)
	}
	return endpoints, nil
}

// enrichSnapshot fills Port, State, RoutingKey, Resource, and Kind on each
// connection from socket maps and tcpListener/tcpConnector endpoints.
func enrichSnapshot(snap *Snapshot) {
	listenersByPort := indexEndpointsByPort(snap.Listeners, "listener", listenerNamePrefix)
	connectorsByPort := indexEndpointsByPort(snap.Connectors, "connector", connectorNamePrefix)

	for i := range snap.TCPConns {
		c := &snap.TCPConns[i]
		if port, ok := portOf(*c); ok {
			c.Port = port
		}
		if sock, ok := matchSocket(*c, *snap); ok {
			c.State = sock.State
		}
		var refs []endpointRef
		switch c.Dir {
		case "in":
			refs = listenersByPort[c.Port]
		case "out":
			refs = connectorsByPort[c.Port]
		}
		if len(refs) == 0 {
			continue
		}
		c.Kind = refs[0].Kind
		c.Resource = formatResource(refs)
		c.RoutingKey = formatRoutingKeys(refs)
	}
}

func indexEndpointsByPort(endpoints []tcpEndpointInfo, kind, prefix string) map[int][]endpointRef {
	byPort := map[int][]endpointRef{}
	for _, e := range endpoints {
		port, err := strconv.Atoi(e.Port)
		if err != nil || port <= 0 {
			continue
		}
		byPort[port] = append(byPort[port], endpointRef{
			Kind:       kind,
			Resource:   displayResource(e.Name, prefix),
			RoutingKey: e.Address,
		})
	}
	return byPort
}

func displayResource(name, prefix string) string {
	if strings.HasPrefix(name, prefix) {
		return name
	}
	if name == "" {
		return ""
	}
	return name
}

func formatResource(refs []endpointRef) string {
	if len(refs) == 0 {
		return ""
	}
	if len(refs) == 1 {
		return refs[0].Resource
	}
	return fmt.Sprintf("%s (+%d)", refs[0].Resource, len(refs)-1)
}

func formatRoutingKeys(refs []endpointRef) string {
	seen := map[string]bool{}
	var keys []string
	for _, r := range refs {
		if r.RoutingKey == "" || seen[r.RoutingKey] {
			continue
		}
		seen[r.RoutingKey] = true
		keys = append(keys, r.RoutingKey)
	}
	return strings.Join(keys, ",")
}

// enrichPortStats attaches listener/connector correlation to port summary
// rows. When both an inbound listener and outbound connector share a port,
// both kinds' routing keys are joined.
func enrichPortStats(stats []PortStat, listeners, connectors []tcpEndpointInfo) {
	listenersByPort := indexEndpointsByPort(listeners, "listener", listenerNamePrefix)
	connectorsByPort := indexEndpointsByPort(connectors, "connector", connectorNamePrefix)

	for i := range stats {
		p := &stats[i]
		var refs []endpointRef
		refs = append(refs, listenersByPort[p.Port]...)
		refs = append(refs, connectorsByPort[p.Port]...)
		if len(refs) == 0 {
			continue
		}
		p.Kind = refs[0].Kind
		if len(listenersByPort[p.Port]) > 0 && len(connectorsByPort[p.Port]) > 0 {
			p.Kind = "listener,connector"
		}
		p.Resource = formatResource(refs)
		p.RoutingKey = formatRoutingKeys(refs)
	}
}
