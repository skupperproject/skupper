//go:build integration || examples
// +build integration examples

package hipstershop

import (
	"fmt"
	"os"
	"testing"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"gotest.tools/assert"
)

const (
	JOBS_PER_NS = 5
)

var (
	jobCommand = []string{"/app/grpcclient_test", "-test.v"}
)

func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}

func TestHipsterShop(t *testing.T) {
	// Cluster needs for hipster shop
	needs := base.ClusterNeeds{
		NamespaceId:     "hipster",
		PublicClusters:  2,
		PrivateClusters: 1,
	}
	testRunner := &base.ClusterTestRunnerBase{}
	if err := testRunner.Validate(needs); err != nil {
		t.Skipf("%s", err)
	}
	_, err := testRunner.Build(needs, nil)
	assert.Assert(t, err)

	// removes namespaces and cancels context
	tearDownFn := func() {
		t.Logf("entering teardown")
		err := base.RemoveNamespacesForContexts(testRunner, []int{1, 2}, []int{1})
		if err != nil {
			t.Logf("Error removing namespaces = %s", err)
		}
		cancelFn()

		// removing temporary directories
		for _, dir := range cleanUpDirs {
			os.RemoveAll(dir)
		}
	}

	// defines an interrupt handler
	base.HandleInterruptSignal(tearDownFn)
	defer tearDownFn()

	// Prepare namespaces on provided clusters
	Setup(t, testRunner)

	// Creating VAN between the three clusters/namespaces
	CreateVAN(t, testRunner)

	// Deploy resources
	DeployResources(t, testRunner)

	// Exposing resources
	ExposeResources(t, testRunner)

	// Running the test job against all clusters/namespaces
	prv1, _ := testRunner.GetPrivateContext(1)
	pub1, _ := testRunner.GetPublicContext(1)
	pub2, _ := testRunner.GetPublicContext(2)
	t.Logf("Starting %d client gRPC jobs on each namespace", JOBS_PER_NS)
	for _, cluster := range []*base.ClusterContext{prv1, pub1, pub2} {
		for i := 1; i <= JOBS_PER_NS; i++ {
			jobName := fmt.Sprintf("grpcclient-%d", i)
			t.Logf("Running gRPC client job %s on %s", jobName, cluster.Namespace)
			_, err := k8s.CreateTestJob(cluster.Namespace, cluster.VanClient.KubeClient, jobName, jobCommand)
			assert.Assert(t, err)
		}
	}
	for _, cluster := range []*base.ClusterContext{prv1, pub1, pub2} {
		t.Run(cluster.Namespace, func(t *testing.T) {
			for i := 1; i <= JOBS_PER_NS; i++ {
				jobName := fmt.Sprintf("grpcclient-%d", i)
				t.Logf("Waiting on gRPC client job %s to finish on %s", jobName, cluster.Namespace)
				job, err := k8s.WaitForJob(cluster.Namespace, cluster.VanClient.KubeClient, jobName, constants.ImagePullingAndResourceCreationTimeout)
				jobSucceeded := job.Status.Succeeded == 1
				if err != nil || !jobSucceeded {
					// retrieving job logs
					log, err := k8s.GetJobLogs(cluster.Namespace, cluster.VanClient.KubeClient, job.Name)
					assert.Assert(t, err)
					t.Logf("Job %s has failed. Job log:", job.Name)
					t.Logf(log)
				}
				if err != nil {
					testRunner.DumpTestInfo(cluster.Namespace)
				}
				assert.Assert(t, err)
				assert.Assert(t, jobSucceeded)
			}
		})
	}
}
