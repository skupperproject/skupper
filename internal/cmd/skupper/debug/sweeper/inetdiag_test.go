package sweeper

import (
	"reflect"
	"testing"
)

// diagOutput is a capture of inetDiagScript against the same two sockets as
// ssOutput
const diagOutput = `127.0.0.1:8080 127.0.0.1:39068 5022 2020
127.0.0.1:39068 127.0.0.1:8080 2020 5022
`

func TestSocketsFromDiagOutput(t *testing.T) {
	byPeer, byLocal := socketsFromDiagOutput([]byte(diagOutput))

	wantByPeer := map[string]socketInfo{
		"127.0.0.1:39068": {LastRcvMs: 5022, LastSndMs: 2020},
		"127.0.0.1:8080":  {LastRcvMs: 2020, LastSndMs: 5022},
	}
	wantByLocal := map[string]socketInfo{
		"127.0.0.1:8080":  {LastRcvMs: 5022, LastSndMs: 2020},
		"127.0.0.1:39068": {LastRcvMs: 2020, LastSndMs: 5022},
	}
	if !reflect.DeepEqual(byPeer, wantByPeer) {
		t.Errorf("byPeer = %+v, want %+v", byPeer, wantByPeer)
	}
	if !reflect.DeepEqual(byLocal, wantByLocal) {
		t.Errorf("byLocal = %+v, want %+v", byLocal, wantByLocal)
	}
}

func TestSocketsFromDiagOutputSkipsUnparseableLines(t *testing.T) {
	// Anything that is not four fields of the expected shape is dropped.
	const out = `127.0.0.1:8080 127.0.0.1:41002 9119
127.0.0.1:8080 127.0.0.1:41004 9119 592 extra
127.0.0.1:8080 127.0.0.1:41006 nine 592
127.0.0.1:8080 127.0.0.1:41008 9119 five-ninety-two

127.0.0.1:8080 127.0.0.1:41010 9119 592
`
	byPeer, byLocal := socketsFromDiagOutput([]byte(out))

	want := map[string]socketInfo{"127.0.0.1:41010": {LastRcvMs: 9119, LastSndMs: 592}}
	if !reflect.DeepEqual(byPeer, want) {
		t.Errorf("byPeer = %+v, want %+v", byPeer, want)
	}
	if len(byLocal) != 1 {
		t.Errorf("byLocal has %d entries, want 1", len(byLocal))
	}
}

func TestSocketsFromDiagOutputEmpty(t *testing.T) {
	byPeer, byLocal := socketsFromDiagOutput(nil)
	if byPeer == nil || byLocal == nil {
		t.Fatal("socketsFromDiagOutput(nil) returned a nil map; callers index it directly")
	}
	if len(byPeer) != 0 || len(byLocal) != 0 {
		t.Errorf("socketsFromDiagOutput(nil) = %+v/%+v, want empty", byPeer, byLocal)
	}
}

func TestSSAndDiagParsersAgree(t *testing.T) {
	const toleranceMs = 100

	ssByPeer, ssByLocal := socketsFromSS([]byte(ssOutput))
	diagByPeer, diagByLocal := socketsFromDiagOutput([]byte(diagOutput))

	for _, m := range []struct {
		name     string
		ss, diag map[string]socketInfo
	}{
		{"byPeer", ssByPeer, diagByPeer},
		{"byLocal", ssByLocal, diagByLocal},
	} {
		if len(m.ss) != len(m.diag) {
			t.Errorf("%s: ss produced %d entries, diag produced %d", m.name, len(m.ss), len(m.diag))
		}
		for key, want := range m.ss {
			got, ok := m.diag[key]
			if !ok {
				t.Errorf("%s: diag is missing key %s that ss reported", m.name, key)
				continue
			}
			if abs(got.LastRcvMs-want.LastRcvMs) > toleranceMs || abs(got.LastSndMs-want.LastSndMs) > toleranceMs {
				t.Errorf("%s[%s]: ss = %+v, diag = %+v (differ by more than %d ms)", m.name, key, want, got, toleranceMs)
			}
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
