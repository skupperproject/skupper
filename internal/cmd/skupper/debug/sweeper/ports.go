package sweeper

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/skupperproject/skupper/internal/ports"
)

// PortStat is the number of TCP adaptor connections on one router port, split
// by direction, plus optional listener/connector correlation.
type PortStat struct {
	Port       int
	In         int
	Out        int
	RoutingKey string
	Resource   string
	Kind       string
}

func (p PortStat) Total() int { return p.In + p.Out }

// ListPorts summarizes the router's TCP adaptor connections by port,
// restricted to cfg.Ports / RoutingKeys when set. State filtering is not
// applied here because list-ports does not query kernel sockets.
func ListPorts(cfg Config) ([]PortStat, error) {
	if cfg.Exec == nil {
		cfg.Exec = LocalExec
	}
	if err := ValidatePorts(cfg.Ports); err != nil {
		return nil, err
	}
	if err := ValidateOutput(cfg.Output); err != nil {
		return nil, err
	}
	conns, err := gatherConns(cfg.Exec, cfg.Skmanage, cfg.URL, cfg.SkmanageExtraArgs...)
	if err != nil {
		return nil, err
	}
	listeners, err := gatherTcpEndpoints(cfg.Exec, cfg.Skmanage, cfg.URL, TcpListenerType, cfg.SkmanageExtraArgs...)
	if err != nil {
		if len(cfg.RoutingKeys) > 0 {
			return nil, err
		}
		listeners = nil
	}
	connectors, err := gatherTcpEndpoints(cfg.Exec, cfg.Skmanage, cfg.URL, TcpConnectorType, cfg.SkmanageExtraArgs...)
	if err != nil {
		if len(cfg.RoutingKeys) > 0 {
			return nil, err
		}
		connectors = nil
	}

	for i := range conns {
		if port, ok := portOf(conns[i]); ok {
			conns[i].Port = port
		}
	}
	tmp := Snapshot{TCPConns: conns, Listeners: listeners, Connectors: connectors}
	enrichSnapshot(&tmp)
	conns = tmp.TCPConns
	conns = FilterByPorts(conns, cfg.Ports)
	conns = FilterByRoutingKeys(conns, cfg.RoutingKeys)

	stats := summarizePorts(conns)
	enrichPortStats(stats, listeners, connectors)
	return stats, nil
}

// FilterByPorts keeps the connections whose router-side port is one of
// portList. An empty portList leaves conns untouched.
func FilterByPorts(conns []connInfo, portList []int) []connInfo {
	if len(portList) == 0 {
		return conns
	}
	wanted := make(map[int]bool, len(portList))
	for _, p := range portList {
		wanted[p] = true
	}
	var matched []connInfo
	for _, c := range conns {
		if port, ok := portOf(c); ok && wanted[port] {
			matched = append(matched, c)
		}
	}
	return matched
}

// ValidatePorts rejects ports no connection could ever carry, so that a typo
// is an error rather than an empty result indistinguishable from a quiet port.
func ValidatePorts(portList []int) error {
	var portErrors []error
	for _, p := range portList {
		if p < 1 || p > ports.MAX_PORT {
			portErrors = append(portErrors, fmt.Errorf("port is not valid: %d is not between 1 and %d", p, ports.MAX_PORT))
		}
	}
	return errors.Join(portErrors...)
}

func FormatPorts(portList []int) string {
	as := make([]string, 0, len(portList))
	for _, p := range portList {
		as = append(as, strconv.Itoa(p))
	}
	return strings.Join(as, ", ")
}

// MergePortStats combines per-pod summaries into one, for the aggregate table
// printed when a site runs more than one router pod.
func MergePortStats(stats ...[]PortStat) []PortStat {
	byPort := map[int]*PortStat{}
	for _, perPod := range stats {
		for _, p := range perPod {
			merged := byPort[p.Port]
			if merged == nil {
				cp := p
				byPort[p.Port] = &cp
				continue
			}
			merged.In += p.In
			merged.Out += p.Out
			merged.RoutingKey = mergeField(merged.RoutingKey, p.RoutingKey)
			merged.Resource = mergeResourceField(merged.Resource, p.Resource)
			merged.Kind = mergeField(merged.Kind, p.Kind)
		}
	}
	return sortedStats(byPort)
}

