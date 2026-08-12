package sweeper

import "strings"

// killAll force-closes each flagged connection by setting adminStatus=deleted
// via skmanage.
func killAll(execFn Execer, skmanageBin, url string, extraArgs []string, decisions []Decision) (killed, failed int) {
	for _, d := range decisions {
		_, err := runSkmanage(execFn, skmanageBin, url, extraArgs,
			"UPDATE",
			"--type="+ConnType,
			"--identity="+d.Conn.Identity,
			"adminStatus=deleted",
		)
		if err == nil {
			logf("  id=%s  host=%s  dir=%s  uptime=%s  reason=%s  → killed",
				d.Conn.Identity, d.Conn.Host, d.Conn.Dir,
				fmtSeconds(d.Conn.UptimeSeconds), d.Reason)
			killed++
			continue
		}
		// Killing one half of a proxied pair cascade-closes the other half,
		// so a later kill of that half fails with "not found".
		_, readErr := runSkmanage(execFn, skmanageBin, url, extraArgs,
			"READ", "--type="+ConnType, "--identity="+d.Conn.Identity)
		if readErr != nil && isNotFound(readErr) {
			logf("  id=%s  host=%s  dir=%s  reason=%s  → already closed",
				d.Conn.Identity, d.Conn.Host, d.Conn.Dir, d.Reason)
			killed++
			continue
		}
		logf("  id=%s  host=%s  dir=%s  reason=%s  → failed: %s",
			d.Conn.Identity, d.Conn.Host, d.Conn.Dir, d.Reason, err.Error())
		failed++
	}
	return killed, failed
}

// isNotFound reports whether a skmanage READ error indicates the connection no
// longer exists rather than the router being unreachable or some other
// management failure that leaves the connection's fate unknown.
func isNotFound(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}
