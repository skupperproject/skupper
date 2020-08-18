package basic

import (
	"context"
	"fmt"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/cluster"
	"gotest.tools/assert"
)

type BasicTestRunner struct {
	cluster.ClusterTestRunnerBase
}

func (r *BasicTestRunner) RunTests(ctx context.Context) {

	tick := time.Tick(cluster.DefaultTick)
	timeout := time.After(cluster.ImagePullingAndResourceCreationTimeout)
	wait_for_conn := func(cc *cluster.ClusterContext) {
		for {
			select {
			case <-timeout:
				assert.Assert(r.T, false, "Timeout waiting for connection")
			case <-tick:
				vir, err := cc.VanClient.RouterInspect(ctx)
				if err == nil && vir.Status.ConnectedSites.Total == 1 {
					r.T.Logf("Van sites connected!\n")
					return
				} else {
					fmt.Printf("Connection not ready yet, current pods state: \n")
					r.Pub1Cluster.KubectlExec("get pods -o wide")
				}
			}
		}
	}
	wait_for_conn(r.Pub1Cluster)
	wait_for_conn(r.Priv1Cluster)
}

func (r *BasicTestRunner) Setup(ctx context.Context, createOptsPublic types.SiteConfig, createOptsPrivate types.SiteConfig) {
	var err error
	err = r.Pub1Cluster.CreateNamespace()
	assert.Assert(r.T, err)

	err = r.Priv1Cluster.CreateNamespace()
	assert.Assert(r.T, err)

	createOptsPublic.Spec.SkupperNamespace = r.Pub1Cluster.CurrentNamespace
	err = r.Pub1Cluster.VanClient.RouterCreate(ctx, createOptsPublic)
	assert.Assert(r.T, err)

	err = r.Pub1Cluster.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, "/tmp/public_secret.yaml")
	assert.Assert(r.T, err)

	createOptsPrivate.Spec.SkupperNamespace = r.Priv1Cluster.CurrentNamespace
	err = r.Priv1Cluster.VanClient.RouterCreate(ctx, createOptsPrivate)
	assert.Assert(r.T, err)

	var connectorCreateOpts types.ConnectorCreateOptions = types.ConnectorCreateOptions{
		SkupperNamespace: r.Priv1Cluster.CurrentNamespace,
		Name:             "",
		Cost:             0,
	}
	_, err = r.Priv1Cluster.VanClient.ConnectorCreateFromFile(ctx, "/tmp/public_secret.yaml", connectorCreateOpts)
	assert.Assert(r.T, err)
}

func (r *BasicTestRunner) TearDown(ctx context.Context) {
	r.Pub1Cluster.DeleteNamespaces()
	r.Priv1Cluster.DeleteNamespaces()
}

func (r *BasicTestRunner) Run(ctx context.Context) {
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
		r.T.Logf("Testing: %s\n", c.doc)
		r.Setup(ctx, c.createOptsPublic, c.createOptsPrivate)
		r.RunTests(ctx)
	}
}
