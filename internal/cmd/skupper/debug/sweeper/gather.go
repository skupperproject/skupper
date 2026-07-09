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
	Identity  string `json:"identity"`
	Container string `json:"container"`
	Host      string `json:"host"`
	// LocalSocket is the router's own "host:port" for this connection's
	// socket. Unlike Host, it is unique per connection even for 'out'
	// connections, which all share the backend's address as their Host.
	LocalSocket    string `json:"localSocket"`
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
	// Sockets is keyed by peer "host:port", matching an 'in' connection's
	// Host (each client has a unique peer address).
	Sockets map[string]socketInfo
	// SocketsByLocal is keyed by the socket's own local "host:port",
	// matching an 'out' connection's LocalSocket ('out' peers all share the
	// backend's address, so only the local side is unique).
	SocketsByLocal map[string]socketInfo
}

// Execer runs a command (argv) and returns its stdout. Both skmanage and the
// socket query go through it, so they always observe the same host — and so
// the same network namespace, which is what makes their results joinable.
type Execer func(argv []string) ([]byte, error)

func LocalExec(argv []string) ([]byte, error) {
	return exec.Command(argv[0], argv[1:]...).Output()
}

// Gather queries the router for its TCP adaptor connections and cross
// references them with kernel socket state. Discards non-TCP-adaptor connections
func Gather(execFn Execer, skmanageBin, url string) (Snapshot, error) {
	raw, err := runSkmanage(execFn, skmanageBin, url, "QUERY", "--type="+ConnType)
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

	byPeer, byLocal := gatherSockets(execFn)
	return Snapshot{
		Now:            time.Now(),
		TCPConns:       tcpConns,
		Sockets:        byPeer,
		SocketsByLocal: byLocal,
	}, nil
}

func isTCPAdaptorConn(c connInfo) bool {
	return c.Container == tcpContainer && c.Host != egressDispatch
}

// gatherSockets reads kernel socket state via `ss -tin`. A host without ss (to be patched later)
// yields no sockets, which leaves every connection unmatched and untouched.
func gatherSockets(execFn Execer) (byPeer, byLocal map[string]socketInfo) {
	out, err := execFn([]string{"ss", "-tin"})
	if err != nil {
		return map[string]socketInfo{}, map[string]socketInfo{}
	}
	return socketsFromSS(out)
}

// socketsFromSS builds two {lastrcv, lastsnd} maps — one keyed by peer
// address, one by local address — by pairing each socket's header line in
// `ss -tin` output with its following detail line.
func socketsFromSS(out []byte) (byPeer, byLocal map[string]socketInfo) {
	byPeer = map[string]socketInfo{}
	byLocal = map[string]socketInfo{}

	var pendingLocal, pendingPeer string
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			pendingLocal, pendingPeer = "", ""
			fields := strings.Fields(line)
			if len(fields) < 5 || fields[0] == "State" {
				continue
			}
			pendingLocal, pendingPeer = fields[3], fields[4]
			continue
		}
		if pendingPeer == "" {
			continue
		}
		sock := socketInfo{
			LastRcvMs: extractMsField(line, "lastrcv:"),
			LastSndMs: extractMsField(line, "lastsnd:"),
		}
		byPeer[pendingPeer] = sock
		byLocal[pendingLocal] = sock
		pendingLocal, pendingPeer = "", ""
	}
	return byPeer, byLocal
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

func runSkmanage(execFn Execer, bin, url string, args ...string) ([]byte, error) {
	argv := append([]string{bin, "--bus", url}, args...)
	out, err := execFn(argv)
	if err != nil {
		return nil, fmt.Errorf("skmanage failed: %w", err)
	}
	return out, nil
}
