package sweeper

import "strings"

// killAll force-closes each flagged connection by setting adminStatus=deleted
// via skmanage. When output is json, results accumulate in reports instead of
// being printed line-by-line.
func killAll(execFn Execer, skmanageBin, url string, extraArgs []string, decisions []Decision, output string) (killed, failed int, reports []ConnReport) {
	for _, d := range decisions {
		_, err := runSkmanage(execFn, skmanageBin, url, extraArgs,
			"UPDATE",
			"--type="+ConnType,
			"--identity="+d.Conn.Identity,
			"adminStatus=deleted",
		)
		if err == nil {
			printKillResult(nil, d, "killed", "", output, &reports)
			killed++
			continue
		}
		// Killing one half of a proxied pair cascade-closes the other half,
		// so a later kill of that half fails with "not found".
		_, readErr := runSkmanage(execFn, skmanageBin, url, extraArgs,
			"READ", "--type="+ConnType, "--identity="+d.Conn.Identity)
		if readErr != nil && isNotFound(readErr) {
			printKillResult(nil, d, "already closed", "", output, &reports)
			killed++
			continue
		}
		printKillResult(nil, d, "failed", err.Error(), output, &reports)
		failed++
	}
	return killed, failed, reports
}

// isNotFound reports whether a skmanage READ error indicates the connection no
// longer exists rather than the router being unreachable or some other
// management failure that leaves the connection's fate unknown.
func isNotFound(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}
