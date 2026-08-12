package sweeper

import (
	"bytes"
	"reflect"
	"testing"
)

func TestSummarizePorts(t *testing.T) {
	// 'in' connections are counted by their LocalSocket port (the listener) and
	// 'out' connections by their Host port (the backend); the other side of
	// each is an ephemeral port that must not be grouped on.
	conns := []connInfo{
		{Identity: "1", Dir: "in", Host: "10.0.0.9:41002", LocalSocket: "10.0.0.2:8080"},
		{Identity: "2", Dir: "in", Host: "10.0.0.9:41004", LocalSocket: "10.0.0.2:8080"},
		{Identity: "3", Dir: "in", Host: "10.0.0.9:41006", LocalSocket: "10.0.0.2:9090"},
		{Identity: "4", Dir: "out", Host: "10.0.0.5:8080", LocalSocket: "10.0.0.2:52001"},
		{Identity: "5", Dir: "out", Host: "10.0.0.5:8080", LocalSocket: "10.0.0.2:52002"},
		{Identity: "6", Dir: "out", Host: "10.0.0.7:5432", LocalSocket: "10.0.0.2:52003"},
	}

	want := []PortStat{
		{Port: 8080, In: 2, Out: 2},
		{Port: 5432, In: 0, Out: 1},
		{Port: 9090, In: 1, Out: 0},
	}
	if got := summarizePorts(conns); !reflect.DeepEqual(got, want) {
		t.Errorf("summarizePorts() = %+v, want %+v", got, want)
	}
}

func TestSummarizePortsSkipsUnusableAddresses(t *testing.T) {
	conns := []connInfo{
		{Identity: "1", Dir: "in", LocalSocket: ""},
		{Identity: "2", Dir: "in", LocalSocket: "10.0.0.2"},
		{Identity: "3", Dir: "out", Host: "10.0.0.5:not-a-port"},
		{Identity: "4", Dir: "unknown", Host: "10.0.0.5:8080", LocalSocket: "10.0.0.2:8080"},
		{Identity: "5", Dir: "out", Host: "[::1]:8080", LocalSocket: "10.0.0.2:52001"},
	}

	want := []PortStat{{Port: 8080, In: 0, Out: 1}}
	if got := summarizePorts(conns); !reflect.DeepEqual(got, want) {
		t.Errorf("summarizePorts() = %+v, want %+v", got, want)
	}
}

func TestMergePortStats(t *testing.T) {
	podA := []PortStat{{Port: 8080, In: 14, Out: 14}, {Port: 9090, In: 2}}
	podB := []PortStat{{Port: 8080, In: 9, Out: 9}, {Port: 5432, Out: 9}}

	want := []PortStat{
		{Port: 8080, In: 23, Out: 23},
		{Port: 5432, In: 0, Out: 9},
		{Port: 9090, In: 2, Out: 0},
	}
	if got := MergePortStats(podA, podB); !reflect.DeepEqual(got, want) {
		t.Errorf("MergePortStats() = %+v, want %+v", got, want)
	}
}

func TestSortedStatsTiesBreakOnPort(t *testing.T) {
	stats := MergePortStats([]PortStat{
		{Port: 9090, In: 1, Out: 1},
		{Port: 5432, In: 2},
		{Port: 8080, In: 1, Out: 1},
	})
	wantPorts := []int{5432, 8080, 9090}
	for i, want := range wantPorts {
		if stats[i].Port != want {
			t.Errorf("stats[%d].Port = %d, want %d", i, stats[i].Port, want)
		}
	}
}

func TestPrintPortStatsEmpty(t *testing.T) {
	var buf bytes.Buffer
	PrintPortStats(&buf, nil, nil)
	if got, want := buf.String(), "No TCP adaptor connections found.\n"; got != want {
		t.Errorf("PrintPortStats() = %q, want %q", got, want)
	}

	// A --port selection that matched nothing must not read like an unfiltered
	// empty router.
	buf.Reset()
	PrintPortStats(&buf, nil, []int{8080, 9090})
	if got, want := buf.String(), "No connections found on port 8080, 9090.\n"; got != want {
		t.Errorf("PrintPortStats() = %q, want %q", got, want)
	}
}

func TestFilterByPorts(t *testing.T) {
	conns := []connInfo{
		{Identity: "1", Dir: "in", Host: "10.0.0.9:41002", LocalSocket: "10.0.0.2:8080"},
		{Identity: "2", Dir: "in", Host: "10.0.0.9:41004", LocalSocket: "10.0.0.2:9090"},
		{Identity: "3", Dir: "out", Host: "10.0.0.5:8080", LocalSocket: "10.0.0.2:52001"},
		{Identity: "4", Dir: "out", Host: "10.0.0.7:5432", LocalSocket: "10.0.0.2:52002"},
	}

	// 8080 must match on both legs, and must not match the ephemeral ports
	// that share the other side of each connection.
	got := FilterByPorts(conns, []int{8080})
	if len(got) != 2 || got[0].Identity != "1" || got[1].Identity != "3" {
		t.Errorf("FilterByPorts(8080) = %+v, want connections 1 and 3", got)
	}

	if got := FilterByPorts(conns, []int{8080, 5432}); len(got) != 3 {
		t.Errorf("FilterByPorts(8080, 5432) matched %d connections, want 3", len(got))
	}
	if got := FilterByPorts(conns, []int{52001}); len(got) != 0 {
		t.Errorf("FilterByPorts(52001) matched %d connections, want 0 (ephemeral port)", len(got))
	}
	if got := FilterByPorts(conns, nil); len(got) != len(conns) {
		t.Errorf("FilterByPorts(nil) matched %d connections, want all %d", len(got), len(conns))
	}
}

func TestValidatePorts(t *testing.T) {
	if err := ValidatePorts([]int{1, 8080, 65535}); err != nil {
		t.Errorf("ValidatePorts() rejected valid ports: %v", err)
	}
	for _, invalid := range [][]int{{0}, {-1}, {65536}, {8080, 70000}} {
		if err := ValidatePorts(invalid); err == nil {
			t.Errorf("ValidatePorts(%v) = nil, want an error", invalid)
		}
	}
}
