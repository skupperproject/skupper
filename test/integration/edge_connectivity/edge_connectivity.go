package edgecon

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

var fp = fmt.Fprintf

type TestCase struct {
	name               string
	diagram            []string
	createOptsPublic   types.SiteConfigSpec
	createOptsPrivate  types.SiteConfigSpec
	public_public_cnx  map[int]int
	private_public_cnx []int
	direct_count       int
	indirect_count     int
}

type EdgeConnectivityTestRunner struct {
	base.ClusterTestRunnerBase
}

func (r *EdgeConnectivityTestRunner) RunTests(testCase *TestCase, ctx context.Context, t *testing.T) (err error) {

	pubCluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prvCluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	tick := time.Tick(constants.DefaultTick)
	timeout := time.After(constants.ImagePullingAndResourceCreationTimeout)
	wait_for_conn := func(cc *base.ClusterContext, countConnections bool) (err error) {
		for {
			select {
			case <-ctx.Done():
				t.Logf("context has been canceled")
				t.FailNow()
			case <-timeout:
				assert.Assert(t, false, "Timeout waiting for connection")
			case <-tick:
				vir, err := cc.VanClient.RouterInspect(ctx)
				if err == nil && vir.Status.ConnectedSites.Total >= 1 {
					t.Logf("Van sites connected!\n")
					if err != nil {
						return err
					}
					if !countConnections {
						return nil
					}
					if testCase.direct_count == vir.Status.ConnectedSites.Direct &&
						testCase.indirect_count == vir.Status.ConnectedSites.Indirect {
						return nil
					}
				} else {
					fmt.Printf("Connection not ready yet, current pods state: \n")
					pubCluster.KubectlExec("get pods -o wide")
				}
			}
		}
	}
	err = wait_for_conn(pubCluster, false)
	if err != nil {
		return err
	}

	err = wait_for_conn(prvCluster, true)
	if err != nil {
		return err
	}

	return nil
}

func (r *EdgeConnectivityTestRunner) Setup(ctx context.Context, testCase *TestCase, t *testing.T) {

	publicSecrets := make(map[int]string, 0)

	// Make Public namespaces -------------------------------------------
	createOptsPublic := testCase.createOptsPublic
	for i := 0; i < int(createOptsPublic.Replicas); i++ {
		pub1Cluster, err := r.GetPublicContext(i + 1) // These numbers are 1-based.
		assert.Assert(t, err)

		err = pub1Cluster.CreateNamespace()
		assert.Assert(t, err)

		// Create and configure the cluster.
		createOptsPublic.SkupperNamespace = pub1Cluster.Namespace
		siteConfig, err := pub1Cluster.VanClient.SiteConfigCreate(context.Background(), createOptsPublic)
		assert.Assert(t, err)

		// Create the router.
		err = pub1Cluster.VanClient.RouterCreate(ctx, *siteConfig)
		assert.Assert(t, err)

		// Create a connection token for this cluster.
		// It is only the public clusters that get connected to.
		// We do this for every public cluster because we are too lazy
		// to figure out which ones will actually need it.
		secretFileName := fmt.Sprintf("/tmp/public_edgecon_%d_secret.yaml", i+1)
		err = pub1Cluster.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, secretFileName)
		assert.Assert(t, err)
		publicSecrets[i] = secretFileName
	}

	// Make Private namespace -------------------------------------------
	// In this test there is always a single private namespace,
	// and it is always an edge.
	privateCluster, err := r.GetPrivateContext(1) // There is always only 1 private/edge namespace.
	assert.Assert(t, err)

	err = privateCluster.CreateNamespace()
	assert.Assert(t, err)

	testCase.createOptsPrivate.SkupperNamespace = privateCluster.Namespace
	siteConfig, err := privateCluster.VanClient.SiteConfigCreate(context.Background(), testCase.createOptsPrivate)
	assert.Assert(t, err)
	err = privateCluster.VanClient.RouterCreate(ctx, *siteConfig)
	assert.Assert(t, err)

	// Make all public-to-public connections. --------------------------
	for public_1, public_2 := range testCase.public_public_cnx {
		secretFileName := publicSecrets[public_2-1]
		public_1_cluster, err := r.GetPublicContext(public_1)
		assert.Assert(t, err)
		connectorCreateOpts := types.ConnectorCreateOptions{SkupperNamespace: public_1_cluster.Namespace,
			Name: "",
			Cost: 0,
		}
		_, err = public_1_cluster.VanClient.ConnectorCreateFromFile(ctx, secretFileName, connectorCreateOpts)
		assert.Assert(t, err)
	}

	// Make all private-to-public connections. -------------------------
	for _, public := range testCase.private_public_cnx {
		secretFileName := publicSecrets[public-1]
		privateCluster, err := r.GetPrivateContext(1) // There can be only one.
		assert.Assert(t, err)
		connectorCreateOpts := types.ConnectorCreateOptions{SkupperNamespace: privateCluster.Namespace,
			Name: "",
			Cost: 0,
		}
		_, err = privateCluster.VanClient.ConnectorCreateFromFile(ctx, secretFileName, connectorCreateOpts)
		assert.Assert(t, err)
	}
}

func (r *EdgeConnectivityTestRunner) TearDown(ctx context.Context, testcase *TestCase) {

	createOptsPublic := testcase.createOptsPublic
	for i := 0; i < int(createOptsPublic.Replicas); i++ {
		pub, err := r.GetPublicContext(i + 1)
		if err != nil {
			log.Warn(err.Error())
		}
		pub.DeleteNamespace()
	}

	priv, err := r.GetPrivateContext(1) // There can be only one.
	if err != nil {
		log.Warn(err.Error())
	}
	priv.DeleteNamespace()
}

func (r *EdgeConnectivityTestRunner) Run(ctx context.Context, testcase *TestCase, t *testing.T) {

	r.Setup(ctx, testcase, t)
	err := r.RunTests(testcase, ctx, t)
	r.TearDown(ctx, testcase) // pass in testcase as arg, get rid of current_testcase global.
	assert.Assert(t, err)
}
