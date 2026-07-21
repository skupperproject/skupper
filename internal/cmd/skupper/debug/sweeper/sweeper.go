package sweeper

import (
	"fmt"
	"time"
)

const (
	DefaultURL           = "amqp://127.0.0.1:5672"
	DefaultSkmanage      = "skmanage"
	DefaultIdleThreshold = 4 * 3600 // 4 hours
)

type Config struct {
	URL               string
	Skmanage          string
	IdleThresholdSecs int
	Execute           bool
	// Exec runs skmanage and the socket query.
	Exec Execer
	// SkmanageExtraArgs is appended to every skmanage invocation — e.g.
	// --ssl-certificate/--ssl-key/--ssl-trustfile when the management
	// endpoint is amqps (nonkube sites).
	SkmanageExtraArgs []string
}

type Result struct {
	Total   int
	Killed  int
	Skipped int
	Failed  int
}

// Run ties the stages together: Gather (gather.go) collects
// raw router + kernel state, Evaluate (criteria.go) applies the idle-time
// criteria against that state, and killAll (kill.go) carries out whatever
// Evaluate decided.
func Run(cfg Config) (Result, error) {
	if cfg.Exec == nil {
		cfg.Exec = LocalExec
	}
	snap, err := Gather(cfg.Exec, cfg.Skmanage, cfg.URL, cfg.SkmanageExtraArgs...)
	if err != nil {
		return Result{}, err
	}

	toKill := Evaluate(snap, time.Duration(cfg.IdleThresholdSecs)*time.Second)
	logf("total:%d  idle-orphan:%d", len(snap.TCPConns), len(toKill))

	if len(toKill) == 0 {
		logf("No idle/orphaned connections found.")
		return Result{Total: len(snap.TCPConns)}, nil
	}

	if !cfg.Execute {
		logf("Found %d idle connection(s) — re-run with --execute to close them:", len(toKill))
		for _, d := range toKill {
			fmt.Printf("  id=%-6s  host=%-25s  dir=%s  uptime=%-10s  reason=%s\n",
				d.Conn.Identity, d.Conn.Host, d.Conn.Dir, fmtSeconds(d.Conn.UptimeSeconds), d.Reason)
		}
		return Result{Total: len(snap.TCPConns), Skipped: len(toKill)}, nil
	}

	logf("--- KILLING %d connection(s) ---", len(toKill))
	killed, failed := killAll(cfg.Exec, cfg.Skmanage, cfg.URL, cfg.SkmanageExtraArgs, toKill)

	return Result{Total: len(snap.TCPConns), Killed: killed, Failed: failed}, nil
}

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
