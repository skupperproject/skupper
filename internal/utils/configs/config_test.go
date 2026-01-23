package configs

import (
	"encoding/json"
	"testing"
)

func TestConnectJson(t *testing.T) {
	for _, test := range []struct {
		name string
		host string
	}{
		{name: "localhost", host: "localhost"},
		{name: "localhost ip", host: "127.0.0.1"},
		{name: "ip address", host: "192.168.1.1"},
		{name: "domain", host: "example.com"},
		{name: "domain with subdomain", host: "subdomain.example.com"},
		{name: "domain with numbers", host: "12345.example.com"},
		{name: "kubernetes service DNS", host: "my-svc.my-namespace.svc.cluster-domain.example"},
		{name: "empty string", host: ""},
	}{
		t.Run(test.name, func(t *testing.T) {
			cj := ConnectJson(test.host)
			assertConnectJsonResult(t, cj, test.host)
		})
	}
}

func assertConnectJsonResult(t *testing.T, cj string, expectedHost string) {
	if cj == "" {
		t.Error("ConnectJson() returned an empty string")
		return
	}

	var result map[string]any
	err := json.Unmarshal([]byte(cj), &result)
	if err != nil {
		t.Errorf("Failed to unmarshal JSON: %v", err)
		return
	}

	if result["host"] != expectedHost {
		t.Errorf("Expected host %s, got %v", expectedHost, result["host"])
	}

	if result["scheme"] != "amqps" {
		t.Errorf("Expected scheme 'amqps', got %v", result["scheme"])
	}

	if result["port"] != "5671" {
		t.Errorf("Expected port '5671', got %v", result["port"])
	}

	assertTlsFields(t, result)
}

func assertTlsFields(t *testing.T, result map[string]any) {
	tls, ok := result["tls"].(map[string]any)
	if !ok {
		t.Error("Expected tls object in JSON")
		return
	}

	if tls["ca"] != "/etc/messaging/ca.crt" {
		t.Errorf("Expected ca '/etc/messaging/ca.crt', got %v", tls["ca"])
	}

	if tls["cert"] != "/etc/messaging/tls.crt" {
		t.Errorf("Expected cert '/etc/messaging/tls.crt', got %v", tls["cert"])
	}

	if tls["key"] != "/etc/messaging/tls.key" {
		t.Errorf("Expected key '/etc/messaging/tls.key', got %v", tls["key"])
	}

	if tls["verify"] != true {
		t.Error("Expected verify to be true")
	}
}