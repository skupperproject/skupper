package tcp_echo

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/pkg/kube"

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

var serviceNs = types.ServiceInterface{
	Address:  "tcp-go-echo-ns",
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
	pub1Cluster, _ := r.GetPublicContext(1)

	defer tearDown(ctx, t, r, pub1Cluster.Namespace)
	setup(ctx, t, pub1Cluster, service, pub1Cluster.Namespace)
	runTests(t, r, service)
}

func RunForNamespace(ctx context.Context, t *testing.T, r base.ClusterTestRunner, namespace string) {
	pub1Cluster, _ := r.GetPublicContext(1)
	_, err := kube.NewNamespace(namespace, pub1Cluster.VanClient.KubeClient)
	assert.Assert(t, err)

	defer kube.DeleteNamespace(namespace, pub1Cluster.VanClient.KubeClient)
	defer tearDown(ctx, t, r, namespace)
	setup(ctx, t, pub1Cluster, serviceNs, namespace)
	runTests(t, r, serviceNs)
}

func tearDown(ctx context.Context, t *testing.T, r base.ClusterTestRunner, namespace string) {
	pub1Cluster, _ := r.GetPublicContext(1)

	t.Logf("Deleting skupper service")
	_ = pub1Cluster.VanClient.ServiceInterfaceRemove(ctx, service.Address)

	t.Logf("Deleting deployment...")
	_ = pub1Cluster.VanClient.KubeClient.AppsV1().Deployments(namespace).Delete(ctx, Deployment.Name, &metav1.DeleteOptions{})
}

func setup(ctx context.Context, t *testing.T, cluster *base.ClusterContext, svc types.ServiceInterface, namespace string) {
	publicDeploymentsClient := cluster.VanClient.KubeClient.AppsV1().Deployments(namespace)

	fmt.Println("Creating deployment...")
	result, err := publicDeploymentsClient.Create(ctx, Deployment, metav1.CreateOptions{})
	assert.Assert(t, err)

	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())

	fmt.Printf("Listing deployments in namespace %q:\n", namespace)
	list, err := publicDeploymentsClient.List(ctx, metav1.ListOptions{})
	assert.Assert(t, err)

	for _, d := range list.Items {
		fmt.Printf(" * %s (%d replicas)\n", d.Name, *d.Spec.Replicas)
	}

	err = cluster.VanClient.ServiceInterfaceCreate(ctx, &svc)
	assert.Assert(t, err)

	err = cluster.VanClient.ServiceInterfaceBind(ctx, &svc, "deployment", "tcp-go-echo", map[int]int{}, namespace)
	assert.Assert(t, err)
}

func runTests(t *testing.T, r base.ClusterTestRunner, svc types.ServiceInterface) {

	// XXX
	endTime := time.Now().Add(constants.ImagePullingAndResourceCreationTimeout)

	pub1Cluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, svc.Address)
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, svc.Address)
	assert.Assert(t, err)

	jobName := svc.Address
	jobCmd := []string{"/app/tcp_echo_test", "-test.run", "Job"}

	// Note here we are executing the same test but, in two different
	// namespaces (or clusters), the same service must exist in both clusters
	// because of the skupper connections and the "skupper exposed"
	// deployment.
	_, err = k8s.CreateTestJobWithEnv(
		pub1Cluster.Namespace,
		pub1Cluster.VanClient.KubeClient,
		jobName,
		jobCmd,
		[]apiv1.EnvVar{
			{Name: "ADDRESS", Value: fmt.Sprintf("%s:%d", svc.Address, svc.Ports[0])},
		})
	assert.Assert(t, err)

	_, err = k8s.CreateTestJobWithEnv(
		prv1Cluster.Namespace,
		prv1Cluster.VanClient.KubeClient,
		jobName,
		jobCmd,
		[]apiv1.EnvVar{
			{Name: "ADDRESS", Value: fmt.Sprintf("%s:%d", svc.Address, svc.Ports[0])},
		})
	assert.Assert(t, err)

	endTime = time.Now().Add(constants.ImagePullingAndResourceCreationTimeout)

	job, err := k8s.WaitForJob(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, jobName, endTime.Sub(time.Now()))
	assert.Assert(t, err)
	pub1Cluster.KubectlExec("logs job/" + jobName)
	k8s.AssertJob(t, job)

	job, err = k8s.WaitForJob(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, jobName, endTime.Sub(time.Now()))
	assert.Assert(t, err)
	prv1Cluster.KubectlExec("logs job/" + jobName)
	k8s.AssertJob(t, job)

	netcatJobName := fmt.Sprintf("netcat-%s", svc.Address)
	// Running netcat
	for _, cluster := range []*base.ClusterContext{pub1Cluster, prv1Cluster} {
		t.Logf("Running netcat job against %s", cluster.Namespace)
		ncJob := k8s.NewJob(netcatJobName, cluster.Namespace, k8s.JobOpts{
			Image:   "quay.io/prometheus/busybox",
			Restart: apiv1.RestartPolicyNever,
			Labels:  map[string]string{"job": netcatJobName},
			Command: []string{"sh"},
			Args:    []string{"-c", fmt.Sprintf("echo Halo | nc %s %d", svc.Address, svc.Ports[0])},
		})
		// Asserting job has been created
		_, err := cluster.VanClient.KubeClient.BatchV1().Jobs(cluster.Namespace).Create(context.TODO(), ncJob, metav1.CreateOptions{})
		assert.Assert(t, err)
		// Asserting job completed
		_, jobErr := k8s.WaitForJob(cluster.Namespace, cluster.VanClient.KubeClient, ncJob.Name, time.Minute)
		// Asserting job output
		logs, err := k8s.GetJobLogs(cluster.Namespace, cluster.VanClient.KubeClient, ncJob.Name)
		if jobErr != nil || err != nil {
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
