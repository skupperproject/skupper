package tcp_echo

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"

	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"

	appsv1 "k8s.io/api/apps/v1"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func int32Ptr(i int32) *int32 { return &i }

var service = types.ServiceInterface{
	Address:  "tcp-go-echo",
	Protocol: "tcp",
	Ports:    []int{9090},
}

var Deployment *appsv1.Deployment = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "tcp-go-echo",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "tcp-go-echo"},
		},
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "tcp-go-echo",
				},
			},
			Spec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name:            "tcp-go-echo",
						Image:           "quay.io/skupper/tcp-go-echo",
						ImagePullPolicy: apiv1.PullIfNotPresent,
						Ports: []apiv1.ContainerPort{
							{
								Name:          "http",
								Protocol:      apiv1.ProtocolTCP,
								ContainerPort: 80,
							},
						},
					},
				},
			},
		},
	},
}

func Run(ctx context.Context, t *testing.T, r base.ClusterTestRunner) {
	defer tearDown(ctx, t, r)
	setup(ctx, t, r)
	runTests(t, r)
}

func tearDown(ctx context.Context, t *testing.T, r base.ClusterTestRunner) {
	pub1Cluster, _ := r.GetPublicContext(1)

	t.Logf("Deleting skupper service")
	_ = pub1Cluster.VanClient.ServiceInterfaceRemove(ctx, service.Address)

	t.Logf("Deleting deployment...")
	_ = pub1Cluster.VanClient.KubeClient.AppsV1().Deployments(pub1Cluster.Namespace).Delete(Deployment.Name, &metav1.DeleteOptions{})
}

func setup(ctx context.Context, t *testing.T, r base.ClusterTestRunner) {
	pub1Cluster, _ := r.GetPublicContext(1)
	publicDeploymentsClient := pub1Cluster.VanClient.KubeClient.AppsV1().Deployments(pub1Cluster.Namespace)

	fmt.Println("Creating deployment...")
	result, err := publicDeploymentsClient.Create(Deployment)
	assert.Assert(t, err)

	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())

	fmt.Printf("Listing deployments in namespace %q:\n", pub1Cluster.Namespace)
	list, err := publicDeploymentsClient.List(metav1.ListOptions{})
	assert.Assert(t, err)

	for _, d := range list.Items {
		fmt.Printf(" * %s (%d replicas)\n", d.Name, *d.Spec.Replicas)
	}

	err = pub1Cluster.VanClient.ServiceInterfaceCreate(ctx, &service)
	assert.Assert(t, err)

	err = pub1Cluster.VanClient.ServiceInterfaceBind(ctx, &service, "deployment", "tcp-go-echo", "tcp", map[int]int{})
	assert.Assert(t, err)
}

func runTests(t *testing.T, r base.ClusterTestRunner) {

	// XXX
	endTime := time.Now().Add(constants.ImagePullingAndResourceCreationTimeout)

	pub1Cluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, "tcp-go-echo")
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, "tcp-go-echo")
	assert.Assert(t, err)

	jobName := "tcp-echo"
	jobCmd := []string{"/app/tcp_echo_test", "-test.run", "Job"}

	// Note here we are executing the same test but, in two different
	// namespaces (or clusters), the same service must exist in both clusters
	// because of the skupper connections and the "skupper exposed"
	// deployment.
	_, err = k8s.CreateTestJob(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, jobName, jobCmd)
	assert.Assert(t, err)

	_, err = k8s.CreateTestJob(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, jobName, jobCmd)
	assert.Assert(t, err)

	endTime = time.Now().Add(constants.ImagePullingAndResourceCreationTimeout)

	rb := r.(*base.ClusterTestRunnerBase)
	job, err := k8s.WaitForJob(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, jobName, endTime.Sub(time.Now()))
	if err != nil {
		rb.DumpTestInfo(jobName)
	}
	assert.Assert(t, err)
	pub1Cluster.KubectlExec("logs job/" + jobName)
	k8s.AssertJob(t, job)

	job, err = k8s.WaitForJob(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, jobName, endTime.Sub(time.Now()))
	if err != nil {
		rb.DumpTestInfo(jobName)
	}
	assert.Assert(t, err)
	prv1Cluster.KubectlExec("logs job/" + jobName)
	k8s.AssertJob(t, job)

	// Running netcat
	for _, cluster := range []*base.ClusterContext{pub1Cluster, prv1Cluster} {
		t.Logf("Running netcat job against %s", cluster.Namespace)
		ncJob := k8s.NewJob("netcat", cluster.Namespace, k8s.JobOpts{
			Image:   "quay.io/prometheus/busybox",
			Restart: apiv1.RestartPolicyNever,
			Labels:  map[string]string{"job": "netcat"},
			Command: []string{"sh"},
			Args:    []string{"-c", "echo Halo | nc tcp-go-echo 9090"},
		})
		// Asserting job has been created
		_, err := cluster.VanClient.KubeClient.BatchV1().Jobs(cluster.Namespace).Create(ncJob)
		assert.Assert(t, err)
		// Asserting job completed
		_, jobErr := k8s.WaitForJob(cluster.Namespace, cluster.VanClient.KubeClient, ncJob.Name, time.Minute)
		// Asserting job output
		logs, err := k8s.GetJobLogs(cluster.Namespace, cluster.VanClient.KubeClient, ncJob.Name)
		if jobErr != nil || err != nil {
			rb.DumpTestInfo(ncJob.Name)
			log.Printf("%s job output: %s", ncJob.Name, logs)
		}
		assert.Assert(t, jobErr)
		assert.Assert(t, err)
		assert.Assert(t, strings.Contains(logs, "HALO"), "invalid response - %s", logs)
	}
}

func SendReceive(addr string) error {
	doneCh := make(chan error)
	go func(doneCh chan error) {

		strEcho := "Halo"
		log.Println("Resolving TCP Address")
		tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			doneCh <- fmt.Errorf("ResolveTCPAddr failed: %s\n", err.Error())
			return
		}

		log.Println("Opening TCP connection")
		conn, err := net.DialTCP("tcp", nil, tcpAddr)
		if err != nil {
			doneCh <- fmt.Errorf("Dial failed: %s\n", err.Error())
			return
		}
		defer conn.Close()

		log.Println("Sending data")
		_, err = conn.Write([]byte(strEcho))
		if err != nil {
			doneCh <- fmt.Errorf("Write to server failed: %s\n", err.Error())
			return
		}

		log.Println("Receiving reply")
		reply := make([]byte, 1024)

		_, err = conn.Read(reply)
		if err != nil {
			doneCh <- fmt.Errorf("Read from server failed: %s\n", err.Error())
			return
		}

		log.Println("Sent to server = ", strEcho)
		log.Println("Reply from server = ", string(reply))

		if !strings.Contains(string(reply), strings.ToUpper(strEcho)) {
			doneCh <- fmt.Errorf("Response from server different that expected: %s\n", string(reply))
			return
		}

		doneCh <- nil
	}(doneCh)
	timeoutCh := time.After(time.Minute)

	// TCP Echo Client job sometimes hangs waiting for response
	// This will cause job to fail and a retry to occur
	var err error
	select {
	case err = <-doneCh:
	case <-timeoutCh:
		err = fmt.Errorf("timed out waiting for tcp-echo job to finish")
	}

	return err
}
