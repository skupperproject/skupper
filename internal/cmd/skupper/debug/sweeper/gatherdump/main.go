// Throwaway debug tool: calls sweeper.Gather directly and dumps the results for debugging
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/sweeper"
)

func main() {
	url := flag.String("url", "amqp://127.0.0.1:5672", "Router management URL")
	skmanageBin := flag.String("skmanage", "skmanage", "Path to skmanage binary")
	flag.Parse()

	snap, err := sweeper.Gather(sweeper.LocalExec, *skmanageBin, *url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gather failed:", err)
		os.Exit(1)
	}

	fmt.Printf("gathered at %s from %s\n\n", snap.Now.Format("15:04:05"), *url)
	fmt.Printf("%-4s %-25s %-25s %-10s %-10s\n", "DIR", "HOST", "LOCALSOCKET", "UPTIME(s)", "LASTDLV(s)")
	for _, c := range snap.TCPConns {
		fmt.Printf("%-4s %-25s %-25s %-10s %-10s\n",
			c.Dir, c.Host, c.LocalSocket, ptrStr(c.UptimeSeconds), ptrStr(c.LastDlvSeconds))
	}

	fmt.Println("\nsockets by peer (host:port -> lastrcv/lastsnd s):")
	for host, s := range snap.Sockets {
		fmt.Printf("  %-25s lastrcv=%.1fs lastsnd=%.1fs\n", host, float64(s.LastRcvMs)/1000, float64(s.LastSndMs)/1000)
	}

	fmt.Println("\nsockets by local (host:port -> lastrcv/lastsnd s):")
	for host, s := range snap.SocketsByLocal {
		fmt.Printf("  %-25s lastrcv=%.1fs lastsnd=%.1fs\n", host, float64(s.LastRcvMs)/1000, float64(s.LastSndMs)/1000)
	}
}

func ptrStr(p *int) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *p)
}
