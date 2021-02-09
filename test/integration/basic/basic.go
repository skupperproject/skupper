package basic

import (
	"context"
	"testing"

	"github.com/prometheus/common/log"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/utils/base"
	"gotest.tools/assert"
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

func (r *BasicTestRunner) Setup(ctx context.Context, createOptsPublic types.SiteConfigSpec, createOptsPrivate types.SiteConfigSpec, t *testing.T) {
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
	err = pub1Cluster.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, secretFile)
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
		doc               string
		createOptsPublic  types.SiteConfigSpec
		createOptsPrivate types.SiteConfigSpec
	}{
		{
			doc: "Connecting, two internals, clusterLocal=true",
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            false,
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
				IsEdge:            false,
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
			doc: "Connecting, two internals, clusterLocal=false",
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            false,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           pubCluster.VanClient.GetIngressRouteIfPossibleLoadBalancerIfNot(),
				Replicas:          1,
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            false,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				Ingress:           pubCluster.VanClient.GetIngressRouteIfPossibleLoadBalancerIfNot(),
				Replicas:          1,
			},
		},
		{
			doc: "connecting, Private Edge, Public Internal, clusterLocal=true",
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            false,
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
				IsEdge:            true,
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
	}

	defer r.TearDown(ctx)

	for _, c := range testcases {
		t.Logf("Testing: %s\n", c.doc)
		r.Setup(ctx, c.createOptsPublic, c.createOptsPrivate, t)
		r.RunTests(ctx, t)
		r.TearDown(ctx)
	}
}
