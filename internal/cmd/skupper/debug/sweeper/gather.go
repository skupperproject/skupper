package sweeper

import (
	"encoding/json"
	"errors"
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
	LocalSocket   string `json:"localSocket"`
	Dir           string `json:"dir"`
	UptimeSeconds *int   `json:"uptimeSeconds"`
}

// socketInfo is the kernel's view of a TCP socket, as reported by `ss -tin`.
// LastRcvMs/LastSndMs come from the kernel's TCP_INFO.
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

// Execer runs a command (argv) and returns its stdout. LocalExec runs on
// this host; the kube variant execs inside the router pod instead, so both
// skmanage and the socket query see the router's network namespace.
type Execer func(argv []string) ([]byte, error)

func LocalExec(argv []string) ([]byte, error) {
	return exec.Command(argv[0], argv[1:]...).Output()
}

// Gather queries the router for its TCP adaptor connections and cross
// references them with kernel socket state. Discards non-TCP-adaptor connections.
// extraArgs are appended to the skmanage invocation (e.g. --ssl-certificate
// options when the management endpoint is amqps).
func Gather(execFn Execer, skmanageBin, url string, extraArgs ...string) (Snapshot, error) {
	tcpConns, err := gatherConns(execFn, skmanageBin, url, extraArgs...)
	if err != nil {
		return Snapshot{}, err
	}

	byPeer, byLocal, err := gatherSockets(execFn)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		Now:            time.Now(),
		TCPConns:       tcpConns,
		Sockets:        byPeer,
		SocketsByLocal: byLocal,
	}, nil
}

// gatherConns queries the router for its connections, keeping only the TCP
// adaptor ones.
func gatherConns(execFn Execer, skmanageBin, url string, extraArgs ...string) ([]connInfo, error) {
	raw, err := runSkmanage(execFn, skmanageBin, url, extraArgs, "QUERY", "--type="+ConnType)
	if err != nil {
		return nil, fmt.Errorf("could not query router at %s: %w", url, err)
	}

	var allConns []connInfo
	if err := json.Unmarshal(raw, &allConns); err != nil {
		return nil, fmt.Errorf("failed to parse connection list: %w", err)
	}

	var tcpConns []connInfo
	for _, c := range allConns {
		if isTCPAdaptorConn(c) {
			tcpConns = append(tcpConns, c)
		}
	}
	return tcpConns, nil
}

func isTCPAdaptorConn(c connInfo) bool {
	return c.Container == tcpContainer && c.Host != egressDispatch
}

// gatherSockets reads kernel socket state, preferring `ss -tin` and falling
// back to the python netlink script when ss isn't available (e.g. inside the
// router container, which ships python3 but not iproute) or when ss runs but
// cannot report idle timers.
func gatherSockets(execFn Execer) (map[string]socketInfo, map[string]socketInfo, error) {
	out, ssErr := execFn([]string{"ss", "-tin"})
	if ssErr == nil {
		byPeer, byLocal := socketsFromSS(out)
		// `ss` exits 0 even when it cannot open a netlink socket: it warns on
		// stderr and falls back to /proc/net/tcp, which lists the sockets but
		// carries no TCP_INFO, so no row has the indented detail line the
		// timers come from. Sockets listed but none parsed means the timers
		// are missing, not that the host is idle, so try the fallback rather
		// than report every connection as unmatchable.
		if len(byPeer) > 0 || !ssListedSockets(out) {
			return byPeer, byLocal, nil
		}
		ssErr = errors.New("ss listed sockets but reported no TCP_INFO")
	}

	out, pyErr := execFn([]string{"python3", "-c", inetDiagScript})
	if pyErr != nil {
		return nil, nil, fmt.Errorf("could not read socket state, so no connection could be matched to its socket: ss unusable (%v) and python3 fallback failed (%v)", ssErr, pyErr)
	}
	byPeer, byLocal := socketsFromDiagOutput(out)
	return byPeer, byLocal, nil
}

// ssListedSockets reports whether out holds at least one socket row, so that
// output carrying no sockets at all can be told apart from output whose rows
// are all missing their detail lines.
func ssListedSockets(out []byte) bool {
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		if _, _, ok := ssSocketRow(line); ok {
			return true
		}
	}
	return false
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
			local, peer, ok := ssSocketRow(line)
			if !ok {
				continue
			}
			pendingLocal, pendingPeer = local, peer
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

// ssSocketRow pulls the local and peer addresses out of an `ss` socket row,
// reporting ok=false for the column header and any line too short to be one.
func ssSocketRow(line string) (local, peer string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] == "State" {
		return "", "", false
	}
	return fields[3], fields[4], true
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

func runSkmanage(execFn Execer, bin, url string, extraArgs []string, args ...string) ([]byte, error) {
	argv := append([]string{bin, "--bus", url}, args...)
	argv = append(argv, extraArgs...)
	out, err := execFn(argv)
	if err != nil {
		return nil, fmt.Errorf("skmanage failed: %w", err)
	}
	return out, nil
}
