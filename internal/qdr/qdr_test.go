package qdr

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/utils"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInitialConfig(t *testing.T) {
	config := InitialConfig("foo", "bar", "1.2.3", true, 10)
	if config.Metadata.Id != "foo" {
		t.Errorf("Invalid id, expected 'foo' got %q", config.Metadata.Id)
	}
	if GetSiteMetadata(config.Metadata.Metadata).Id != "bar" {
		t.Errorf("Invalid metadata, expected id to be 'bar' got %q", GetSiteMetadata(config.Metadata.Metadata).Id)
	}
	if GetSiteMetadata(config.Metadata.Metadata).Version != "1.2.3" {
		t.Errorf("Invalid metadata, expected version to be '1.2.3' got %q", GetSiteMetadata(config.Metadata.Metadata).Version)
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
	if GetSiteMetadata(config.Metadata.Metadata).Id != "bong" {
		t.Errorf("Invalid metadata, expectedsite id to be 'bong' got %q", GetSiteMetadata(config.Metadata.Metadata).Id)
	}
	if GetSiteMetadata(config.Metadata.Metadata).Version != "3.2.1" {
		t.Errorf("Invalid metadata, expected version to be '3.2.1' got %q", GetSiteMetadata(config.Metadata.Metadata).Version)
	}
	if config.Metadata.Mode != ModeInterior {
		t.Errorf("Invalid id, expected %q got %q", ModeInterior, config.Metadata.Mode)
	}
	if config.Metadata.HelloMaxAgeSeconds != "5" {
		t.Errorf("Invalid id, expected %q got %q", "10", config.Metadata.HelloMaxAgeSeconds)
	}
}

func TestAddRemoveListener(t *testing.T) {
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
	result := config.AddListener(Listener{
		Name: "l1",
		Port: 5672,
	})
	if result == true {
		t.Errorf("Expect add to fail, duplicate listener")
	}
	if len(config.Listeners) != 2 {
		t.Errorf("Expected two listeners but found %d", len(config.Listeners))
	}
	result, _ = config.RemoveListener("l1")
	if result == false {
		t.Errorf("Expected to remove existing listener")
	}
	result, _ = config.RemoveListener("bad")
	if result == true {
		t.Errorf("Expected to not find listener")
	}
	if len(config.Listeners) != 1 {
		t.Errorf("Expected 1 listener but found %d", len(config.Listeners))
	}
}

func TestAddRemoveConnector(t *testing.T) {
	config := InitialConfig("foo", "bar", "undefined", true, 3)
	config.AddConnector(Connector{
		Name: "c1",
		Role: RoleInterRouter,
		Port: "5672",
		Cost: 1,
	})
	if config.Connectors["c1"].Cost != 1 {
		t.Errorf("Expected cost 1 but got %d", config.Connectors["c1"].Cost)
	}
	config.AddConnector(Connector{
		Name: "c2",
		Host: "127.0.0.1",
		Port: "8888",
		Role: RoleEdge,
	})
	if config.Connectors["c2"].Port != "8888" {
		t.Errorf("Expected port 8888 but got %s", config.Connectors["c2"].Port)
	}
	result := config.AddConnector(Connector{
		Name: "c1",
		Role: RoleInterRouter,
		Port: "5672",
		Cost: 1,
	})
	if result == true {
		t.Errorf("Expect add to fail, duplicate Connector")
	}
	if len(config.Connectors) != 2 {
		t.Errorf("Expected two Connectors but found %d", len(config.Connectors))
	}
	result, _ = config.RemoveConnector("c1")
	if result == false {
		t.Errorf("Expected to remove existing Connector")
	}
	result, _ = config.RemoveConnector("bad")
	if result == true {
		t.Errorf("Expected to not find Connector")
	}
	if len(config.Connectors) != 1 {
		t.Errorf("Expected 1 Connector but found %d", len(config.Connectors))
	}
}

