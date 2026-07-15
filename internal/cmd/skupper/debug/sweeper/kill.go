package sweeper

// killAll force-closes every TCP connection that criteria.go decided sequentially
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
		// so a later kill of that half fails with "not found". Confirm the
		// connection is really gone before calling it a failure.
		if _, readErr := runSkmanage(execFn, skmanageBin, url, extraArgs,
			"READ", "--type="+ConnType, "--identity="+d.Conn.Identity); readErr != nil {
			logf("  id=%s  host=%s  dir=%s  reason=%s  → already closed",
				d.Conn.Identity, d.Conn.Host, d.Conn.Dir, d.Reason)
			killed++
		} else {
			logf("  id=%s  host=%s  dir=%s  reason=%s  → failed: %s",
				d.Conn.Identity, d.Conn.Host, d.Conn.Dir,
				d.Reason, err.Error())
			failed++
		}
	}
	return killed, failed
}
