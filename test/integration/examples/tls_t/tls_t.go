package tls_t

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
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
	Address:        "ssl-server",
	Protocol:       "tcp",
	Ports:          []int{8443},
	EnableTls:      true,
	TlsCredentials: "skupper-tls-ssl-server",
}

var Deployment *appsv1.Deployment = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "ssl-server",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"application": "ssl-server"},
		},
		Template: apiv1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application": "ssl-server",
				},
			},
			Spec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name:            "ssl-server",
						Image:           k8s.GetTestImage(),
						ImagePullPolicy: apiv1.PullIfNotPresent,
						Args: []string{
							"sh", "-c",
							"openssl s_server " +
								"-port 8443 " +
								"-cert /cert/tls.crt " +
								"-key /cert/tls.key " +
								"-rev"},
						Ports: []apiv1.ContainerPort{
							{
								Protocol:      apiv1.ProtocolTCP,
								ContainerPort: 8443,
							},
						},
						VolumeMounts: []apiv1.VolumeMount{
							{
								Name:      "cert",
								MountPath: "/cert",
							},
						},
					},
				},
				Volumes: []apiv1.Volume{
					{
						Name: "cert",
						VolumeSource: apiv1.VolumeSource{
							Secret: &apiv1.SecretVolumeSource{
								SecretName: "skupper-tls-ssl-server",
								//DefaultMode: 0420,
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

	// We need to create the service interface before the deployment, because
	// the deployment needs to mount the secret created by the service
	err := pub1Cluster.VanClient.ServiceInterfaceCreate(ctx, &service)
	assert.Assert(t, err)

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

	err = pub1Cluster.VanClient.ServiceInterfaceBind(ctx, &service, "deployment", "ssl-server", "tcp", map[int]int{})
	assert.Assert(t, err)
}

func runTests(t *testing.T, r base.ClusterTestRunner) {

	endTime := time.Now().Add(constants.ImagePullingAndResourceCreationTimeout)

	pub1Cluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prv1Cluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, "ssl-server")
	assert.Assert(t, err)

	_, err = k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, "ssl-server")
	assert.Assert(t, err)

	jobName := "ssl-client"
	jobCmd := []string{"/app/tls_test", "-test.run", "TestTlsJob"}

	// Note here we are executing the same test but, in two different
	// namespaces (or clusters), the same service must exist in both clusters
	// because of the skupper connections and the "skupper exposed"
	// deployment.
	_, err = k8s.CreateTestJobWithSecret(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, jobName, jobCmd, "skupper-tls-ssl-server")
	assert.Assert(t, err)

	_, err = k8s.CreateTestJobWithSecret(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, jobName, jobCmd, "skupper-tls-ssl-server")
	assert.Assert(t, err)

	endTime = time.Now().Add(constants.ImagePullingAndResourceCreationTimeout)

	rb := r.(*base.ClusterTestRunnerBase)
	job, err := k8s.WaitForJob(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, jobName, endTime.Sub(time.Now()))
	if err != nil || job.Status.Succeeded != 1 {
		rb.DumpTestInfo(jobName)
		logs, _ := k8s.GetJobsLogs(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, jobName, true)
		log.Printf("%s job output: %s", jobName, logs)
	}
	assert.Assert(t, err)
	pub1Cluster.KubectlExec("logs job/" + jobName)
	k8s.AssertJob(t, job)

	job, err = k8s.WaitForJob(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, jobName, endTime.Sub(time.Now()))
	if err != nil || job.Status.Succeeded != 1 {
		rb.DumpTestInfo(jobName)
		logs, _ := k8s.GetJobsLogs(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, jobName, true)
		log.Printf("%s job output: %s", jobName, logs)
	}
	assert.Assert(t, err)
	prv1Cluster.KubectlExec("logs job/" + jobName)
	k8s.AssertJob(t, job)

}

func SendReceive(addr string) error {
	doneCh := make(chan error)
	go func(doneCh chan error) {

		strEcho := "Halo\n"

		log.Println("Starting openssl s_client...")
		cmd := exec.Command(
			"openssl",
			"s_client",
			"-quiet",
			"-verify_return_error",
			"-connect",
			addr,
			"-CAfile",
			"/tmp/certs/skupper-tls-ssl-server/ca.crt",
		)
		cmd.Stderr = os.Stderr

		pipeIn, err := cmd.StdinPipe()
		if err != nil {
			doneCh <- fmt.Errorf("error opening stdin pipe: %w", err)
			return
		}
		defer pipeIn.Close()

		pipeOut, err := cmd.StdoutPipe()
		if err != nil {
			doneCh <- fmt.Errorf("error opening stdout pipe: %w", err)
			return
		}

		err = cmd.Start()
		if err != nil {
			doneCh <- fmt.Errorf("error opening stdin pipe: %w", err)
		}
		defer func() {
			log.Printf("Closing stdin pipe...")
			pipeIn.Close()
			log.Printf("Waiting for the command to complete...")
			cmd.Wait()
			log.Printf("...done")
		}()

		log.Println("Sending data")
		_, err = pipeIn.Write([]byte(strEcho))
		if err != nil {
			doneCh <- fmt.Errorf("write to server failed: %w", err)
			return
		}

		log.Println("Receiving reply")

		pReader := bufio.NewReader(pipeOut)

		reply, err := pReader.ReadString('\n')

		if err != nil {
			doneCh <- fmt.Errorf("read from server failed: %w (reply: %q)", err, reply)
			return
		}

		log.Printf("Sent to server = %q", strEcho)
		log.Printf("Reply from server = %q", string(reply))

		if len(reply) == len(strEcho) {
			for i_c, c := range []byte(strings.Trim(strEcho, "\n")) {
				i_r := len(reply) - 2 - i_c
				r := reply[i_r]

				if r != c {
					doneCh <- fmt.Errorf("response from server different than expected: %s (%d/%q != %d/%q)", string(reply), i_r, r, i_c, c)
				}
			}
		} else {

			doneCh <- fmt.Errorf("response length from server different than expected: %s", string(reply))
		}

		doneCh <- nil
	}(doneCh)
	timeoutCh := time.After(time.Minute)

	// This will cause job to fail and a retry to occur if the job is hung
	var err error
	select {
	case err = <-doneCh:
	case <-timeoutCh:
		err = fmt.Errorf("timed out waiting for SendReceive function to finish")
	}

	return err
}