func mergeField(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" || a == b {
		return a
	}
	parts := map[string]bool{}
	for _, p := range strings.Split(a+","+b, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			parts[p] = true
		}
	}
	var out []string
	for p := range parts {
		out = append(out, p)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func mergeResourceField(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" || a == b {
		return a
	}
	return a + " (+more)"
}

// PrintPortStats renders the port table. filter is the --port selection, used
// only to say which ports came up empty.
func PrintPortStats(w io.Writer, stats []PortStat, filter []int) error {
	return PrintPortStatsOutput(w, stats, filter, OutputText)
}

// PrintPortStatsOutput is PrintPortStats with an explicit output format.
func PrintPortStatsOutput(w io.Writer, stats []PortStat, filter []int, output string) error {
	if NormalizeOutput(output) == OutputJSON {
		reports := make([]PortReport, 0, len(stats))
		for _, p := range stats {
			reports = append(reports, PortReport{
				Port:       p.Port,
				In:         p.In,
				Out:        p.Out,
				Total:      p.Total(),
				RoutingKey: p.RoutingKey,
				Resource:   p.Resource,
				Kind:       p.Kind,
			})
		}
		return WriteJSON(w, reports)
	}
	if len(stats) == 0 {
		if len(filter) > 0 {
			_, err := fmt.Fprintf(w, "No connections found on port %s.\n", FormatPorts(filter))
			return err
		}
		_, err := fmt.Fprintln(w, "No TCP adaptor connections found.")
		return err
	}
	tw := tabwriter.NewWriter(w, 8, 8, 1, '\t', tabwriter.TabIndent)
	if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", "PORT", "IN", "OUT", "TOTAL", "ROUTING-KEY", "RESOURCE"); err != nil {
		return err
	}
	for _, p := range stats {
		if _, err := fmt.Fprintf(tw, "%d\t%d\t%d\t%d\t%s\t%s\n",
			p.Port, p.In, p.Out, p.Total(), emptyDash(p.RoutingKey), emptyDash(p.Resource)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// PrintPortStatsToStdout is used by platform adapters with Config.Output.
func PrintPortStatsToStdout(stats []PortStat, filter []int, output string) error {
	return PrintPortStatsOutput(os.Stdout, stats, filter, output)
}

func summarizePorts(conns []connInfo) []PortStat {
	byPort := map[int]*PortStat{}
	for _, c := range conns {
		port, ok := portOf(c)
		if !ok {
			continue
		}
		stat := byPort[port]
		if stat == nil {
			stat = &PortStat{Port: port}
			byPort[port] = stat
		}
		if c.Dir == "in" {
			stat.In++
		} else {
			stat.Out++
		}
	}
	return sortedStats(byPort)
}

// portOf returns the router-side port of c. The two directions keep it in
// different fields: an 'in' connection's peer port is the client's ephemeral
// one, so the listener's port is on LocalSocket; an 'out' connection's local
// port is the ephemeral one, so the backend's port is on Host.
func portOf(c connInfo) (int, bool) {
	if c.Port > 0 {
		return c.Port, true
	}
	switch c.Dir {
	case "in":
		return portFromAddr(c.LocalSocket)
	case "out":
		return portFromAddr(c.Host)
	}
	return 0, false
}

// portFromAddr pulls the port off a "host:port" address.
func portFromAddr(addr string) (int, bool) {
	i := strings.LastIndex(addr, ":")
	if i == -1 {
		return 0, false
	}
	port, err := strconv.Atoi(addr[i+1:])
	if err != nil || port <= 0 {
		return 0, false
	}
	return port, true
}

// sortedStats orders by busiest port first, breaking ties by port number so
// the table is stable across runs.
func sortedStats(byPort map[int]*PortStat) []PortStat {
	stats := make([]PortStat, 0, len(byPort))
	for _, p := range byPort {
		stats = append(stats, *p)
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Total() != stats[j].Total() {
			return stats[i].Total() > stats[j].Total()
		}
		return stats[i].Port < stats[j].Port
	})
	return stats
}
