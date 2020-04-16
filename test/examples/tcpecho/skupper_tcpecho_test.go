package tcpecho_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
    "github.com/skupperproject/skupper/test/examples/tcpecho"
	"github.com/rh-messaging/shipshape/pkg/apps/skupper"
	"github.com/rh-messaging/shipshape/pkg/framework"
	"io/ioutil"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"strings"
	"time"
)

var _ = Describe("skupper-example-tcp-echo", func() {

	It("is able access exposed service from another namespace", func() {

		// TCP Echo Deployment YAML to use
		tcpEchoDeploymentUrl := "https://raw.githubusercontent.com/skupperproject/skupper-example-tcp-echo/master/public-deployment.yaml"

		// Loading tcp-echo deployment from github
		tcpEchoDeployment, err := framework.LoadYamlFromUrl(tcpEchoDeploymentUrl)
		gomega.Expect(err).To(gomega.BeNil())

		// Deploying the tcp-echo demo
		By("creating the tcp-go-echo application on public namespace")
		tcpEchoDeployment, err = tcpecho.PubCtx.CreateResourceGroupVersion(tcpecho.TcpEchoDeploymentGroupVersion, tcpEchoDeployment, v1.CreateOptions{})
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(tcpEchoDeployment).NotTo(gomega.BeNil())

		// Wait for tcp-echo successful deployment
		err = framework.WaitForDeployment(tcpecho.PubCtx.Clients.KubeClient, tcpecho.PubCtx.Namespace, tcpEchoDeployment.GetName(), 1, 10 * time.Second, 300 * time.Second)
		gomega.Expect(err).To(gomega.BeNil())

		// Interface for skupper (currently using CLI)
		skupperPublic := skupper.NewSkupper(tcpecho.PubCtx)
		skupperPrivate := skupper.NewSkupper(tcpecho.PrvCtx)

		// Exposing the tcp-echo deployment
		By("exposing the tcp-go-echo deployment")
		err = skupperPublic.ExposeDeployment(tcpEchoDeployment.GetName(), skupper.ExposeFlags{Port: 9090})
		gomega.Expect(err).To(gomega.BeNil())

		// Saving the connection token
		By("creating a connection-token to the public namespace")
		tokenFile, _ := ioutil.TempFile("", "public-token")
		tokenFile.Close()
		defer os.Remove(tokenFile.Name())
		err = skupperPublic.ConnectionToken(tokenFile.Name(), skupper.ConnectionTokenFlags{})
		gomega.Expect(err).To(gomega.BeNil())

		// Connecting private namespace to public
		By("connecting the private namespace with the public")
		err = skupperPrivate.Connect(tokenFile.Name(), skupper.ConnectFlags{})
		gomega.Expect(err).To(gomega.BeNil())

		// Waiting for services (on public and private to be available)
		By("validating public tcp-go-echo service created")
		pubSvc, err := tcpecho.PubCtx.WaitForService(tcpEchoDeployment.GetName(), 60 * time.Second, 5 * time.Second)
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(pubSvc).NotTo(gomega.BeNil())

		By("validating private tcp-go-echo service created")
		prvSvc, err := tcpecho.PrvCtx.WaitForService(tcpEchoDeployment.GetName(), 300 * time.Second, 10 * time.Second)
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(prvSvc).NotTo(gomega.BeNil())

		By("testing the tcp-go-echo service from the private namespace")
		// Deploy and wait for client pod to be running
		const testContent = "test content"
		commands := []string{"sh", "-c", fmt.Sprintf("echo \"%s\" | nc %s %d -i 1", testContent, prvSvc.Name, prvSvc.Spec.Ports[0].Port)}
		clientPod := createClientPod(commands...)
		clientPod, err = tcpecho.PrvCtx.WaitForPodStatus(clientPod.Name, v12.PodSucceeded, 300 * time.Second, 10 * time.Second)
		gomega.Expect(err).To(gomega.BeNil())
		// validating pod logs
		stdout, err := tcpecho.PrvCtx.GetLogs(clientPod.Name)
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(stdout).To(gomega.ContainSubstring(strings.ToUpper(testContent)))

	})

})

func createClientPod(commands ...string) *v12.Pod {
	// Create a pod to send messages through the tcp-go-echo service
	pb := framework.NewPodBuilder("tcp-go-echo-client", tcpecho.PrvCtx.Namespace)
	pb.RestartPolicy(string(v12.RestartPolicyNever))
	c := framework.
		NewContainerBuilder("tcp-go-echo-client", "docker.io/busybox").
		WithCommands(commands...).
		Build()
	pb.AddContainer(c)
	clientPod, err := tcpecho.PrvCtx.Clients.KubeClient.CoreV1().Pods(tcpecho.PrvCtx.Namespace).Create(pb.Build())
	gomega.Expect(err).To(gomega.BeNil())
	return clientPod
}
