package sweeper

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	TcpListenerType        = "io.skupper.router.tcpListener"
	TcpConnectorType       = "io.skupper.router.tcpConnector"
	ListenerAddressType    = "io.skupper.router.listenerAddress"
	listenerNamePrefix     = "listener/"
	connectorNamePrefix    = "connector/"
	multiAddressPrefix     = "multiAddress/"
	multiKeyResourcePrefix = "multikeylistener/"
)

// tcpEndpointInfo is a bridge listener or connector as returned by skmanage
// QUERY --type=io.skupper.router.tcp{Listener,Connector}. Address is the
// routing key for a normal Listener/Connector; MultiKeyListener tcpListeners
// leave Address empty and store keys on listenerAddress entities instead.
type tcpEndpointInfo struct {
	Name    string `json:"name"`
	Port    string `json:"port"`
	Address string `json:"address"`
}

// listenerAddressInfo is a MultiKeyListener routing-key binding as returned by
// skmanage QUERY --type=io.skupper.router.listenerAddress. Listener is the
// parent tcpListener name (multiAddress/<cr-name>).
type listenerAddressInfo struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	Listener string `json:"listener"`
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

func gatherListenerAddresses(execFn Execer, skmanageBin, url string, extraArgs ...string) ([]listenerAddressInfo, error) {
	raw, err := runSkmanage(execFn, skmanageBin, url, extraArgs, "QUERY", "--type="+ListenerAddressType)
	if err != nil {
		return nil, fmt.Errorf("could not query %s: %w", ListenerAddressType, err)
	}
	var addresses []listenerAddressInfo
	if err := json.Unmarshal(raw, &addresses); err != nil {
		return nil, fmt.Errorf("failed to parse %s list: %w", ListenerAddressType, err)
	}
	return addresses, nil
}

// enrichSnapshot fills Port, State, RoutingKey, Resource, and Kind on each
// connection from socket maps, tcpListener/tcpConnector endpoints, and
// listenerAddress entities (MultiKeyListener routing keys).
func enrichSnapshot(snap *Snapshot) {
	addressesByListener := indexListenerAddresses(snap.ListenerAddresses)
	listenersByPort := indexListenersByPort(snap.Listeners, addressesByListener)
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

func indexListenerAddresses(addresses []listenerAddressInfo) map[string][]string {
	byListener := map[string][]string{}
	seen := map[string]map[string]bool{}
	for _, a := range addresses {
		if a.Listener == "" || a.Address == "" {
			continue
		}
		if seen[a.Listener] == nil {
			seen[a.Listener] = map[string]bool{}
		}
		if seen[a.Listener][a.Address] {
			continue
		}
		seen[a.Listener][a.Address] = true
		byListener[a.Listener] = append(byListener[a.Listener], a.Address)
	}
	return byListener
}

func indexListenersByPort(endpoints []tcpEndpointInfo, addressesByListener map[string][]string) map[int][]endpointRef {
	byPort := map[int][]endpointRef{}
	for _, e := range endpoints {
		port, err := strconv.Atoi(e.Port)
		if err != nil || port <= 0 {
			continue
		}
		kind, resource := listenerKindAndResource(e.Name)
		routingKey := e.Address
		if routingKey == "" {
			routingKey = strings.Join(addressesByListener[e.Name], ",")
		}
		byPort[port] = append(byPort[port], endpointRef{
			Kind:       kind,
			Resource:   resource,
			RoutingKey: routingKey,
		})
	}
	return byPort
}

func listenerKindAndResource(name string) (kind, resource string) {
	if strings.HasPrefix(name, multiAddressPrefix) {
		return "multikeylistener", multiKeyResourcePrefix + strings.TrimPrefix(name, multiAddressPrefix)
	}
	return "listener", displayResource(name, listenerNamePrefix)
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
		for _, part := range strings.Split(r.RoutingKey, ",") {
			part = strings.TrimSpace(part)
			if part == "" || seen[part] {
				continue
			}
			seen[part] = true
			keys = append(keys, part)
		}
	}
	return strings.Join(keys, ",")
}

// enrichPortStats attaches listener/connector correlation to port summary
// rows. When both an inbound listener and outbound connector share a port,
// both kinds' routing keys are joined.
func enrichPortStats(stats []PortStat, listeners, connectors []tcpEndpointInfo, addresses []listenerAddressInfo) {
	addressesByListener := indexListenerAddresses(addresses)
	listenersByPort := indexListenersByPort(listeners, addressesByListener)
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
			p.Kind = refs[0].Kind + ",connector"
		}
		p.Resource = formatResource(refs)
		p.RoutingKey = formatRoutingKeys(refs)
	}
}
