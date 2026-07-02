package qdr

import "testing"

func TestRecoverPortMappingPreservesListenerPorts(t *testing.T) {
	config := InitialConfig("router", "site", "test", false, 3)
	config.Bridges.AddTcpListener(TcpEndpoint{
		Name:    TcpListenerNamePrefix + "frontend",
		Port:    "12001",
		Address: "frontend",
	})
	config.Bridges.AddTcpListener(TcpEndpoint{
		Name:    TcpListenerNamePrefix + "backend",
		Port:    "12002",
		Address: "backend",
	})
	config.Bridges.AddTcpListener(TcpEndpoint{
		Name:    TcpListenerNamePrefix + "per-target@pod-a",
		Port:    "12003",
		Address: "per-target.pod-a",
	})
	config.Bridges.AddTcpListener(TcpEndpoint{
		Name: "multiAddress/multi",
		Port: "12004",
	})

	mapping := RecoverPortMapping(&config)

	assertPortForKey(t, mapping, "frontend", 12001)
	assertPortForKey(t, mapping, "backend", 12002)
	assertPortForKey(t, mapping, "per-target.pod-a", 12003)
	assertPortForKey(t, mapping, "multiaddress-multi", 12004)
}

func assertPortForKey(t *testing.T, mapping *PortMapping, key string, expected int) {
	t.Helper()
	actual, err := mapping.GetPortForKey(key)
	if err != nil {
		t.Fatalf("GetPortForKey(%q) returned error: %v", key, err)
	}
	if actual != expected {
		t.Fatalf("GetPortForKey(%q) = %d, want %d", key, actual, expected)
	}
}
