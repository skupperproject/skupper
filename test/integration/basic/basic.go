package basic

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"
)

type BasicTestRunner struct {
	base.ClusterTestRunnerBase
}

func (r *BasicTestRunner) RunTests(ctx context.Context) {

	pubCluster := r.GetPublicContext(1)
	prvCluster := r.GetPrivateContext(1)
	tick := time.Tick(constants.DefaultTick)
	timeout := time.After(constants.ImagePullingAndResourceCreationTimeout)
	wait_for_conn := func(cc *base.ClusterContext) {
		for {
			select {
			case <-ctx.Done():
				r.T.Logf("context has been canceled")
				r.T.FailNow()
			case <-timeout:
				assert.Assert(r.T, false, "Timeout waiting for connection")
			case <-tick:
				vir, err := cc.VanClient.RouterInspect(ctx)
				if err == nil && vir.Status.ConnectedSites.Total == 1 {
					r.T.Logf("Van sites connected!\n")
					return
				} else {
					fmt.Printf("Connection not ready yet, current pods state: \n")
					pubCluster.KubectlExec("get pods -o wide")
				}
			}
		}
	}
	wait_for_conn(pubCluster)
	wait_for_conn(prvCluster)
}

func (r *BasicTestRunner) Setup(ctx context.Context, createOptsPublic types.SiteConfigSpec, createOptsPrivate types.SiteConfigSpec) {
	var err error
	pub1Cluster := r.GetPublicContext(1)
	prv1Cluster := r.GetPrivateContext(1)
	err = pub1Cluster.CreateNamespace()
	assert.Assert(r.T, err)

	err = prv1Cluster.CreateNamespace()
	assert.Assert(r.T, err)

	createOptsPublic.SkupperNamespace = pub1Cluster.Namespace
	siteConfig, err := pub1Cluster.VanClient.SiteConfigCreate(context.Background(), createOptsPublic)
	assert.Assert(r.T, err)
	err = pub1Cluster.VanClient.RouterCreate(ctx, *siteConfig)
	assert.Assert(r.T, err)

	const secretFile = "/tmp/public_basic_1_secret.yaml"
	err = pub1Cluster.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, secretFile)
	assert.Assert(r.T, err)

	createOptsPrivate.SkupperNamespace = prv1Cluster.Namespace
	siteConfig, err = prv1Cluster.VanClient.SiteConfigCreate(context.Background(), createOptsPrivate)
	assert.Assert(r.T, err)
	err = prv1Cluster.VanClient.RouterCreate(ctx, *siteConfig)
	assert.Assert(r.T, err)

	var connectorCreateOpts types.ConnectorCreateOptions = types.ConnectorCreateOptions{
		SkupperNamespace: prv1Cluster.Namespace,
		Name:             "",
		Cost:             0,
	}
	_, err = prv1Cluster.VanClient.ConnectorCreateFromFile(ctx, secretFile, connectorCreateOpts)
	assert.Assert(r.T, err)
}

func (r *BasicTestRunner) TearDown(ctx context.Context) {
	r.GetPublicContext(1).DeleteNamespace()
	r.GetPrivateContext(1).DeleteNamespace()
}

func (r *BasicTestRunner) Run(ctx context.Context) {
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
				ClusterLocal:      true,
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
				ClusterLocal:      true,
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
				ClusterLocal:      false,
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
				ClusterLocal:      false,
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
				ClusterLocal:      true,
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
				ClusterLocal:      true,
				Replicas:          1,
			},
		},
	}

	defer r.TearDown(ctx)

	for _, c := range testcases {
		r.T.Logf("Testing: %s\n", c.doc)
		r.Setup(ctx, c.createOptsPublic, c.createOptsPrivate)
		r.RunTests(ctx)
		r.TearDown(ctx)
	}
}