func TestAddSslProfile(t *testing.T) {
	config := InitialConfig("foo", "bar", "undefined", true, 3)
	config.AddSslProfile(SslProfile{
		Name:     "myprofile",
		CertFile: "/my/certs/cert.pem",
	})
	assert.Equal(t, config.SslProfiles["myprofile"].CertFile, "/my/certs/cert.pem")

	config.AddSslProfile(ConfigureSslProfile("another", "/etc/skupper-router-certs", true))
	assert.Equal(t, config.SslProfiles["another"].CaCertFile, "/etc/skupper-router-certs/another/ca.crt")
	assert.Equal(t, config.SslProfiles["another"].CertFile, "/etc/skupper-router-certs/another/tls.crt")
	assert.Equal(t, config.SslProfiles["another"].PrivateKeyFile, "/etc/skupper-router-certs/another/tls.key")

	config.AddSslProfile(ConfigureSslProfile("third", "/foo/bar/", false))
	assert.Equal(t, config.SslProfiles["third"].CaCertFile, "/foo/bar/third/ca.crt")
	assert.Equal(t, config.SslProfiles["third"].CertFile, "")
	assert.Equal(t, config.SslProfiles["third"].PrivateKeyFile, "")
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
		SiteConfig: &SiteConfig{
			Name:      "razzle",
			Namespace: "dazzle",
			Location:  "pizzazz",
			Provider:  "azure",
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
	if !reflect.DeepEqual(input.SiteConfig, output.SiteConfig) {
		t.Errorf("Incorrect siteconfig. Expected %#v got %#v", input.SiteConfig, output.SiteConfig)
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

func TestUnmarshalErrorInvalidLogValue(t *testing.T) {
	_, err := UnmarshalRouterConfig(`[["log", ["wrong"]]]`)
	if err == nil {
		t.Errorf("Expected error for invalid log value")
	}
}

func checkLevel(t *testing.T, config *RouterConfig, mod string, level string) {
	t.Helper()
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

func TestUpdateConfigMap(t *testing.T) {
	routerConfig := RouterConfig{}
	data, _ := routerConfig.AsConfigMapData()

	siteConfig := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: types.SiteConfigMapName,
		},
		Data: data,
	}

	updated, err := routerConfig.UpdateConfigMap(siteConfig)
	if !updated {
		t.Errorf("Expect routerconfig to be updated")
	}

	if err != nil {
		t.Errorf("Failed updating routerconfig")
	}
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

		TcpConnectors: map[string]TcpEndpoint{
			"c3": {
				Name:       "c3",
				Address:    "foo",
				Host:       "nowhere.com",
				Port:       "4321",
				SiteId:     "abc",
				SslProfile: types.ServiceClientSecret,
			},
			"c4": {
				Name:       "c4",
				Address:    "bar",
				Host:       "here.com",
				Port:       "8765",
				SiteId:     "def",
				SslProfile: types.ServiceClientSecret,
			},
		},
		TcpListeners: map[string]TcpEndpoint{
			"l3": {
				Name:       "l3",
				Address:    "green",
				Host:       "0.0.0.0",
				Port:       "4321",
				SiteId:     "abc",
				SslProfile: types.SkupperServiceCertPrefix + "green",
			},
			"l4": {
				Name:       "l4",
				Address:    "blue",
				Host:       "localhost",
				Port:       "8765",
				SiteId:     "def",
				SslProfile: types.SkupperServiceCertPrefix + "blue",
			},
		},
	}

	after := BridgeConfig{

		TcpConnectors: map[string]TcpEndpoint{
			"newConnector": {
				Name:       "newConnector",
				Address:    "new-connector",
				Host:       "nowhere.com",
				Port:       "4321",
				SiteId:     "abc",
				SslProfile: types.ServiceClientSecret,
			},
			"c4": {
				Name:       "c4",
				Address:    "bar",
				Host:       "here.com",
				Port:       "8765",
				SiteId:     "def",
				SslProfile: types.ServiceClientSecret,
			},
			"newTcpConnector": {
				Name:       "newTcpConnector",
				Address:    "new-tcp-connector",
				Host:       "abc.io",
				Port:       "4321",
				SiteId:     "abc",
				SslProfile: types.ServiceClientSecret,
			},
		},
		TcpListeners: map[string]TcpEndpoint{
			"l3": {
				Name:       "l3",
				Address:    "green",
				Host:       "0.0.0.0",
				Port:       "4321",
				SiteId:     "abc",
				SslProfile: types.SkupperServiceCertPrefix + "green",
			},
			"newListener": {
				Name:       "newListener",
				Address:    "new-listener",
				Host:       "localhost",
				Port:       "8765",
				SiteId:     "def",
				SslProfile: types.SkupperServiceCertPrefix + "new-listener",
			},
			"anotherNewListener": {
				Name:       "anotherNewListener",
				Address:    "another-new-listener",
				Host:       "localhost",
				Port:       "8765",
				SiteId:     "def",
				SslProfile: types.SkupperServiceCertPrefix + "another-new-listener",
			},
			"newTcpListener": {
				Name:       "newTCPListener",
				Address:    "new-tcp-listener",
				Host:       "localhost",
				Port:       "8765",
				SiteId:     "def",
				SslProfile: types.SkupperServiceCertPrefix + "the-database",
			},
		},
	}

	addedSslProfiles, deletedSslProfiles := getSslProfilesDifference(&before, &after)

	expectedAddedSslProfiles := []string{types.SkupperServiceCertPrefix + "new-listener", types.SkupperServiceCertPrefix + "another-new-listener", types.SkupperServiceCertPrefix + "the-database"}
	expectedDeletedSslProfiles := []string{types.SkupperServiceCertPrefix + "blue"}
	assert.Assert(t, utils.StringSlicesEqual(addedSslProfiles, expectedAddedSslProfiles), "Expected %v but got %v", expectedAddedSslProfiles, addedSslProfiles)
	assert.Assert(t, utils.StringSlicesEqual(deletedSslProfiles, expectedDeletedSslProfiles), "Expected %v but got %v", expectedDeletedSslProfiles, deletedSslProfiles)

}

func TestSiteConfig(t *testing.T) {
	config := InitialConfig("foo", "bar", "1.2.3", true, 10)
	if config.SiteConfig != nil {
		t.Error("expected no site configuration by default")
	}

	data, err := MarshalRouterConfig(config)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var entities [][]json.RawMessage
	if err := json.Unmarshal([]byte(data), &entities); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	for _, entity := range entities {
		if len(entity) > 1 {
			var entityType string
			if err := json.Unmarshal(entity[0], &entityType); err != nil {
				t.Fatalf("Failed to unmarshal entity type: %v", err)
			}
			if entityType == "site" {
				t.Errorf("expected no site config entity but got %q", string(entity[1]))
			}
		}
	}
}

func TestRecordTypes_GH2081(t *testing.T) {
	// assert Records do not contain floating point values
	testCases := []recordType{
		TcpEndpoint{
			Name: "backend",
		},
		Connector{
			Name:         "cnctr",
			Cost:         10,
			LinkCapacity: 256,
		},
		Listener{
			Name:         "cnctr",
			Cost:         10,
			Port:         9931,
			LinkCapacity: 256,
		},
		SslProfile{
			Name: "myprofile",
		},
	}
	for _, rt := range testCases {
		t.Run("", func(t *testing.T) {
			record := rt.toRecord()
			for key, val := range record {
				isFloat := false
				switch val.(type) {
				case float64:
					isFloat = true
				case float32:
					isFloat = true
				}

				if isFloat {
					t.Errorf("Record field %q contains floating point number", key)
				}
			}
		})
	}
}

func TestTcpEndpointObserverToRecord(t *testing.T) {
	// empty observer -> omitted
	e := TcpEndpoint{
		Name:     "t1",
		Address:  "a",
		Host:     "h",
		Port:     "1234",
		SiteId:   "s",
		Observer: "",
	}
	r := e.toRecord()
	_, ok := r["observer"]
	assert.Assert(t, !ok, "observer should be omitted when empty")

	// non-empty observer -> included
	for _, val := range []string{"auto", "none"} {
		e.Observer = val
		r = e.toRecord()
		got, ok := r["observer"]
		assert.Assert(t, ok, "observer should be present when set")
		assert.Equal(t, got, val)
	}
}

func TestTcpEndpointEquivalentObserverAutoVsEmpty(t *testing.T) {
	a := TcpEndpoint{
		Name:     "t1",
		Address:  "a",
		Host:     "",
		Port:     "1234",
		SiteId:   "s",
		Observer: "auto",
	}
	b := TcpEndpoint{
		Name:     "t1",
		Address:  "a",
		Host:     "",
		Port:     "1234",
		SiteId:   "s",
		Observer: "",
	}
	if !a.Equivalent(b) {
		t.Errorf("expected endpoints to be equivalent when comparing observer 'auto' vs empty")
	}
}
