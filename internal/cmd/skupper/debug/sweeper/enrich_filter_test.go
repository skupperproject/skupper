package sweeper

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestNormalizeStates(t *testing.T) {
	got, err := NormalizeStates([]string{"estab", "FIN-WAIT-2", "CLOSE", "established"})
	if err != nil {
		t.Fatalf("NormalizeStates returned %v", err)
	}
	want := []string{"ESTAB", "FIN-WAIT-2", "UNCONN"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NormalizeStates() = %v, want %v", got, want)
	}

	if _, err := NormalizeStates([]string{"NOT-A-STATE"}); err == nil {
		t.Error("NormalizeStates accepted an unknown state")
	}
}

func TestFilterByStates(t *testing.T) {
	snap := Snapshot{
		TCPConns: []connInfo{
			{Identity: "1", Dir: "in", Host: "10.0.0.9:41002", LocalSocket: "10.0.0.2:8080"},
			{Identity: "2", Dir: "in", Host: "10.0.0.9:41004", LocalSocket: "10.0.0.2:8080"},
			{Identity: "3", Dir: "out", Host: "10.0.0.5:8080", LocalSocket: "10.0.0.2:52001"},
		},
		Sockets: map[string]socketInfo{
			"10.0.0.9:41002": {State: "ESTAB", LastRcvMs: 100, LastSndMs: 100},
			"10.0.0.9:41004": {State: "FIN-WAIT-2", LastRcvMs: 100, LastSndMs: 100},
		},
		SocketsByLocal: map[string]socketInfo{
			"10.0.0.2:52001": {State: "ESTAB", LastRcvMs: 100, LastSndMs: 100},
		},
	}

	got := FilterByStates(snap, []string{"ESTAB"})
	if len(got) != 2 || got[0].Identity != "1" || got[1].Identity != "3" {
		t.Errorf("FilterByStates(ESTAB) = %+v, want connections 1 and 3", got)
	}

	// Unmatched sockets are excluded when a state filter is set.
	snap.TCPConns = append(snap.TCPConns, connInfo{Identity: "4", Dir: "in", Host: "10.0.0.9:49999", LocalSocket: "10.0.0.2:8080"})
	got = FilterByStates(snap, []string{"ESTAB"})
	for _, c := range got {
		if c.Identity == "4" {
			t.Error("FilterByStates included a connection with no matched socket")
		}
	}

	if got := FilterByStates(snap, nil); len(got) != len(snap.TCPConns) {
		t.Errorf("FilterByStates(nil) matched %d, want all %d", len(got), len(snap.TCPConns))
	}
}

func TestEnrichSnapshotCorrelatesPorts(t *testing.T) {
	snap := Snapshot{
		TCPConns: []connInfo{
			{Identity: "1", Dir: "in", Host: "10.0.0.9:41002", LocalSocket: "10.0.0.2:1024"},
			{Identity: "2", Dir: "out", Host: "10.0.0.5:9090", LocalSocket: "10.0.0.2:52001"},
		},
		Sockets: map[string]socketInfo{
			"10.0.0.9:41002": {State: "ESTAB"},
		},
		SocketsByLocal: map[string]socketInfo{
			"10.0.0.2:52001": {State: "CLOSE-WAIT"},
		},
		Listeners: []tcpEndpointInfo{
			{Name: "listener/frontend", Port: "1024", Address: "echo:8080"},
		},
		Connectors: []tcpEndpointInfo{
			{Name: "connector/backend@10.0.0.5", Port: "9090", Address: "echo:8080"},
		},
	}
	enrichSnapshot(&snap)

	if snap.TCPConns[0].Port != 1024 || snap.TCPConns[0].RoutingKey != "echo:8080" ||
		snap.TCPConns[0].Resource != "listener/frontend" || snap.TCPConns[0].Kind != "listener" ||
		snap.TCPConns[0].State != "ESTAB" {
		t.Errorf("inbound enrichment = %+v", snap.TCPConns[0])
	}
	if snap.TCPConns[1].Port != 9090 || snap.TCPConns[1].RoutingKey != "echo:8080" ||
		snap.TCPConns[1].Resource != "connector/backend@10.0.0.5" || snap.TCPConns[1].Kind != "connector" ||
		snap.TCPConns[1].State != "CLOSE-WAIT" {
		t.Errorf("outbound enrichment = %+v", snap.TCPConns[1])
	}
}

