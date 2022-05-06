package qdr

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
	"gotest.tools/assert"
	"reflect"
	"testing"
)

func TestInitialConfig(t *testing.T) {
	config := InitialConfig("foo", "bar", "1.2.3", true, 10)
	if config.Metadata.Id != "foo" {
		t.Errorf("Invalid id, expected 'foo' got %q", config.Metadata.Id)
	}
	if getSiteMetadata(config.Metadata.Metadata).Id != "bar" {
		t.Errorf("Invalid metadata, expected id to be 'bar' got %q", getSiteMetadata(config.Metadata.Metadata).Id)
	}
	if getSiteMetadata(config.Metadata.Metadata).Version != "1.2.3" {
		t.Errorf("Invalid metadata, expected version to be '1.2.3' got %q", getSiteMetadata(config.Metadata.Metadata).Version)
	}
	if config.Metadata.Mode != ModeEdge {
		t.Errorf("Invalid id, expected %q got %q", ModeEdge, config.Metadata.Mode)
	}
	if config.Metadata.HelloMaxAgeSeconds != "10" {
		t.Errorf("Invalid id, expected %q got %q", "10", config.Metadata.HelloMaxAgeSeconds)
	}
	config = InitialConfig("bing", "bong", "3.2.1", false, 5)
	if config.Metadata.Id != "bing" {
		t.Errorf("Invalid id, expected 'bing' got %q", config.Metadata.Id)
	}
	if getSiteMetadata(config.Metadata.Metadata).Id != "bong" {
		t.Errorf("Invalid metadata, expectedsite id to be 'bong' got %q", getSiteMetadata(config.Metadata.Metadata).Id)
	}
	if getSiteMetadata(config.Metadata.Metadata).Version != "3.2.1" {
		t.Errorf("Invalid metadata, expected version to be '3.2.1' got %q", getSiteMetadata(config.Metadata.Metadata).Version)
	}
	if config.Metadata.Mode != ModeInterior {
		t.Errorf("Invalid id, expected %q got %q", ModeInterior, config.Metadata.Mode)
	}
	if config.Metadata.HelloMaxAgeSeconds != "5" {
		t.Errorf("Invalid id, expected %q got %q", "10", config.Metadata.HelloMaxAgeSeconds)
	}
}

func TestAddListener(t *testing.T) {
	config := InitialConfig("foo", "bar", "undefined", true, 3)
	config.AddListener(Listener{
		Name: "l1",
		Port: 5672,
	})
	if config.Listeners["l1"].Port != 5672 {
		t.Errorf("Expected port 5672 but got %d", config.Listeners["l1"].Port)
	}
	config.AddListener(Listener{
		Host: "127.0.0.1",
		Port: 8888,
	})
	if config.Listeners["127.0.0.1@8888"].Port != 8888 {
		t.Errorf("Expected port 8888 but got %d", config.Listeners["127.0.0.1@8888"].Port)
	}
	if config.Listeners["127.0.0.1@8888"].Name != "127.0.0.1@8888" {
		t.Errorf("Expected name '127.0.0.1@8888' but got %q", config.Listeners["127.0.0.1@8888"].Name)
	}
}

func TestAddSslProfile(t *testing.T) {
	config := InitialConfig("foo", "bar", "undefined", true, 3)
	config.AddSslProfile(SslProfile{
		Name:     "myprofile",
		CertFile: "/my/certs/cert.pem",
	})
	if config.SslProfiles["myprofile"].CertFile != "/my/certs/cert.pem" {
		t.Errorf("Expected cert file '/my/certs/cert.pem' but got %q", config.SslProfiles["myprofile"].CertFile)
	}
	config.AddSslProfile(SslProfile{
		Name: "another",
	})
	if config.SslProfiles["another"].CertFile != "/etc/skupper-router-certs/another/tls.crt" {
		t.Errorf("Expected cert file '/etc/skupper-router-certs/another/tls.crt' but got %q", config.SslProfiles["another"].CertFile)
	}
	if config.SslProfiles["another"].CaCertFile != "/etc/skupper-router-certs/another/ca.crt" {
		t.Errorf("Expected cert file '/etc/skupper-router-certs/another/ca.crt' but got %q", config.SslProfiles["another"].CaCertFile)
	}
	if config.SslProfiles["another"].PrivateKeyFile != "/etc/skupper-router-certs/another/tls.key" {
		t.Errorf("Expected cert file '/etc/skupper-router-certs/another/tls.key' but got %q", config.SslProfiles["another"].PrivateKeyFile)
	}
}

