package mongodb

import (
	"context"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	vanClient "github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"gotest.tools/assert"
)

func int32Ptr(i int32) *int32 { return &i }

func RunTests(ctx context.Context, t *testing.T, r *base.ClusterTestRunnerBase) {
	pubCluster1, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prvCluster1, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, "mongo-a")
	assert.Assert(t, err)
	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(prvCluster1.Namespace, prvCluster1.VanClient.KubeClient, "mongo-b")
	assert.Assert(t, err)

	_, err = prvCluster1.KubectlExec(`exec deploy/mongo-a -- mongo --host 127.0.0.1:27017 --eval 'rs.initiate({ _id : "rs0", members: [ { _id: 0, host: "mongo-a:27017" }, { _id: 1, host: "mongo-b:27017" }]})'`)
	assert.Assert(t, err)

	jobName := "mongo"
	jobCmd := []string{"/app/mongo_test", "-test.run", "Job"}

	_, err = k8s.CreateTestJob(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, jobName, jobCmd)
	assert.Assert(t, err)
	job, err := k8s.WaitForJob(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, jobName, constants.ImagePullingAndResourceCreationTimeout)
	jobLogs, _ := k8s.GetJobsLogs(pubCluster1.Namespace, pubCluster1.VanClient.KubeClient, jobName, true)
	t.Logf("%s logs:", jobName)
	t.Logf(jobLogs)
	if err != nil {
		r.DumpTestInfo(jobName)
	}
	assert.Assert(t, err)

	k8s.AssertJob(t, job)
}

func Setup(ctx context.Context, t *testing.T, r *base.ClusterTestRunnerBase) {
	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	pub1Cluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	_, err = prv1Cluster.KubectlExec("apply -f https://raw.githubusercontent.com/skupperproject/skupper-example-mongodb-replica-set/master/deployment-mongo-a.yaml")
	assert.Assert(t, err)

	_, err = pub1Cluster.KubectlExec("apply -f https://raw.githubusercontent.com/skupperproject/skupper-example-mongodb-replica-set/master/deployment-mongo-b.yaml")
	assert.Assert(t, err)

	expose := func(name string, cli *vanClient.VanClient) {
		t.Helper()
		service := types.ServiceInterface{
			Address:  name,
			Protocol: "tcp",
			Ports:    []int{27017},
		}

		err = cli.ServiceInterfaceCreate(ctx, &service)
		assert.Assert(t, err)

		err = cli.ServiceInterfaceBind(ctx, &service, "deployment", name, "tcp", map[int]int{})
		assert.Assert(t, err)

	}
	expose("mongo-a", prv1Cluster.VanClient)
	expose("mongo-b", pub1Cluster.VanClient)
	// Just need to do load orr rs inittiate, the easiest one.

}

func Run(ctx context.Context, t *testing.T, r *base.ClusterTestRunnerBase) {
	Setup(ctx, t, r)
	RunTests(ctx, t, r)
}
