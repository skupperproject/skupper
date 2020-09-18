package basic

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/log"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"

	"github.com/skupperproject/skupper/api/types"
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

	tick := time.Tick(constants.DefaultTick)
	timeout := time.After(constants.ImagePullingAndResourceCreationTimeout)
	wait_for_conn := func(cc *base.ClusterContext) {
		for {
			select {
			case <-ctx.Done():
				t.Logf("context has been canceled")
				t.FailNow()
			case <-timeout:
				assert.Assert(t, false, "Timeout waiting for connection")
			case <-tick:
				vir, err := cc.VanClient.RouterInspect(ctx)
				if err == nil && vir.Status.ConnectedSites.Total == 1 {
					t.Logf("Van sites connected!\n")
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

func (r *BasicTestRunner) Setup(ctx context.Context, createOptsPublic types.SiteConfig, createOptsPrivate types.SiteConfig, t *testing.T) {
	var err error
	pub1Cluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	err = pub1Cluster.CreateNamespace()
	assert.Assert(t, err)

	err = prv1Cluster.CreateNamespace()
	assert.Assert(t, err)

	createOptsPublic.Spec.SkupperNamespace = pub1Cluster.Namespace
	err = pub1Cluster.VanClient.RouterCreate(ctx, createOptsPublic)
	assert.Assert(t, err)

	const secretFile = "/tmp/public_basic_1_secret.yaml"
	err = pub1Cluster.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, secretFile)
	assert.Assert(t, err)

	createOptsPrivate.Spec.SkupperNamespace = prv1Cluster.Namespace
	err = prv1Cluster.VanClient.RouterCreate(ctx, createOptsPrivate)
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
	testcases := []struct {
		doc               string
		createOptsPublic  types.SiteConfig
		createOptsPrivate types.SiteConfig
	}{
		{
			doc: "Connecting, two internals, clusterLocal=true",
			createOptsPublic: types.SiteConfig{
				Spec: types.SiteConfigSpec{
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
			createOptsPrivate: types.SiteConfig{
				Spec: types.SiteConfigSpec{
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
		},
		{
			doc: "Connecting, two internals, clusterLocal=false",
			createOptsPublic: types.SiteConfig{
				Spec: types.SiteConfigSpec{
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
			createOptsPrivate: types.SiteConfig{
				Spec: types.SiteConfigSpec{
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
		},
		{
			doc: "connecting, Private Edge, Public Internal, clusterLocal=true",
			createOptsPublic: types.SiteConfig{
				Spec: types.SiteConfigSpec{
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
			createOptsPrivate: types.SiteConfig{
				Spec: types.SiteConfigSpec{
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
