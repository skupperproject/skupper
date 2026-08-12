package sweeper

import (
	"fmt"
	"time"
)

// Decision is a connection flagged for closure
type Decision struct {
	Conn   connInfo
	Reason string
}

// Evaluate applies all kill criteria (currently only based off of time) to all
// connections and determines which connections are to be killed by kill.go.
func Evaluate(snap Snapshot, idleThreshold time.Duration) []Decision {
	var orphans []Decision
	for _, c := range snap.TCPConns {
		if reason, isOrphan := orphanReason(c, snap, idleThreshold); isOrphan {
			orphans = append(orphans, Decision{Conn: c, Reason: reason})
		}
	}
	return orphans
}

// orphanReason decides whether connection, c, should be closed and why. When no socket can
// be matched the connection is skipped.
func orphanReason(c connInfo, snap Snapshot, idleThreshold time.Duration) (string, bool) {
	sock, ok := matchSocket(c, snap)
	if !ok {
		return "", false
	}
	idle := time.Duration(min(sock.LastRcvMs, sock.LastSndMs)) * time.Millisecond
	if idle >= idleThreshold {
		return fmt.Sprintf("idle for %s", idle.Round(time.Second)), true
	}
	return "", false
}

// matchSocket finds c's kernel socket. An 'in' connection's Host is its
// unique peer address; an 'out' connection's Host is the shared backend
// address, so it is matched by its unique LocalSocket instead.
func matchSocket(c connInfo, snap Snapshot) (socketInfo, bool) {
	switch c.Dir {
	case "in":
		sock, ok := snap.Sockets[c.Host]
		return sock, ok
	case "out":
		if c.LocalSocket == "" {
			return socketInfo{}, false
		}
		sock, ok := snap.SocketsByLocal[c.LocalSocket]
		return sock, ok
	}
	return socketInfo{}, false
}
