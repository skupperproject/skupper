package sweeper

import (
	"errors"
	"fmt"
	"strings"
)

// KnownTCPStates are the ss -tin state names we accept for --state.
// CLOSE is accepted as an alias for UNCONN (TCP_CLOSE).
var KnownTCPStates = map[string]string{
	"ESTAB":       "ESTAB",
	"SYN-SENT":    "SYN-SENT",
	"SYN-RECV":    "SYN-RECV",
	"FIN-WAIT-1":  "FIN-WAIT-1",
	"FIN-WAIT-2":  "FIN-WAIT-2",
	"TIME-WAIT":   "TIME-WAIT",
	"UNCONN":      "UNCONN",
	"CLOSE":       "UNCONN",
	"CLOSE-WAIT":  "CLOSE-WAIT",
	"LAST-ACK":    "LAST-ACK",
	"LISTEN":      "LISTEN",
	"CLOSING":     "CLOSING",
	"ESTABLISHED": "ESTAB",
}

// KnownResourceKinds are the correlated Skupper resource kinds accepted by --kind.
var KnownResourceKinds = map[string]string{
	"listener":           "listener",
	"connector":          "connector",
	"multikeylistener":   "multikeylistener",
	"multi-key-listener": "multikeylistener",
	"multikey":           "multikeylistener",
}

const (
	OutputText = "text"
	OutputJSON = "json"
)

// NormalizeStates canonicalizes user-supplied TCP state names to ss form.
func NormalizeStates(states []string) ([]string, error) {
	if len(states) == 0 {
		return nil, nil
	}
	var out []string
	var errs []error
	seen := map[string]bool{}
	for _, raw := range expandCommaList(states) {
		canon, ok := KnownTCPStates[strings.ToUpper(raw)]
		if !ok {
			errs = append(errs, fmt.Errorf("unknown TCP state %q (expected ss names like ESTAB, FIN-WAIT-2)", raw))
			continue
		}
		if seen[canon] {
			continue
		}
		seen[canon] = true
		out = append(out, canon)
	}
	return out, errors.Join(errs...)
}

// ValidateOutput accepts text (default/empty) or json.
func ValidateOutput(output string) error {
	switch strings.ToLower(strings.TrimSpace(output)) {
	case "", OutputText, OutputJSON:
		return nil
	default:
		return fmt.Errorf("output must be %q or %q", OutputText, OutputJSON)
	}
}

func NormalizeOutput(output string) string {
	if strings.EqualFold(strings.TrimSpace(output), OutputJSON) {
		return OutputJSON
	}
	return OutputText
}

func expandCommaList(values []string) []string {
	var out []string
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// NormalizeKinds canonicalizes user-supplied resource kind names.
// A nil/empty slice means no kind filter. A nonempty slice that expands to
// no values (e.g. --kind ",") is rejected so Run cannot treat it as unrestricted.
func NormalizeKinds(kinds []string) ([]string, error) {
	if len(kinds) == 0 {
		return nil, nil
	}
	expanded := expandCommaList(kinds)
	if len(expanded) == 0 {
		return nil, fmt.Errorf("kind filter is empty (expected listener, connector, or multikeylistener)")
	}
	var out []string
	var errs []error
	seen := map[string]bool{}
	for _, raw := range expanded {
		canon, ok := KnownResourceKinds[strings.ToLower(raw)]
		if !ok {
			errs = append(errs, fmt.Errorf("unknown resource kind %q (expected listener, connector, or multikeylistener)", raw))
			continue
		}
		if seen[canon] {
			continue
		}
		seen[canon] = true
		out = append(out, canon)
	}
	return out, errors.Join(errs...)
}

// FilterByRoutingKeys keeps connections whose enriched RoutingKey is one of
// the wanted keys. An empty list leaves conns untouched. Connections with no
// routing key are dropped when a filter is active.
func FilterByRoutingKeys(conns []connInfo, keys []string) []connInfo {
	keys = expandCommaList(keys)
	if len(keys) == 0 {
		return conns
	}
	wanted := make(map[string]bool, len(keys))
	for _, k := range keys {
		wanted[k] = true
	}
	var matched []connInfo
	for _, c := range conns {
		if csvFieldMatches(c.RoutingKey, wanted) {
			matched = append(matched, c)
		}
	}
	return matched
}

// FilterByKinds keeps connections whose enriched Kind is one of the wanted
// kinds. An empty list leaves conns untouched. Connections with no kind are
// dropped when a filter is active. Comma-joined kinds (e.g. from --list-ports
// when listener and connector share a port) match if any part is wanted.
func FilterByKinds(conns []connInfo, kinds []string) []connInfo {
	kinds = expandCommaList(kinds)
	if len(kinds) == 0 {
		return conns
	}
	wanted := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		wanted[k] = true
	}
	var matched []connInfo
	for _, c := range conns {
		if csvFieldMatches(c.Kind, wanted) {
			matched = append(matched, c)
		}
	}
	return matched
}

// FilterPortStatsByKinds keeps port stats whose Kind matches any wanted kind.
func FilterPortStatsByKinds(stats []PortStat, kinds []string) []PortStat {
	kinds = expandCommaList(kinds)
	if len(kinds) == 0 {
		return stats
	}
	wanted := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		wanted[k] = true
	}
	var matched []PortStat
	for _, p := range stats {
		if csvFieldMatches(p.Kind, wanted) {
			matched = append(matched, p)
		}
	}
	return matched
}

func csvFieldMatches(field string, wanted map[string]bool) bool {
	if field == "" {
		return false
	}
	for _, part := range strings.Split(field, ",") {
		if wanted[strings.TrimSpace(part)] {
			return true
		}
	}
	return false
}

// FilterByStates keeps connections whose matched kernel socket is in one of
// the given ss states. An empty list leaves conns untouched. Connections with
// no matched socket are dropped when a filter is active.
func FilterByStates(snap Snapshot, states []string) []connInfo {
	if len(states) == 0 {
		return snap.TCPConns
	}
	wanted := make(map[string]bool, len(states))
	for _, s := range states {
		wanted[s] = true
	}
	var matched []connInfo
	for _, c := range snap.TCPConns {
		sock, ok := matchSocket(c, snap)
		if !ok {
			continue
		}
		if !wanted[sock.State] {
			continue
		}
		c.State = sock.State
		matched = append(matched, c)
	}
	return matched
}