func TestAddAddress(t *testing.T) {
	config := InitialConfig("foo", "bar", "undefined", true, 3)
	config.AddAddress(Address{
		Prefix:       "foo",
		Distribution: DistributionMulticast,
	})
	if config.Addresses["foo"].Distribution != "multicast" {
		t.Errorf("Expected distribution %q but got %q", DistributionMulticast, config.Addresses["foo"].Distribution)
	}
}

func TestMarshalUnmarshalRouterConfig(t *testing.T) {
	verifyHostName := new(bool)
	*verifyHostName = false

	input := RouterConfig{
		Metadata: RouterMetadata{
			Id:                 "${HOSTNAME}",
			Mode:               ModeEdge,
			Metadata:           "MySiteId",
			HelloMaxAgeSeconds: "5",
		},
		SslProfiles: map[string]SslProfile{
			"one": SslProfile{
				Name:           "one",
				CertFile:       "/somewhere/myCert.pem",
				CaCertFile:     "/somewhere/myCA.pem",
				PrivateKeyFile: "/somewhere/myKey.pem",
			},
			"two": SslProfile{
				Name:           "two",
				CertFile:       "/somewhere/else/myCert.pem",
				CaCertFile:     "/somewhere/else/myCA.pem",
				PrivateKeyFile: "/somewhere/else/myKey.pem",
			},
		},
		Connectors: map[string]Connector{
			"c1": Connector{
				Name:       "c1",
				Host:       "somewhere.com",
				Port:       "1234",
				SslProfile: "one",
			},
			"c2": Connector{
				Name:       "c2",
				Host:       "elsewhere.com",
				Port:       "5678",
				SslProfile: "two",
			},
		},
		Listeners: map[string]Listener{
			"l1": Listener{
				Name:             "l1",
				Host:             "127.0.0.1",
				Port:             1234,
				SslProfile:       "one",
				AuthenticatePeer: true,
			},
			"l2": Listener{
				Name:       "l2",
				Host:       "0.0.0.0",
				Port:       5678,
				SslProfile: "two",
				Cost:       101,
			},
		},
		Bridges: BridgeConfig{
			TcpConnectors: map[string]TcpEndpoint{
				"c1": TcpEndpoint{
					Name:    "c1",
					Address: "foo",
					Host:    "somewhere.com",
					Port:    "1234",
					SiteId:  "abc",
				},
				"c2": TcpEndpoint{
					Name:    "c2",
					Address: "bar",
					Host:    "elsewhere.com",
					Port:    "5678",
					SiteId:  "def",
				},
			},
			TcpListeners: map[string]TcpEndpoint{
				"l1": TcpEndpoint{
					Name:    "l1",
					Address: "apples",
					Host:    "0.0.0.0",
					Port:    "1234",
					SiteId:  "abc",
				},
				"l2": TcpEndpoint{
					Name:    "l2",
					Address: "oranges",
					Host:    "localhost",
					Port:    "5678",
					SiteId:  "def",
				},
			},
			HttpConnectors: map[string]HttpEndpoint{
				"c3": HttpEndpoint{
					Name:    "c3",
					Address: "foo",
					Host:    "nowhere.com",
					Port:    "4321",
					SiteId:  "abc",
				},
				"c4": HttpEndpoint{
					Name:           "c4",
					Address:        "bar",
					Host:           "here.com",
					Port:           "8765",
					SiteId:         "def",
					VerifyHostname: verifyHostName,
				},
			},
			HttpListeners: map[string]HttpEndpoint{
				"l3": HttpEndpoint{
					Name:    "l3",
					Address: "green",
					Host:    "0.0.0.0",
					Port:    "4321",
					SiteId:  "abc",
				},
				"l4": HttpEndpoint{
					Name:    "l4",
					Address: "blue",
					Host:    "localhost",
					Port:    "8765",
					SiteId:  "def",
				},
			},
		},
		Addresses: map[string]Address{
			"happy": Address{
				Prefix:       "happy",
				Distribution: "multicast",
			},
			"dopey": Address{
				Prefix:       "dopey",
				Distribution: "closest",
			},
			"sneezy": Address{
				Prefix:       "sneezy",
				Distribution: "balanced",
			},
		},
	}
	data, err := MarshalRouterConfig(input)
	if err != nil {
		t.Errorf("Failed to marshal: %v", err)
	}
	output, err := UnmarshalRouterConfig(data)
	if err != nil {
		t.Errorf("Failed to unmarshal: %v", err)
	}
	if !reflect.DeepEqual(input.Metadata, output.Metadata) {
		t.Errorf("Incorrect metadata. Expected %#v got %#v", input.Metadata, output.Metadata)
	}
	if !reflect.DeepEqual(input.SslProfiles, output.SslProfiles) {
		t.Errorf("Incorrect sslprofiles. Expected %#v got %#v", input.SslProfiles, output.SslProfiles)
	}
	if !reflect.DeepEqual(input.Connectors, output.Connectors) {
		t.Errorf("Incorrect connectors. Expected %#v got %#v", input.Connectors, output.Connectors)
	}
	if !reflect.DeepEqual(input.Listeners, output.Listeners) {
		t.Errorf("Incorrect listeners. Expected %#v got %#v", input.Listeners, output.Listeners)
	}
	if !reflect.DeepEqual(input.Addresses, output.Addresses) {
		t.Errorf("Incorrect addresses. Expected %#v got %#v", input.Addresses, output.Addresses)
	}
	if !reflect.DeepEqual(input.Bridges, output.Bridges) {
		t.Errorf("Incorrect bridges. Expected %#v got %#v", input.Bridges, output.Bridges)
	}
}

