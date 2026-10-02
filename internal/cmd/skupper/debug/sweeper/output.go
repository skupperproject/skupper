package sweeper

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// ConnReport is one connection as emitted in text or JSON listings.
type ConnReport struct {
	Identity   string `json:"identity"`
	Host       string `json:"host"`
	Dir        string `json:"dir"`
	Port       int    `json:"port,omitempty"`
	State      string `json:"state,omitempty"`
	Uptime     string `json:"uptime,omitempty"`
	Idle       string `json:"idle,omitempty"`
	RoutingKey string `json:"routingKey,omitempty"`
	Resource   string `json:"resource,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Action     string `json:"action,omitempty"`
}

// PortReport is one port summary row for JSON --list-ports output.
type PortReport struct {
	Port       int    `json:"port"`
	In         int    `json:"in"`
	Out        int    `json:"out"`
	Total      int    `json:"total"`
	RoutingKey string `json:"routingKey,omitempty"`
	Resource   string `json:"resource,omitempty"`
	Kind       string `json:"kind,omitempty"`
}

func decisionReport(d Decision) ConnReport {
	return ConnReport{
		Identity:   d.Conn.Identity,
		Host:       d.Conn.Host,
		Dir:        d.Conn.Dir,
		Port:       d.Conn.Port,
		State:      d.Conn.State,
		Uptime:     fmtSeconds(d.Conn.UptimeSeconds),
		RoutingKey: d.Conn.RoutingKey,
		Resource:   d.Conn.Resource,
		Kind:       d.Conn.Kind,
		Reason:     d.Reason,
	}
}

func decisionsToReports(decisions []Decision) []ConnReport {
	reports := make([]ConnReport, 0, len(decisions))
	for _, d := range decisions {
		reports = append(reports, decisionReport(d))
	}
	return reports
}

func printDecisions(w io.Writer, decisions []Decision, output string) error {
	if NormalizeOutput(output) == OutputJSON {
		return WriteJSON(w, decisionsToReports(decisions))
	}
	for _, d := range decisions {
		if _, err := fmt.Fprintf(w, "  id=%-6s  host=%-25s  dir=%s  port=%-5d  state=%-10s  uptime=%-10s  routing-key=%-16s  resource=%-24s  reason=%s\n",
			d.Conn.Identity, d.Conn.Host, d.Conn.Dir, d.Conn.Port, emptyDash(d.Conn.State),
			fmtSeconds(d.Conn.UptimeSeconds), emptyDash(d.Conn.RoutingKey), emptyDash(d.Conn.Resource), d.Reason); err != nil {
			return err
		}
	}
	return nil
}

func printKillResult(w io.Writer, d Decision, action, errMsg string, output string, accumulate *[]ConnReport) {
	r := decisionReport(d)
	r.Action = action
	if errMsg != "" {
		r.Reason = d.Reason + "; " + errMsg
	}
	if NormalizeOutput(output) == OutputJSON {
		*accumulate = append(*accumulate, r)
		return
	}
	switch action {
	case "killed":
		logf("  id=%s  host=%s  dir=%s  port=%d  routing-key=%s  resource=%s  uptime=%s  reason=%s  → killed",
			d.Conn.Identity, d.Conn.Host, d.Conn.Dir, d.Conn.Port, emptyDash(d.Conn.RoutingKey), emptyDash(d.Conn.Resource),
			fmtSeconds(d.Conn.UptimeSeconds), d.Reason)
	case "already closed":
		logf("  id=%s  host=%s  dir=%s  reason=%s  → already closed",
			d.Conn.Identity, d.Conn.Host, d.Conn.Dir, d.Reason)
	default:
		logf("  id=%s  host=%s  dir=%s  reason=%s  → failed: %s",
			d.Conn.Identity, d.Conn.Host, d.Conn.Dir, d.Reason, errMsg)
	}
	_ = w
}

// WriteJSON encodes v as indented JSON to w. Used by platform adapters so
// multi-pod runs can emit a single document on stdout.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeJSON(w io.Writer, v any) error { return WriteJSON(w, v) }

func logf(format string, args ...any) {
	ts := time.Now().Format("15:04:05")
	fmt.Printf("["+ts+"] "+format+"\n", args...)
}

func fmtSeconds(s *int) string {
	if s == nil {
		return "never"
	}
	return fmtDuration(time.Duration(*s) * time.Second)
}

func fmtDuration(d time.Duration) string {
	sec := int(d.Seconds())
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
