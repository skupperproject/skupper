package bookinfo

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"

	"gotest.tools/assert"
)

func RunTests(ctx context.Context, t *testing.T, r *base.ClusterTestRunnerBase) {
	pub1Cluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	fmt.Printf("Running tests!!!\n")

	jobName := "bookinfo"
	jobCmd := []string{"/app/bookinfo_test", "-test.run", "Job"}

	_, err = k8s.CreateTestJob(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, jobName, jobCmd)
	assert.Assert(t, err)

	job, err := k8s.WaitForJob(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, jobName, constants.ImagePullingAndResourceCreationTimeout)
	if err != nil {
		pub1Cluster.KubectlExec("logs job/" + jobName)
		r.DumpTestInfo(jobName)
	}
	assert.Assert(t, err)
	k8s.AssertJob(t, job)
}

func TearDown(ctx context.Context, t *testing.T, r *base.ClusterTestRunnerBase) {
	pub1Cluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	_, err = pub1Cluster.KubectlExec("delete -f https://raw.githubusercontent.com/skupperproject/skupper-example-bookinfo/master/public-cloud.yaml")
	assert.Assert(t, err)

	_, err = prv1Cluster.KubectlExec("delete -f https://raw.githubusercontent.com/skupperproject/skupper-example-bookinfo/master/private-cloud.yaml")
	assert.Assert(t, err)
}

func Setup(ctx context.Context, t *testing.T, r *base.ClusterTestRunnerBase) {
	pub1Cluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	_, err = pub1Cluster.KubectlExec("apply -f https://raw.githubusercontent.com/skupperproject/skupper-example-bookinfo/master/public-cloud.yaml")
	assert.Assert(t, err)

	_, err = prv1Cluster.KubectlExec("apply -f https://raw.githubusercontent.com/skupperproject/skupper-example-bookinfo/master/private-cloud.yaml")
	assert.Assert(t, err)

	pub1Cluster.KubectlExec("annotate service ratings skupper.io/proxy=http")
	prv1Cluster.KubectlExec("annotate service details skupper.io/proxy=http")
	prv1Cluster.KubectlExec("annotate service reviews skupper.io/proxy=http")

	var wg sync.WaitGroup
	waitFor := func(cc *base.ClusterContext, serviceName string, err error) {
		defer wg.Done()
		_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(cc.Namespace, cc.VanClient.KubeClient, serviceName)
	}

	var detailsError, reviewsError, ratingsError error
	wg.Add(3)
	go waitFor(pub1Cluster, "details", detailsError)
	go waitFor(pub1Cluster, "reviews", reviewsError)
	go waitFor(prv1Cluster, "ratings", ratingsError)
	wg.Wait()

	assert.Assert(t, detailsError)
	assert.Assert(t, reviewsError)
	assert.Assert(t, ratingsError)
}

func Run(ctx context.Context, t *testing.T, r *base.ClusterTestRunnerBase) {
	Setup(ctx, t, r)
	RunTests(ctx, t, r)
}
