package sweeper

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/skupperproject/skupper/internal/ports"
)

// PortStat is the number of TCP adaptor connections on one router port, split
// by direction.
type PortStat struct {
	Port int
	In   int
	Out  int
}

func (p PortStat) Total() int { return p.In + p.Out }

// ListPorts summarizes the router's TCP adaptor connections by port,
// restricted to cfg.Ports when it is set.
func ListPorts(cfg Config) ([]PortStat, error) {
	if cfg.Exec == nil {
		cfg.Exec = LocalExec
	}
	conns, err := gatherConns(cfg.Exec, cfg.Skmanage, cfg.URL, cfg.SkmanageExtraArgs...)
	if err != nil {
		return nil, err
	}
	return summarizePorts(FilterByPorts(conns, cfg.Ports)), nil
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
				merged = &PortStat{Port: p.Port}
				byPort[p.Port] = merged
			}
			merged.In += p.In
			merged.Out += p.Out
		}
	}
	return sortedStats(byPort)
}

// PrintPortStats renders the port table. filter is the --port selection, used
// only to say which ports came up empty.
func PrintPortStats(w io.Writer, stats []PortStat, filter []int) {
	if len(stats) == 0 {
		if len(filter) > 0 {
			fmt.Fprintf(w, "No connections found on port %s.\n", FormatPorts(filter))
			return
		}
		fmt.Fprintln(w, "No TCP adaptor connections found.")
		return
	}
	tw := tabwriter.NewWriter(w, 8, 8, 1, '\t', tabwriter.TabIndent)
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", "PORT", "IN", "OUT", "TOTAL")
	for _, p := range stats {
		fmt.Fprintf(tw, "%d\t%d\t%d\t%d\n", p.Port, p.In, p.Out, p.Total())
	}
	_ = tw.Flush()
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
