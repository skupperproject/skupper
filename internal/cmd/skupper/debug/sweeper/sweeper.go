package sweeper

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"
)

const (
	DefaultURL           = "amqp://127.0.0.1:5672"
	DefaultSkmanage      = "skmanage"
	DefaultIdleThreshold = 4 * 3600 // 4 hours
	MaxIdleThreshold     = math.MaxInt64 / int64(time.Second)
)

type Config struct {
	URL               string
	Skmanage          string
	IdleThresholdSecs int
	Execute           bool
	// Ports limits the sweep to connections on these router-side ports. Empty
	// means every port.
	Ports []int
	// States limits to connections whose kernel socket is in one of these
	// ss-style TCP states (after NormalizeStates). Empty means any state.
	States []string
	// RoutingKeys limits to connections whose correlated routing key matches.
	// Empty means any routing key.
	RoutingKeys []string
	// Output is "text" (default) or "json".
	Output string
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
	// Reports holds idle-list or kill-result rows. In JSON mode Run fills this
	// and does not write to stdout, so callers (especially multi-pod kube) can
	// emit a single JSON document.
	Reports []ConnReport
}

// Run ties the stages together: Gather (gather.go) collects
// raw router + kernel state, filters apply, Evaluate (criteria.go) applies the
// idle-time criteria, and killAll (kill.go) carries out whatever Evaluate decided.
// Text mode prints progress and rows to stdout; JSON mode only returns Reports.
func Run(cfg Config) (Result, error) {
	if cfg.IdleThresholdSecs < 0 || int64(cfg.IdleThresholdSecs) > MaxIdleThreshold {
		return Result{}, fmt.Errorf("idle threshold must be between 0 and %d seconds", MaxIdleThreshold)
	}
	if cfg.Exec == nil {
		cfg.Exec = LocalExec
	}
	if err := ValidatePorts(cfg.Ports); err != nil {
		return Result{}, err
	}
	states, err := NormalizeStates(cfg.States)
	if err != nil {
		return Result{}, err
	}
	if err := ValidateOutput(cfg.Output); err != nil {
		return Result{}, err
	}
	cfg.Output = NormalizeOutput(cfg.Output)
	cfg.States = states
	jsonMode := cfg.Output == OutputJSON

	snap, err := Gather(cfg.Exec, cfg.Skmanage, cfg.URL, len(cfg.RoutingKeys) > 0, cfg.SkmanageExtraArgs...)
	if err != nil {
		return Result{}, err
	}

	snap.TCPConns = applyFilters(snap, cfg)
	if len(cfg.Ports) > 0 || len(cfg.States) > 0 || len(cfg.RoutingKeys) > 0 {
		if len(snap.TCPConns) == 0 {
			if jsonMode {
				return Result{Reports: []ConnReport{}}, nil
			}
			logf("%s", filterEmptyMessage(cfg))
			return Result{}, nil
		}
	}

	toKill := Evaluate(snap, time.Duration(cfg.IdleThresholdSecs)*time.Second)
	if !jsonMode {
		logf("total:%d  idle-orphan:%d", len(snap.TCPConns), len(toKill))
	}

	if len(toKill) == 0 {
		if jsonMode {
			return Result{Total: len(snap.TCPConns), Reports: []ConnReport{}}, nil
		}
		logf("No idle/orphaned connections found.")
		return Result{Total: len(snap.TCPConns)}, nil
	}

	if !cfg.Execute {
		if jsonMode {
			return Result{
				Total:   len(snap.TCPConns),
				Skipped: len(toKill),
				Reports: decisionsToReports(toKill),
			}, nil
		}
		logf("Found %d idle connection(s) — re-run with --execute to close them:", len(toKill))
		if err := printDecisions(os.Stdout, toKill, cfg.Output); err != nil {
			return Result{}, err
		}
		return Result{Total: len(snap.TCPConns), Skipped: len(toKill)}, nil
	}

	if !jsonMode {
		logf("--- KILLING %d connection(s) ---", len(toKill))
	}
	killed, failed, reports := killAll(cfg.Exec, cfg.Skmanage, cfg.URL, cfg.SkmanageExtraArgs, toKill, cfg.Output)
	return Result{
		Total:   len(snap.TCPConns),
		Killed:  killed,
		Failed:  failed,
		Reports: reports,
	}, nil
}

func applyFilters(snap Snapshot, cfg Config) []connInfo {
	conns := snap.TCPConns
	if len(cfg.Ports) > 0 {
		conns = FilterByPorts(conns, cfg.Ports)
	}
	snap.TCPConns = conns
	if len(cfg.RoutingKeys) > 0 {
		conns = FilterByRoutingKeys(conns, cfg.RoutingKeys)
		snap.TCPConns = conns
	}
	if len(cfg.States) > 0 {
		conns = FilterByStates(snap, cfg.States)
	}
	return conns
}

func filterEmptyMessage(cfg Config) string {
	var parts []string
	if len(cfg.Ports) > 0 {
		parts = append(parts, "port "+FormatPorts(cfg.Ports))
	}
	if len(cfg.States) > 0 {
		parts = append(parts, "state "+joinQuoted(cfg.States))
	}
	if len(cfg.RoutingKeys) > 0 {
		parts = append(parts, "routing-key "+joinQuoted(cfg.RoutingKeys))
	}
	if len(parts) == 0 {
		return "No connections found."
	}
	msg := "No connections found matching"
	for i, p := range parts {
		if i == 0 {
			msg += " " + p
		} else {
			msg += ", " + p
		}
	}
	return msg + "."
}

func joinQuoted(vals []string) string {
	return strings.Join(vals, ", ")
}
