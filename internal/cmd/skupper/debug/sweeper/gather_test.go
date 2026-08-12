package sweeper

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// ssOutput is a capture of `ss -tin`: one header line per socket followed by a
// tab-indented detail line. The two sockets have deliberately asymmetric
// lastsnd/lastrcv values so a transposition shows up.
const ssOutput = `State Recv-Q Send-Q Local Address:Port  Peer Address:Port
ESTAB 12     0          127.0.0.1:8080     127.0.0.1:39068
	 cubic wscale:8,8 rto:201 rtt:0.01/0.007 ato:40 mss:32768 pmtu:65535 rcvmss:536 advmss:65483 cwnd:10 bytes_sent:12 bytes_acked:12 bytes_received:12 segs_out:2 segs_in:4 data_segs_out:1 data_segs_in:1 send 262144000000bps lastsnd:2004 lastrcv:5006 lastack:2004 pacing_rate 482103908040bps delivery_rate 10922666664bps delivered:2 app_limited rcv_space:65483 rcv_ssthresh:65483 minrtt:0.009 snd_wnd:65536 rcv_wnd:65536
ESTAB 12     0          127.0.0.1:39068    127.0.0.1:8080
	 cubic wscale:8,8 rto:201 rtt:0.014/0.008 ato:40 mss:32768 pmtu:65535 rcvmss:536 advmss:65483 cwnd:10 bytes_sent:12 bytes_acked:13 bytes_received:12 segs_out:4 segs_in:3 data_segs_out:1 data_segs_in:1 send 187245714286bps lastsnd:5006 lastrcv:2004 lastack:2004 pacing_rate 364722086952bps delivery_rate 87381333328bps delivered:2 app_limited rcv_space:65495 rcv_ssthresh:65495 minrtt:0.003 snd_wnd:65536 rcv_wnd:65536
`

// The server socket is the one local to :8080; the client socket is its peer.
var (
	wantListener = socketInfo{LastRcvMs: 5006, LastSndMs: 2004}
	wantClient   = socketInfo{LastRcvMs: 2004, LastSndMs: 5006}
)

func TestSocketsFromSS(t *testing.T) {
	byPeer, byLocal := socketsFromSS([]byte(ssOutput))

	if got := byPeer["127.0.0.1:39068"]; got != wantListener {
		t.Errorf("byPeer[39068] = %+v, want %+v", got, wantListener)
	}
	if got := byLocal["127.0.0.1:39068"]; got != wantClient {
		t.Errorf("byLocal[39068] = %+v, want %+v", got, wantClient)
	}
	if got := byPeer["127.0.0.1:8080"]; got != wantClient {
		t.Errorf("byPeer[8080] = %+v, want %+v", got, wantClient)
	}
	if got := byLocal["127.0.0.1:8080"]; got != wantListener {
		t.Errorf("byLocal[8080] = %+v, want %+v", got, wantListener)
	}
}

// ssSharedPortOutput holds two client connections to one server, so all four
// sockets have port 8080 on one side. Detail lines are abridged.
const ssSharedPortOutput = `State Recv-Q Send-Q Local Address:Port  Peer Address:Port
ESTAB 5      0          127.0.0.1:8080     127.0.0.1:49714
	 cubic wscale:8,8 rto:201 ato:40 mss:32768 cwnd:10 bytes_received:5 send 655360000000bps lastsnd:2004 lastrcv:2004 lastack:2004 delivered:1 app_limited
ESTAB 0      0          127.0.0.1:49704    127.0.0.1:8080
	 cubic wscale:8,8 rto:201 mss:32768 cwnd:10 bytes_sent:5 send 145635555556bps lastsnd:2004 lastrcv:2004 lastack:2004 delivered:2 app_limited
ESTAB 0      0          127.0.0.1:49714    127.0.0.1:8080
	 cubic wscale:8,8 rto:201 mss:32768 cwnd:10 bytes_sent:5 send 291271111111bps lastsnd:2004 lastrcv:2004 lastack:2004 delivered:2 app_limited
ESTAB 5      0          127.0.0.1:8080     127.0.0.1:49704
	 cubic wscale:8,8 rto:201 ato:40 mss:32768 cwnd:10 bytes_received:5 send 291271111111bps lastsnd:2004 lastrcv:2004 lastack:2004 delivered:1 app_limited
`