func TestEnrichSnapshotAmbiguousPort(t *testing.T) {
	snap := Snapshot{
		TCPConns: []connInfo{
			{Identity: "1", Dir: "in", Host: "10.0.0.9:41002", LocalSocket: "10.0.0.2:1024"},
		},
		Listeners: []tcpEndpointInfo{
			{Name: "listener/a", Port: "1024", Address: "key-a"},
			{Name: "listener/b", Port: "1024", Address: "key-b"},
		},
	}
	enrichSnapshot(&snap)
	if snap.TCPConns[0].Resource != "listener/a (+1)" {
		t.Errorf("resource = %q, want listener/a (+1)", snap.TCPConns[0].Resource)
	}
	if snap.TCPConns[0].RoutingKey != "key-a,key-b" {
		t.Errorf("routingKey = %q, want key-a,key-b", snap.TCPConns[0].RoutingKey)
	}
}

func TestFilterByRoutingKeys(t *testing.T) {
	conns := []connInfo{
		{Identity: "1", RoutingKey: "echo:8080"},
		{Identity: "2", RoutingKey: "db:5432"},
		{Identity: "3", RoutingKey: "echo:8080,other"},
		{Identity: "4"},
	}
	got := FilterByRoutingKeys(conns, []string{"echo:8080"})
	if len(got) != 2 || got[0].Identity != "1" || got[1].Identity != "3" {
		t.Errorf("FilterByRoutingKeys = %+v, want 1 and 3", got)
	}
	if got := FilterByRoutingKeys(conns, nil); len(got) != len(conns) {
		t.Errorf("FilterByRoutingKeys(nil) matched %d, want %d", len(got), len(conns))
	}
}

func TestPrintDecisionsJSON(t *testing.T) {
	uptime := 120
	decisions := []Decision{{
		Conn: connInfo{
			Identity: "7", Host: "10.0.0.9:41002", Dir: "in", Port: 1024,
			State: "ESTAB", UptimeSeconds: &uptime, RoutingKey: "echo:8080",
			Resource: "listener/frontend", Kind: "listener",
		},
		Reason: "idle for 4h0m0s",
	}}
	var buf bytes.Buffer
	if err := printDecisions(&buf, decisions, OutputJSON); err != nil {
		t.Fatal(err)
	}
	var reports []ConnReport
	if err := json.Unmarshal(buf.Bytes(), &reports); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(reports) != 1 || reports[0].Identity != "7" || reports[0].RoutingKey != "echo:8080" {
		t.Errorf("reports = %+v", reports)
	}
}

func TestPrintPortStatsJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := PrintPortStatsOutput(&buf, []PortStat{{Port: 8080, In: 1, Out: 1, RoutingKey: "echo", Resource: "listener/x"}}, nil, OutputJSON); err != nil {
		t.Fatal(err)
	}
	var reports []PortReport
	if err := json.Unmarshal(buf.Bytes(), &reports); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(reports) != 1 || reports[0].Total != 2 || reports[0].RoutingKey != "echo" {
		t.Errorf("reports = %+v", reports)
	}
}

func TestValidateOutput(t *testing.T) {
	if err := ValidateOutput(""); err != nil {
		t.Errorf("empty output should be valid: %v", err)
	}
	if err := ValidateOutput("json"); err != nil {
		t.Errorf("json should be valid: %v", err)
	}
	if err := ValidateOutput("yaml"); err == nil {
		t.Error("yaml should be rejected")
	}
}
