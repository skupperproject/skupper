package sweeper

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	ConnType       = "io.skupper.router.connection"
	tcpContainer   = "TcpAdaptor"
	egressDispatch = "egress-dispatch"
)

// connInfo is the router's view of a single connection, as returned by
// `skmanage QUERY --type=io.skupper.router.connection`.
type connInfo struct {
	Identity       string `json:"identity"`
	Container      string `json:"container"`
	Host           string `json:"host"`
	Dir            string `json:"dir"`
	UptimeSeconds  *int   `json:"uptimeSeconds"`
	LastDlvSeconds *int   `json:"lastDlvSeconds"`
}

// socketInfo is the kernel's view of a TCP socket, as reported by `ss -tin`.
// LastRcvMs/LastSndMs come straight from TCP_INFO and should show actual activity vs lastDlvSeconds
type socketInfo struct {
	LastRcvMs int
	LastSndMs int
}

// Snapshot bundles the router's connection list and the kernel's socket
// state at one point in time.
type Snapshot struct {
	Now      time.Time
	TCPConns []connInfo
	// Sockets is keyed by peer "host:port" so it can be matched against an
	// inbound connInfo.Host. Only sockets for 'in' (client-facing) connections
	// can be matched reliably.
	Sockets map[string]socketInfo
}

// Gather queries the router for its TCP adaptor connections and cross
// references them with kernel socket state. It performs no filtering beyond
// discarding non-TCP-adaptor connections, and makes no decisions about which
// connections are healthy.
func Gather(skmanageBin, url string) (Snapshot, error) {
	raw, err := runSkmanage(skmanageBin, url, "QUERY", "--type="+ConnType)
	if err != nil {
		return Snapshot{}, fmt.Errorf("could not query router at %s: %w", url, err)
	}

	var allConns []connInfo
	if err := json.Unmarshal(raw, &allConns); err != nil {
		return Snapshot{}, fmt.Errorf("failed to parse connection list: %w", err)
	}

	var tcpConns []connInfo
	for _, c := range allConns {
		if isTCPAdaptorConn(c) {
			tcpConns = append(tcpConns, c)
		}
	}

	return Snapshot{
		Now:      time.Now(),
		TCPConns: tcpConns,
		Sockets:  gatherSockets(),
	}, nil
}

func isTCPAdaptorConn(c connInfo) bool {
	return c.Container == tcpContainer && c.Host != egressDispatch
}

//runs ss -tin and builds a peer-address → {lastrcv, lastsnd} map by pairing each socket's 
//header line with its following detail line.
func gatherSockets() map[string]socketInfo {
	sockets := map[string]socketInfo{}
	out, err := exec.Command("ss", "-tin").Output()
	if err != nil {
		return sockets
	}

	var pendingPeer string
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			pendingPeer = ""
			fields := strings.Fields(line)
			if len(fields) < 5 || fields[0] == "State" {
				continue
			}
			pendingPeer = fields[4]
			continue
		}
		if pendingPeer == "" {
			continue
		}
		sockets[pendingPeer] = socketInfo{
			LastRcvMs: extractMsField(line, "lastrcv:"),
			LastSndMs: extractMsField(line, "lastsnd:"),
		}
		pendingPeer = ""
	}
	return sockets
}

// extractMsField returns the integer following "key:" in line (e.g. key
// "lastrcv:" on "... lastrcv:592 lastack:9119 ..." returns 592). Returns 0
// if key isn't found in line.
func extractMsField(line, key string) int {
	idx := strings.Index(line, key)
	if idx == -1 {
		return 0
	}
	rest := line[idx+len(key):]
	end := strings.IndexAny(rest, " \t")
	if end != -1 {
		rest = rest[:end]
	}
	val, err := strconv.Atoi(rest)
	if err != nil {
		return 0
	}
	return val
}

func runSkmanage(bin, url string, args ...string) ([]byte, error) {
	cmdArgs := append([]string{"--bus", url}, args...)
	out, err := exec.Command(bin, cmdArgs...).Output()
	if err != nil {
		return nil, fmt.Errorf("skmanage failed: %w", err)
	}
	return out, nil
}