// Four sockets collapse into three entries per map: two share a local address
// and two share a peer address. Hence the two maps, one keyed each way.
func TestSocketsFromSSKeysCollideOnTheSharedSide(t *testing.T) {
	byPeer, byLocal := socketsFromSS([]byte(ssSharedPortOutput))

	if len(byPeer) != 3 || len(byLocal) != 3 {
		t.Fatalf("got %d byPeer / %d byLocal entries, want 3 each (4 sockets, one collision per map)", len(byPeer), len(byLocal))
	}
	for _, key := range []string{"127.0.0.1:49704", "127.0.0.1:49714"} {
		if _, ok := byPeer[key]; !ok {
			t.Errorf("byPeer is missing unique key %s", key)
		}
		if _, ok := byLocal[key]; !ok {
			t.Errorf("byLocal is missing unique key %s", key)
		}
	}
	// The shared key resolves to whichever socket parsed last; only presence
	// is asserted.
	if _, ok := byLocal["127.0.0.1:8080"]; !ok {
		t.Error("byLocal is missing the shared listener address")
	}
	if _, ok := byPeer["127.0.0.1:8080"]; !ok {
		t.Error("byPeer is missing the shared server address")
	}
}

func TestSocketsFromSSSkipsSocketsWithoutDetailLine(t *testing.T) {
	// A header with no detail line carries no timers, so it is left out rather
	// than recorded as zero idle.
	const out = `State Recv-Q Send-Q Local Address:Port  Peer Address:Port
ESTAB 0      0          127.0.0.1:8080     127.0.0.1:49704
ESTAB 0      0          127.0.0.1:8080     127.0.0.1:49714
	 cubic wscale:8,8 rto:201 lastsnd:100 lastrcv:200 lastack:100
`
	byPeer, byLocal := socketsFromSS([]byte(out))

	if _, ok := byPeer["127.0.0.1:49704"]; ok {
		t.Error("recorded a socket that had no detail line; it would report 0 ms idle")
	}
	if got, want := byPeer["127.0.0.1:49714"], (socketInfo{LastRcvMs: 200, LastSndMs: 100}); got != want {
		t.Errorf("byPeer[49714] = %+v, want %+v", got, want)
	}
	if len(byLocal) != 1 {
		t.Errorf("byLocal has %d entries, want 1", len(byLocal))
	}
}

func TestSocketsFromSSIgnoresUnpairedAndMalformedLines(t *testing.T) {
	// Defensive: a detail line with no header, and a header too short to index.
	const orphanDetail = "\t cubic lastsnd:1 lastrcv:2\n"
	if byPeer, byLocal := socketsFromSS([]byte(orphanDetail)); len(byPeer) != 0 || len(byLocal) != 0 {
		t.Errorf("orphan detail line produced %+v/%+v, want empty", byPeer, byLocal)
	}

	const shortHeader = "ESTAB 0 0 127.0.0.1:8080\n\t cubic lastsnd:1 lastrcv:2\n"
	if byPeer, byLocal := socketsFromSS([]byte(shortHeader)); len(byPeer) != 0 || len(byLocal) != 0 {
		t.Errorf("short header produced %+v/%+v, want empty", byPeer, byLocal)
	}

	byPeer, byLocal := socketsFromSS(nil)
	if byPeer == nil || byLocal == nil {
		t.Fatal("socketsFromSS(nil) returned a nil map; callers index it directly")
	}
	if len(byPeer) != 0 || len(byLocal) != 0 {
		t.Errorf("socketsFromSS(nil) = %+v/%+v, want empty", byPeer, byLocal)
	}
}

func TestExtractMsField(t *testing.T) {
	const line = "\t cubic wscale:8,8 rto:201 rtt:0.014/0.008 mss:32768 send 187245714286bps lastsnd:5006 lastrcv:2004 lastack:2004 pacing_rate 364722086952bps delivered:2 app_limited busy:1ms"

	tests := []struct {
		name string
		key  string
		want int
	}{
		{"value bounded by spaces", "lastsnd:", 5006},
		{"value followed by another key", "lastrcv:", 2004},
		{"key absent from line", "lastseen:", 0},
		// A unit suffix (busy:1ms) is not an integer, so it yields 0.
		{"value with a unit suffix", "busy:", 0},
		// pacing_rate's value is space-separated, so "pacing_rate:" never occurs.
		{"space-separated key is not a colon key", "pacing_rate:", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractMsField(line, tt.key); got != tt.want {
				t.Errorf("extractMsField(_, %q) = %d, want %d", tt.key, got, tt.want)
			}
		})
	}

	// The last field on a line has no trailing separator.
	if got := extractMsField("\t cubic rto:201 lastrcv:99", "lastrcv:"); got != 99 {
		t.Errorf("extractMsField(value at end of line) = %d, want 99", got)
	}
}