func TestUnmarshalErrorInvalidJson(t *testing.T) {
	_, err := UnmarshalRouterConfig("{[foo=bar;;}")
	if err == nil {
		t.Errorf("Expected error for bad JSON")
	}
}

func TestUnmarshalErrorInvalidStructure(t *testing.T) {
	_, err := UnmarshalRouterConfig(`{"foo":"bar"}`)
	if err == nil {
		t.Errorf("Expected error for incorrect structure")
	}
}

func TestUnmarshalErrorInvalidStructure2(t *testing.T) {
	_, err := UnmarshalRouterConfig(`[{"foo":"bar"},{"baz":100}]`)
	if err == nil {
		t.Errorf("Expected error for incorrect structure")
	}
}

func TestUnmarshalErrorInvalidEntityType(t *testing.T) {
	_, err := UnmarshalRouterConfig(`[[100, {"foo":"bar"}],["whatsit", {"baz":100}]]`)
	if err == nil {
		t.Errorf("Expected error for non-string entity type")
	}
}

func TestUnmarshalErrorInvalidAddressValue(t *testing.T) {
	_, err := UnmarshalRouterConfig(`[["address", ["wrong"]]]`)
	if err == nil {
		t.Errorf("Expected error for invalid address value")
	}
}

func TestUnmarshalErrorInvalidSslProfileValue(t *testing.T) {
	_, err := UnmarshalRouterConfig(`[["sslProfile", ["wrong"]]]`)
	if err == nil {
		t.Errorf("Expected error for invalid sslprofile value")
	}
}

func TestUnmarshalErrorInvalidRouterValue(t *testing.T) {
	_, err := UnmarshalRouterConfig(`[["router", ["wrong"]]]`)
	if err == nil {
		t.Errorf("Expected error for invalid router value")
	}
}

func TestUnmarshalErrorInvalidConnectorValue(t *testing.T) {
	_, err := UnmarshalRouterConfig(`[["connector", ["wrong"]]]`)
	if err == nil {
		t.Errorf("Expected error for invalid connector value")
	}
}

