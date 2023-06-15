package client

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"gotest.tools/assert"
)

func TestSiteConfigRoundtrip(t *testing.T) {
	testcases := []struct {
		input    types.SiteConfigSpec
		expected types.SiteConfigSpec
	}{
		{
			expected: types.SiteConfigSpec{
				SkupperName:      "site-config-roundtrip-1",
				SkupperNamespace: "site-config-roundtrip-1",
				Ingress:          "loadbalancer",
				RouterMode:       "interior",
				AuthMode:         "internal",
				Annotations:      map[string]string{},
				Labels:           map[string]string{},
				Router:           types.RouterOptions{Logging: []types.RouterLogConfig{{Module: "ROUTER_CORE", Level: "error+"}}},
				FlowCollector:    types.FlowCollectorOptions{FlowRecordTtl: types.DefaultFlowTimeoutDuration},
				PrometheusServer: types.PrometheusServerOptions{AuthMode: "tls"},
			},
		},
		{
			input: types.SiteConfigSpec{
				SkupperName: "foo",
				Ingress:     "none",
				RouterMode:  "edge",
				AuthMode:    "none",
			},
			expected: types.SiteConfigSpec{
				SkupperName:      "foo",
				SkupperNamespace: "site-config-roundtrip-2",
				Ingress:          "none",
				RouterMode:       "edge",
				AuthMode:         "none",
				Annotations:      map[string]string{},
				Labels:           map[string]string{},
				Router:           types.RouterOptions{Logging: []types.RouterLogConfig{{Module: "ROUTER_CORE", Level: "error+"}}},
				FlowCollector:    types.FlowCollectorOptions{FlowRecordTtl: types.DefaultFlowTimeoutDuration},
				PrometheusServer: types.PrometheusServerOptions{AuthMode: "tls"},
			},
		},
		{
			input: types.SiteConfigSpec{
				SkupperName:    "bar",
				Ingress:        "none",
				ConsoleIngress: "loadbalancer",
				User:           "squirrel",
				Password:       "secret",
			},
			expected: types.SiteConfigSpec{
				SkupperName:      "bar",
				SkupperNamespace: "site-config-roundtrip-3",
				Ingress:          "none",
				ConsoleIngress:   "loadbalancer",
				RouterMode:       "interior",
				AuthMode:         "internal",
				User:             "squirrel",
				Password:         "secret",
				Annotations:      map[string]string{},
				Labels:           map[string]string{},
				Router:           types.RouterOptions{Logging: []types.RouterLogConfig{{Module: "ROUTER_CORE", Level: "error+"}}},
				FlowCollector:    types.FlowCollectorOptions{FlowRecordTtl: types.DefaultFlowTimeoutDuration},
				PrometheusServer: types.PrometheusServerOptions{AuthMode: "tls"},
			},
		},
		{
			input: types.SiteConfigSpec{
				Ingress: "none",
				Router: types.RouterOptions{
					Tuning: types.Tuning{
						Cpu:          "1",
						Memory:       "2G",
						Affinity:     "app.kubernetes.io/name=foo",
						AntiAffinity: "app.kubernetes.io/name=bar",
						NodeSelector: "kubernetes.io/hostname=node1",
					},
					Logging: []types.RouterLogConfig{
						{
							Level: "trace",
						},
					},
					DebugMode:        "gdb",
					MaxFrameSize:     1111,
					MaxSessionFrames: 2222,
				},
			},
			expected: types.SiteConfigSpec{
				SkupperName:      "site-config-roundtrip-4",
				SkupperNamespace: "site-config-roundtrip-4",
				Ingress:          "none",
				RouterMode:       "interior",
				AuthMode:         "internal",
				Annotations:      map[string]string{},
				Labels:           map[string]string{},
				Router: types.RouterOptions{
					Tuning: types.Tuning{
						Cpu:          "1",
						Memory:       "2G",
						Affinity:     "app.kubernetes.io/name=foo",
						AntiAffinity: "app.kubernetes.io/name=bar",
						NodeSelector: "kubernetes.io/hostname=node1",
					},
					Logging: []types.RouterLogConfig{
						{
							Level: "trace",
						},
					},
					DebugMode:        "gdb",
					MaxFrameSize:     1111,
					MaxSessionFrames: 2222,
				},
				FlowCollector:    types.FlowCollectorOptions{FlowRecordTtl: types.DefaultFlowTimeoutDuration},
				PrometheusServer: types.PrometheusServerOptions{AuthMode: "tls"},
			},
		},
		{
			input: types.SiteConfigSpec{
				Ingress: "none",
				Controller: types.ControllerOptions{
					Tuning: types.Tuning{
						Cpu:          "100m",
						Memory:       "3M",
						Affinity:     "app.kubernetes.io/name=apple",
						AntiAffinity: "app.kubernetes.io/name=pear",
						NodeSelector: "kubernetes.io/hostname=nodeX",
					},
				},
			},
			expected: types.SiteConfigSpec{
				SkupperName:      "site-config-roundtrip-5",
				SkupperNamespace: "site-config-roundtrip-5",
				Ingress:          "none",
				RouterMode:       "interior",
				AuthMode:         "internal",
				Annotations:      map[string]string{},
				Labels:           map[string]string{},
				Router:           types.RouterOptions{Logging: []types.RouterLogConfig{{Module: "ROUTER_CORE", Level: "error+"}}},
				Controller: types.ControllerOptions{
					Tuning: types.Tuning{
						Cpu:          "100m",
						Memory:       "3M",
						Affinity:     "app.kubernetes.io/name=apple",
						AntiAffinity: "app.kubernetes.io/name=pear",
						NodeSelector: "kubernetes.io/hostname=nodeX",
					},
				},
				FlowCollector:    types.FlowCollectorOptions{FlowRecordTtl: types.DefaultFlowTimeoutDuration},
				PrometheusServer: types.PrometheusServerOptions{AuthMode: "tls"},
			},
		},
		{
			input: types.SiteConfigSpec{
				Ingress: "nodeport",
				Router: types.RouterOptions{
					IngressHost: "foo.com",
				},
			},
			expected: types.SiteConfigSpec{
				SkupperName:      "site-config-roundtrip-6",
				SkupperNamespace: "site-config-roundtrip-6",
				Ingress:          "nodeport",
				RouterMode:       "interior",
				AuthMode:         "internal",
				Annotations:      map[string]string{},
				Labels:           map[string]string{},
				Router: types.RouterOptions{
					IngressHost: "foo.com",
					Logging:     []types.RouterLogConfig{{Module: "ROUTER_CORE", Level: "error+"}},
				},
				FlowCollector:    types.FlowCollectorOptions{FlowRecordTtl: types.DefaultFlowTimeoutDuration},
				PrometheusServer: types.PrometheusServerOptions{AuthMode: "tls"},
			},
		},
		{
			input: types.SiteConfigSpec{
				RunAsUser:  1000,
				RunAsGroup: 2000,
			},
			expected: types.SiteConfigSpec{
				RunAsUser:        1000,
				RunAsGroup:       2000,
				SkupperName:      "site-config-roundtrip-7",
				SkupperNamespace: "site-config-roundtrip-7",
				Ingress:          "loadbalancer",
				RouterMode:       "interior",
				AuthMode:         "internal",
				Annotations:      map[string]string{},
				Labels:           map[string]string{},
				Router:           types.RouterOptions{Logging: []types.RouterLogConfig{{Module: "ROUTER_CORE", Level: "error+"}}},
				FlowCollector:    types.FlowCollectorOptions{FlowRecordTtl: types.DefaultFlowTimeoutDuration},
				PrometheusServer: types.PrometheusServerOptions{AuthMode: "tls"},
			},
		},
		{
			input: types.SiteConfigSpec{
				Ingress: "none",
				FlowCollector: types.FlowCollectorOptions{
					Tuning: types.Tuning{
						Cpu:    "1",
						Memory: "2G",
					},
					FlowRecordTtl: time.Minute * 30,
				},
				PrometheusServer: types.PrometheusServerOptions{
					Tuning: types.Tuning{
						Cpu:    "1",
						Memory: "2G",
					},
				},
			},
			expected: types.SiteConfigSpec{
				SkupperName:      "site-config-roundtrip-8",
				SkupperNamespace: "site-config-roundtrip-8",
				Ingress:          "none",
				RouterMode:       "interior",
				AuthMode:         "internal",
				Annotations:      map[string]string{},
				Labels:           map[string]string{},
				FlowCollector: types.FlowCollectorOptions{
					Tuning: types.Tuning{
						Cpu:    "1",
						Memory: "2G",
					},
					FlowRecordTtl: time.Minute * 30,
				},
				Router: types.RouterOptions{Logging: []types.RouterLogConfig{{Module: "ROUTER_CORE", Level: "error+"}}},
				PrometheusServer: types.PrometheusServerOptions{
					Tuning: types.Tuning{
						Cpu:    "1",
						Memory: "2G",
					},
					AuthMode: "tls",
				},
			},
		},
	}

	isCluster := *clusterRun

	for i, c := range testcases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		namespace := fmt.Sprintf("site-config-roundtrip-%d", i+1)
		var cli *VanClient
		var err error
		if !isCluster {
			cli, err = newMockClient(namespace, "", "")
		} else {
			cli, err = NewClient(namespace, "", "")
		}
		assert.Check(t, err, namespace)

		_, err = kube.NewNamespace(namespace, cli.KubeClient)
		assert.Check(t, err, namespace)
		defer kube.DeleteNamespace(namespace, cli.KubeClient)

		_, err = cli.SiteConfigCreate(ctx, c.input)
		assert.Check(t, err, namespace)

		config, err := cli.SiteConfigInspect(ctx, nil)
		assert.Check(t, err, namespace)

		if diff := cmp.Diff(c.expected, config.Spec); diff != "" {
			t.Errorf("TestSiteConfigRoundtrip %d config not as expected (-want +got):\n%s", (i + 1), diff)
		}
	}
}