// ssNoTCPInfoOutput is what `ss -tin` prints when it cannot open a netlink
// socket: it warns on stderr, exits 0, and falls back to /proc/net/tcp, which
// lists the sockets but has no TCP_INFO, so no row gets a detail line. Trailing
// column padding is stripped.
const ssNoTCPInfoOutput = `State    Recv-Q Send-Q Local Address:Port  Peer Address:Port
ESTAB    0      0          127.0.0.1:8080     127.0.0.1:58822 rto:0.2 ato:0.04 qack:15 bidir cwnd:10 reordering:0
SYN-SENT 0      1         10.88.0.35:51890 10.255.255.1:9     rto:1 cwnd:1 ssthresh:7 retrans:4/0 reordering:0
ESTAB    0      0          127.0.0.1:58822    127.0.0.1:8080  rto:0.2 ato:0.04 qack:15 bidir cwnd:10 reordering:0
ESTAB    0      0              [::1]:44096        [::1]:8081  rto:0.2 ato:0.04 qack:15 bidir cwnd:10 reordering:0
ESTAB    0      0              [::1]:8081         [::1]:44096 rto:0.2 ato:0.04 qack:15 bidir cwnd:10 reordering:0
`

// fakeExec answers the two commands gatherSockets runs and records which of
// them it actually reached.
type fakeExec struct {
	ss, python   []byte
	ssErr, pyErr error
	called       []string
}

func (f *fakeExec) run(argv []string) ([]byte, error) {
	f.called = append(f.called, argv[0])
	switch argv[0] {
	case "ss":
		return f.ss, f.ssErr
	case "python3":
		return f.python, f.pyErr
	}
	return nil, errors.New("unexpected command " + argv[0])
}

func TestGatherSocketsFallsBackWhenSSReportsNoTCPInfo(t *testing.T) {
	f := &fakeExec{ss: []byte(ssNoTCPInfoOutput), python: []byte(diagOutput)}

	byPeer, byLocal, err := gatherSockets(f.run)
	if err != nil {
		t.Fatalf("gatherSockets returned %v, want nil", err)
	}
	wantPeer, wantLocal := socketsFromDiagOutput([]byte(diagOutput))
	if !reflect.DeepEqual(byPeer, wantPeer) || !reflect.DeepEqual(byLocal, wantLocal) {
		t.Errorf("got %+v/%+v, want the fallback's %+v/%+v", byPeer, byLocal, wantPeer, wantLocal)
	}
	if len(f.called) != 2 || f.called[1] != "python3" {
		t.Errorf("commands run = %v, want ss then python3", f.called)
	}
}

func TestGatherSocketsErrorsWhenSSHasNoTCPInfoAndFallbackFails(t *testing.T) {
	f := &fakeExec{ss: []byte(ssNoTCPInfoOutput), pyErr: errors.New("no python3")}

	_, _, err := gatherSockets(f.run)
	if err == nil {
		t.Fatal("gatherSockets succeeded with no readable socket state; every connection would be skipped and reported as nothing to close")
	}
	if !strings.Contains(err.Error(), "TCP_INFO") || !strings.Contains(err.Error(), "no python3") {
		t.Errorf("error = %q, want it to name both failures", err)
	}
}

func TestGatherSocketsAcceptsSSListingNoSockets(t *testing.T) {
	// A host with no TCP connections prints the column header and nothing
	// else. That is a real empty result, so the fallback is not run.
	f := &fakeExec{
		ss:    []byte("State Recv-Q Send-Q Local Address:Port  Peer Address:Port\n"),
		pyErr: errors.New("no python3"),
	}

	byPeer, byLocal, err := gatherSockets(f.run)
	if err != nil {
		t.Fatalf("gatherSockets returned %v, want nil", err)
	}
	if len(byPeer) != 0 || len(byLocal) != 0 {
		t.Errorf("got %+v/%+v, want empty", byPeer, byLocal)
	}
	if len(f.called) != 1 {
		t.Errorf("commands run = %v, want ss only", f.called)
	}
}

func TestGatherSocketsPrefersSSWhenItReportsTimers(t *testing.T) {
	f := &fakeExec{ss: []byte(ssOutput), pyErr: errors.New("no python3")}

	byPeer, _, err := gatherSockets(f.run)
	if err != nil {
		t.Fatalf("gatherSockets returned %v, want nil", err)
	}
	if got := byPeer["127.0.0.1:39068"]; got != wantListener {
		t.Errorf("byPeer[39068] = %+v, want %+v", got, wantListener)
	}
	if len(f.called) != 1 {
		t.Errorf("commands run = %v, want ss only", f.called)
	}
}
