package basic

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/prometheus/common/log"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/env"
	"gotest.tools/assert"
)

const (
	SkipReasonIngressNone           = "this test only runs against a single cluster (ingress=none)"
	SkipReasonNodePortNoIngressHost = "this test can only be executed if PUBLIC_1_INGRESS_HOST environment varialbe is set (ingress=nodeport)"
)

type BasicTestRunner struct {
	base.ClusterTestRunnerBase
}

func (r *BasicTestRunner) RunTests(ctx context.Context, t *testing.T) {

	pubCluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prvCluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	assert.Assert(t, base.WaitForSkupperConnectedSites(ctx, pubCluster, 1))
	assert.Assert(t, base.WaitForSkupperConnectedSites(ctx, prvCluster, 1))
}

func (r *BasicTestRunner) Setup(ctx context.Context, createOptsPublic types.SiteConfigSpec, createOptsPrivate types.SiteConfigSpec, tokenType string, t *testing.T) {
	var err error
	pub1Cluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	err = pub1Cluster.CreateNamespace()
	assert.Assert(t, err)

	err = prv1Cluster.CreateNamespace()
	assert.Assert(t, err)

	createOptsPublic.SkupperNamespace = pub1Cluster.Namespace
	siteConfig, err := pub1Cluster.VanClient.SiteConfigCreate(context.Background(), createOptsPublic)
	assert.Assert(t, err)
	err = pub1Cluster.VanClient.RouterCreate(ctx, *siteConfig)
	assert.Assert(t, err)

	const secretFile = "/tmp/public_basic_1_secret.yaml"
	if tokenType == "claim" {
		err = pub1Cluster.VanClient.TokenClaimCreateFile(ctx, types.DefaultVanName, []byte(createOptsPublic.Password), 15*time.Minute, 1, secretFile)
	} else {
		err = pub1Cluster.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, secretFile)
	}
	assert.Assert(t, err)

	createOptsPrivate.SkupperNamespace = prv1Cluster.Namespace
	siteConfig, err = prv1Cluster.VanClient.SiteConfigCreate(context.Background(), createOptsPrivate)
	assert.Assert(t, err)
	err = prv1Cluster.VanClient.RouterCreate(ctx, *siteConfig)
	assert.Assert(t, err)

	var connectorCreateOpts types.ConnectorCreateOptions = types.ConnectorCreateOptions{
		SkupperNamespace: prv1Cluster.Namespace,
		Name:             "",
		Cost:             0,
	}
	_, err = prv1Cluster.VanClient.ConnectorCreateFromFile(ctx, secretFile, connectorCreateOpts)
	assert.Assert(t, err)
}

func (r *BasicTestRunner) TearDown(ctx context.Context) {
	errMsg := "Something failed! aborting teardown"

	pub, err := r.GetPublicContext(1)
	if err != nil {
		log.Warn(errMsg)
	}

	priv, err := r.GetPrivateContext(1)
	if err != nil {
		log.Warn(errMsg)
	}

	pub.DeleteNamespace()
	priv.DeleteNamespace()
}

func (r *BasicTestRunner) Run(ctx context.Context, t *testing.T) {
	pubCluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	testcases := []struct {
		id                string
		doc               string
		skip              bool
		skipReason        string
		tokenType         string
		createOptsPublic  types.SiteConfigSpec
		createOptsPrivate types.SiteConfigSpec
	}{
		{
			id:         "interiors-ingress-none",
			doc:        "Connecting two interiors with ingress=none",
			skip:       base.MultipleClusters(),
			skipReason: SkipReasonIngressNone,
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           types.IngressNoneString,
				Replicas:          1,
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           types.IngressNoneString,
				Replicas:          1,
			},
		},
		{
			id:  "interiors-ingress-default",
			doc: "Connecting two interiors with ingress=default (route if available or loadbalancer)",
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           pubCluster.VanClient.GetIngressDefault(),
				Replicas:          1,
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           pubCluster.VanClient.GetIngressDefault(),
				Replicas:          1,
			},
		},
		{
			id:         "edge-interior-ingress-none",
			doc:        "Connecting a private edge to a public interior with ingress=none",
			skip:       base.MultipleClusters(),
			skipReason: SkipReasonIngressNone,
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           types.IngressNoneString,
				Replicas:          1,
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeEdge),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           types.IngressNoneString,
				Replicas:          1,
			},
		},
		{
			id:         "interiors-ingress-nodeport",
			doc:        "Connecting two interiors with ingress=nodeport",
			skip:       os.Getenv(env.Public1IngressHost) == "",
			skipReason: SkipReasonNodePortNoIngressHost,
			tokenType:  "claim",
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     true,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           types.IngressNodePortString,
				Router: types.RouterOptions{
					IngressHost: os.Getenv(env.Public1IngressHost),
				},
				Controller: types.ControllerOptions{
					IngressHost: os.Getenv(env.Public1IngressHost),
				},
				Replicas: 1,
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				RouterMode:        string(types.TransportModeInterior),
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           pubCluster.VanClient.GetIngressDefault(),
				Replicas:          1,
			},
		},
	}

	defer r.TearDown(ctx)

	for _, c := range testcases {
		t.Run(c.id, func(t *testing.T) {
			if c.skip {
				t.Skipf("Skipping: %s [%s]\n", c.doc, c.skipReason)
			}
			t.Logf("Testing: %s\n", c.doc)
			r.Setup(ctx, c.createOptsPublic, c.createOptsPrivate, c.tokenType, t)
			r.RunTests(ctx, t)
			r.TearDown(ctx)
		})
	}
}
