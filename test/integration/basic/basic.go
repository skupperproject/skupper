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

	tick := time.Tick(5 * time.Second)
	wait_for_conn := func(cc *cluster.ClusterContext) {
		timeout := time.After(120 * time.Second)
		for {
			select {
			case <-timeout:
				assert.Assert(r.T, false, "Timeout waiting for connection")
			case <-tick:
				vir, err := cc.VanClient.VanRouterInspect(ctx)
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

func (r *BasicTestRunner) Setup(ctx context.Context, createOptsPublic types.VanSiteConfig, createOptsPrivate types.VanSiteConfig) {
	var err error
	err = r.Pub1Cluster.CreateNamespace()
	assert.Assert(r.T, err)

	err = r.Priv1Cluster.CreateNamespace()
	assert.Assert(r.T, err)

	createOptsPublic.Spec.SkupperNamespace = r.Pub1Cluster.CurrentNamespace
	r.Pub1Cluster.VanClient.VanRouterCreate(ctx, createOptsPublic)

	err = r.Pub1Cluster.VanClient.VanConnectorTokenCreateFile(ctx, types.DefaultVanName, "/tmp/public_secret.yaml")
	assert.Assert(r.T, err)

	createOptsPrivate.Spec.SkupperNamespace = r.Priv1Cluster.CurrentNamespace
	err = r.Priv1Cluster.VanClient.VanRouterCreate(ctx, createOptsPrivate)

	var vanConnectorCreateOpts types.VanConnectorCreateOptions = types.VanConnectorCreateOptions{
		SkupperNamespace: r.Priv1Cluster.CurrentNamespace,
		Name:             "",
		Cost:             0,
	}
	r.Priv1Cluster.VanClient.VanConnectorCreateFromFile(ctx, "/tmp/public_secret.yaml", vanConnectorCreateOpts)
}

func (r *BasicTestRunner) TearDown(ctx context.Context) {
	r.Pub1Cluster.DeleteNamespaces()
	r.Priv1Cluster.DeleteNamespaces()
}

func (r *BasicTestRunner) Run(ctx context.Context) {
	testcases := []struct {
		doc               string
		createOptsPublic  types.VanSiteConfig
		createOptsPrivate types.VanSiteConfig
	}{
		{
			doc: "Connecting, two internals, clusterLocal=true",
			createOptsPublic: types.VanSiteConfig{
				Spec: types.VanSiteConfigSpec{
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
			createOptsPrivate: types.VanSiteConfig{
				Spec: types.VanSiteConfigSpec{
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
			createOptsPublic: types.VanSiteConfig{
				Spec: types.VanSiteConfigSpec{
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
			createOptsPrivate: types.VanSiteConfig{
				Spec: types.VanSiteConfigSpec{
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
			createOptsPublic: types.VanSiteConfig{
				Spec: types.VanSiteConfigSpec{
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
			createOptsPrivate: types.VanSiteConfig{
				Spec: types.VanSiteConfigSpec{
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