func TestUnmarshalErrorInvalidListenerValue(t *testing.T) {
	_, err := UnmarshalRouterConfig(`[["listener", ["wrong"]]]`)
	if err == nil {
		t.Errorf("Expected error for invalid listener value")
	}
}

func TestUnmarshalErrorInvalidTcpConnectorValue(t *testing.T) {
	_, err := UnmarshalRouterConfig(`[["tcpConnector", ["wrong"]]]`)
	if err == nil {
		t.Errorf("Expected error for invalid tcpconnector value")
	}
}

func TestUnmarshalErrorInvalidTcpListenerValue(t *testing.T) {
	_, err := UnmarshalRouterConfig(`[["tcpListener", ["wrong"]]]`)
	if err == nil {
		t.Errorf("Expected error for invalid tcplistener value")
	}
}

func TestUnmarshalErrorInvalidHttpConnectorValue(t *testing.T) {
	_, err := UnmarshalRouterConfig(`[["httpConnector", ["wrong"]]]`)
	if err == nil {
		t.Errorf("Expected error for invalid httpconnector value")
	}
}

func TestUnmarshalErrorInvalidHttpListenerValue(t *testing.T) {
	_, err := UnmarshalRouterConfig(`[["httpListener", ["wrong"]]]`)
	if err == nil {
		t.Errorf("Expected error for invalid httplistener value")
	}
}

func TestUnmarshalErrorInvalidLogValue(t *testing.T) {
	_, err := UnmarshalRouterConfig(`[["log", ["wrong"]]]`)
	if err == nil {
		t.Errorf("Expected error for invalid log value")
	}
}

func checkLevel(t *testing.T, config *RouterConfig, mod string, level string) {
	entry, ok := config.LogConfig[mod]
	if ok && entry.Module != mod {
		t.Errorf("Inconsistent log config for %s. Expected %q got %q", mod, mod, entry.Module)
	}
	if entry.Enable != level {
		t.Errorf("Incorrect log level for %s. Expected %q got %q", mod, level, entry.Enable)
	}
}

func TestMarshalUnmarshalRouterConfigWithLogging(t *testing.T) {
	input := RouterConfig{}
	desired := map[string]string{
		"":         "debug",
		"ROUTER":   "info",
		"PROTOCOL": "trace+",
		"POLICY":   "notice",
	}
	changed := input.SetLogLevels(desired)
	if !changed {
		t.Errorf("Expected change to be indicated")
	}
	data, err := MarshalRouterConfig(input)
	if err != nil {
		t.Errorf("Failed to marshal: %v", err)
	}
	output, err := UnmarshalRouterConfig(data)
	if err != nil {
		t.Errorf("Failed to unmarshal: %v", err)
	}
	if !reflect.DeepEqual(input.Metadata, output.Metadata) {
		t.Errorf("Incorrect metadata. Expected %#v got %#v", input.Metadata, output.Metadata)
	}
	checkLevel(t, &input, "ROUTER", "info+")
	checkLevel(t, &input, "PROTOCOL", "trace+")
	checkLevel(t, &input, "POLICY", "notice+")
	checkLevel(t, &input, "DEFAULT", "debug+")

	if !reflect.DeepEqual(input.LogConfig, output.LogConfig) {
		t.Errorf("Incorrect log config. Expected %#v got %#v", input.LogConfig, output.LogConfig)
	}
	changed = input.SetLogLevels(desired)
	if changed {
		t.Errorf("Expected no change to be indicated")
	}
	changed = output.SetLogLevels(desired)
	if changed {
		t.Errorf("Expected no change to be indicated")
	}
	delete(desired, "POLICY")
	desired["TCP_ADAPTOR"] = "notice"
	changed = input.SetLogLevels(desired)
	if !changed {
		t.Errorf("Expected change to be indicated after altering config")
	}
	checkLevel(t, &input, "ROUTER", "info+")
	checkLevel(t, &input, "PROTOCOL", "trace+")
	checkLevel(t, &input, "POLICY", "")
	checkLevel(t, &input, "TCP_ADAPTOR", "notice+")
	checkLevel(t, &input, "DEFAULT", "debug+")
}

