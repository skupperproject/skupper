package qdr

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/qdr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestConfigMapCompressionWriteAndRead(t *testing.T) {
	routerConfig := testRouterConfig()
	marshalled, err := qdr.MarshalRouterConfig(routerConfig)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		writer     ConfigMapWriter
		compressed bool
	}{
		{"disabled", ConfigMapWriter{}, false},
		{"below threshold", ConfigMapWriter{CompressionThreshold: len(marshalled) + 1}, false},
		{"at threshold", ConfigMapWriter{CompressionThreshold: len(marshalled)}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cm := &corev1.ConfigMap{}
			if err := test.writer.WriteConfigMap(&routerConfig, cm); err != nil {
				t.Fatal(err)
			}
			_, compressed := cm.BinaryData[types.TransportConfigFileCompressed]
			if compressed != test.compressed {
				t.Errorf("expected compressed=%v", test.compressed)
			}
			_, plain := cm.Data[types.TransportConfigFile]
			if plain == test.compressed {
				t.Errorf("expected plain=%v", !test.compressed)
			}
			actual, err := GetRouterConfigFromConfigMap(cm)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(*actual, routerConfig) {
				t.Errorf("retrieved config does not match original: %#v != %#v", *actual, routerConfig)
			}
		})
	}
}

func TestConfigMapCompressionFormatTransitions(t *testing.T) {
	cm := &corev1.ConfigMap{}
	writer := ConfigMapWriter{CompressionThreshold: 1000}

	if err := writer.writeConfigMapData(strings.Repeat("x", 999), cm); err != nil {
		t.Fatal(err)
	}
	if len(cm.BinaryData[types.TransportConfigFileCompressed]) != 0 {
		t.Error("expected config below threshold to be stored plain")
	}

	if err := writer.writeConfigMapData(strings.Repeat("x", 1000), cm); err != nil {
		t.Fatal(err)
	}
	if len(cm.BinaryData[types.TransportConfigFileCompressed]) == 0 {
		t.Fatal("expected config at threshold to be stored compressed")
	}
	if _, ok := cm.Data[types.TransportConfigFile]; ok {
		t.Error("expected plain config to be removed when compressed")
	}

	if err := writer.writeConfigMapData(strings.Repeat("x", 999), cm); err != nil {
		t.Fatal(err)
	}
	if _, ok := cm.BinaryData[types.TransportConfigFileCompressed]; ok {
		t.Error("expected config below threshold to revert to plain")
	}
}

func TestGetRouterConfigDataCorruptCompressed(t *testing.T) {
	cm := &corev1.ConfigMap{
		Data: map[string]string{
			types.TransportConfigFile: "stale plain config",
		},
		BinaryData: map[string][]byte{
			types.TransportConfigFileCompressed: []byte("not gzip"),
		},
	}
	if _, err := GetRouterConfigData(cm); err == nil {
		t.Error("expected corrupt compressed config to return an error, not fall back to plain data")
	}
	cm.BinaryData[types.TransportConfigFileCompressed] = nil
	if _, err := GetRouterConfigData(cm); err == nil {
		t.Error("expected empty compressed config to return an error, not fall back to plain data")
	}
}

func TestUpdateRouterConfigNormalizesRepresentation(t *testing.T) {
	ctx := context.Background()
	routerConfig := testRouterConfig()
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "skupper-router", Namespace: "test"}}
	if err := (ConfigMapWriter{CompressionThreshold: 1}).WriteConfigMap(&routerConfig, cm); err != nil {
		t.Fatal(err)
	}
	client := k8sfake.NewSimpleClientset(cm)

	if err := UpdateRouterConfig(client, cm.Name, cm.Namespace, ctx, noConfigUpdate{}, nil, ConfigMapWriter{}); err != nil {
		t.Fatal(err)
	}
	actual, err := client.CoreV1().ConfigMaps(cm.Namespace).Get(ctx, cm.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := actual.Data[types.TransportConfigFile]; !ok {
		t.Error("expected compressed config to be normalized to plain when compression is disabled")
	}
	if _, ok := actual.BinaryData[types.TransportConfigFileCompressed]; ok {
		t.Error("expected compressed config to be removed during normalization")
	}
}

type noConfigUpdate struct{}

func (noConfigUpdate) Apply(*qdr.RouterConfig) bool {
	return false
}

func testRouterConfig() qdr.RouterConfig {
	routerConfig := qdr.InitialConfig("foo", "bar", "1.2.3", false, 10)
	routerConfig.AddListener(qdr.Listener{Name: "l1", Host: "0.0.0.0", Port: 1234})
	routerConfig.AddConnector(qdr.Connector{Name: "c1", Host: "somewhere.com", Port: "4321"})
	return routerConfig
}