func TestFailedConvert(t *testing.T) {
	a := []string{"random"}
	b := SslProfile{}
	err := convert(a, b)
	if err == nil {
		t.Errorf("Expected error for invalid conversion")
	}
}

func TestGetSslProfilesDifference(t *testing.T) {
	before := BridgeConfig{

		HttpConnectors: map[string]HttpEndpoint{
			"c3": {
				Name:            "c3",
				Address:         "foo",
				Host:            "nowhere.com",
				Port:            "4321",
				SiteId:          "abc",
				ProtocolVersion: HttpVersion2,
				SslProfile:      types.ServiceClientSecret,
			},
			"c4": {
				Name:            "c4",
				Address:         "bar",
				Host:            "here.com",
				Port:            "8765",
				SiteId:          "def",
				ProtocolVersion: HttpVersion2,
				SslProfile:      types.ServiceClientSecret,
			},
		},
		HttpListeners: map[string]HttpEndpoint{
			"l3": {
				Name:            "l3",
				Address:         "green",
				Host:            "0.0.0.0",
				Port:            "4321",
				SiteId:          "abc",
				ProtocolVersion: HttpVersion2,
				SslProfile:      types.SkupperServiceCertPrefix + "green",
			},
			"l4": {
				Name:            "l4",
				Address:         "blue",
				Host:            "localhost",
				Port:            "8765",
				SiteId:          "def",
				ProtocolVersion: HttpVersion2,
				SslProfile:      types.SkupperServiceCertPrefix + "blue",
			},
		},
	}

	after := BridgeConfig{

		HttpConnectors: map[string]HttpEndpoint{
			"newConnector": {
				Name:            "newConnector",
				Address:         "new-connector",
				Host:            "nowhere.com",
				Port:            "4321",
				SiteId:          "abc",
				ProtocolVersion: HttpVersion2,
				SslProfile:      types.ServiceClientSecret,
			},
			"c4": {
				Name:            "c4",
				Address:         "bar",
				Host:            "here.com",
				Port:            "8765",
				SiteId:          "def",
				ProtocolVersion: HttpVersion2,
				SslProfile:      types.ServiceClientSecret,
			},
		},
		HttpListeners: map[string]HttpEndpoint{
			"l3": {
				Name:            "l3",
				Address:         "green",
				Host:            "0.0.0.0",
				Port:            "4321",
				SiteId:          "abc",
				ProtocolVersion: HttpVersion2,
				SslProfile:      types.SkupperServiceCertPrefix + "green",
			},
			"newListener": {
				Name:            "newListener",
				Address:         "new-listener",
				Host:            "localhost",
				Port:            "8765",
				SiteId:          "def",
				ProtocolVersion: HttpVersion2,
				SslProfile:      types.SkupperServiceCertPrefix + "new-listener",
			},
			"anotherNewListener": {
				Name:            "anotherNewListener",
				Address:         "another-new-listener",
				Host:            "localhost",
				Port:            "8765",
				SiteId:          "def",
				ProtocolVersion: HttpVersion2,
				SslProfile:      types.SkupperServiceCertPrefix + "another-new-listener",
			},
		},
	}

	addedSslProfiles, deletedSslProfiles := getSslProfilesDifference(&before, &after)

	expectedAddedSslProfiles := []string{types.SkupperServiceCertPrefix + "new-listener", types.SkupperServiceCertPrefix + "another-new-listener"}
	expectedDeletedSslProfiles := []string{types.SkupperServiceCertPrefix + "blue"}
	assert.Assert(t, utils.StringSlicesEqual(addedSslProfiles, expectedAddedSslProfiles), "Expected %v but got %v", expectedAddedSslProfiles, addedSslProfiles)
	assert.Assert(t, utils.StringSlicesEqual(deletedSslProfiles, expectedDeletedSslProfiles), "Expected %v but got %v", expectedDeletedSslProfiles, deletedSslProfiles)

}
