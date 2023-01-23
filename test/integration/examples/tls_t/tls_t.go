//go:build integration || smoke || examples || job
// +build integration smoke examples job

// This test is based on the tcp_echo example test.  However, instead
// of using a tcp echo server, it uses openssl's s_server and s_client
// for the communication, with a service that is tls-enabled.
//
// Here, the tests consist of using different s_server and s_client
// flags to check how the skupper-router reacts to them.  In all cases,
// we write a string through the connection and read its response (on
// the server, we're using -rev, so the response is the reverse of what
// we just sent).
package tls_t

import (
	"bufio"
	"context"
	"fmt"
	"io"
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

// A skupper service with tls just by setting the credentials
var service = types.ServiceInterface{
	Address:          "ssl-server",
	Protocol:         "tcp",
	Ports:            serverPortsInt(),
	TlsCredentials:   "skupper-tls-ssl-server",
	TlsCertAuthority: "skupper-service-client",
}

// The options will be sent to openssl s_server's call from cmd.Exec, but
// that command will be executing 'sh -c', so any redirections, variables
// or quoted arguments are accepted in this single string; they'll just
// be concatenated to the rest of the command before execution
type serverProfile struct {
	Port    int32
	Options string
}

var plainServer = serverProfile{
	Port:    8443,
	Options: "",
}

var tls1Server = serverProfile{
	Port:    8444,
	Options: "-tls1",
}

var tls1_1Server = serverProfile{
	Port:    8445,
	Options: "-tls1_1",
}

var tls1_2Server = serverProfile{
	Port:    8446,
	Options: "-tls1_2",
}

var tls1_3Server = serverProfile{
	Port:    8447,
	Options: "-tls1_3",
}

// These are no longer supported by the openssl client, so we
// cannot test with them
// var ssl2Server = serverProfile{
// 	Port:    8448,
// 	Options: "-ssl2",
// }
// var ssl3Server = serverProfile{
// 	Port:    8449,
// 	Options: "-ssl3",
// }

var alpnServer = serverProfile{
	Port:    8449,
	Options: "-alpn test_proto1",
}

var npnServer = serverProfile{
	Port:    8450,
	Options: "-nextprotoneg test_proto1 -no_tls1_3",
}

// Compression
var compServer = serverProfile{
	Port:    8451,
	Options: "-comp",
}

var noTicketServer = serverProfile{
	Port:    8452,
	Options: "-no_ticket -no_tls1_3",
}

var prefServer = serverProfile{
	Port:    8453,
	Options: "-serverpref",
}

var bugsServer = serverProfile{
	Port:    8454,
	Options: "-bugs",
}

// We do not currently support SNI
var sniServer = serverProfile{
	Port: 8455,
	Options: "-servername ssl-server " +
		"-cert2 /cert/tls.crt " +
		"-key2 /cert/tls.key ",
}

var servers = []serverProfile{
	plainServer,
	tls1Server,
	tls1_1Server,
	tls1_2Server,
	tls1_3Server,
	alpnServer,
	npnServer,
	compServer,
	noTicketServer,
	prefServer,
	bugsServer,
	sniServer,
}

// Returns a string with a shell line consisting of multiple commands to be
// run in background, and a final 'wait' command.  It's to be used in an
// invocation to 'sh -c'
func testServers() string {
	var resp string
	for _, s := range servers {
		resp += "openssl s_server " +
			"-cert /cert/tls.crt " +
			"-key /cert/tls.key " +
			"-servername_fatal " +
			// The client is not presenting a certificate, so we do not use these
			// "-Verify 10 " +
			// "-verify_return_error " +
			"-brief " +
			"-rev "
		resp += fmt.Sprintf("--port %d %v & ", s.Port, s.Options)
	}
	resp += "wait"
	return resp
}

// The ports to be exposed by the container hosting the SSL server
func serverPorts() (resp []apiv1.ContainerPort) {
	for _, s := range servers {
		resp = append(resp,
			apiv1.ContainerPort{

				Protocol:      apiv1.ProtocolTCP,
				ContainerPort: s.Port,
			})
	}
	return
}

// A list of ports the server will be listening to.  It will be used on the
// Skupper service
func serverPortsInt() (resp []int) {
	for _, s := range servers {
		resp = append(resp, int(s.Port))
	}
	return
}

// The options will be sent to openssl s_client's call via cmd.Exec,
// as provided
type clientProfile struct {
	Options []string
}

var plainClient = clientProfile{
	Options: []string{},
}

var tls1Client = clientProfile{
	Options: []string{"-tls1"},
}

var tls1_1Client = clientProfile{
	Options: []string{"-tls1_1"},
}

var tls1_2Client = clientProfile{
	Options: []string{"-tls1_2"},
}

var tls1_3Client = clientProfile{
	Options: []string{"-tls1_3"},
}

// No longer supported by openssl cli
// var ssl3Client = clientProfile{
// 	Options: []string{"-ssl3"},
// }

var reconnectClient = clientProfile{
	Options: []string{"-reconnect"},
}

var bugsClient = clientProfile{
	Options: []string{"-bugs"},
}

var compClient = clientProfile{
	Options: []string{"-comp"},
}

var alpnClient = clientProfile{
	Options: []string{"-alpn", "test_proto1"},
}

var npnClient = clientProfile{
	Options: []string{"-nextprotoneg", "test_proto1", "-no_tls1_3"},
}

// We do not currently support SNI
var sniClient = clientProfile{
	Options: []string{"-servername", "ssl-server"},
}

var Tests = []struct {
	Client  clientProfile
	Server  serverProfile
	Success bool
	// a string to be sought in the initial openssl cli output.  No match is a failure
	Seek string
}{
	// plainClient with a variety of servers
	{
		Client:  plainClient,
		Server:  plainServer,
		Success: true,
	}, {
		Client:  plainClient,
		Server:  tls1Server,
		Success: false,
	}, {
		Client:  plainClient,
		Server:  tls1_1Server,
		Success: false,
	}, {
		Client:  plainClient,
		Server:  tls1_2Server,
		Success: true,
	}, {
		Client:  plainClient,
		Server:  tls1_3Server,
		Success: true,
	}, {
		Client:  plainClient,
		Server:  alpnServer,
		Success: true,
	}, {
		Client:  plainClient,
		Server:  npnServer,
		Success: true,
	}, {
		// TLS compression is insecure; we want to make sure
		// it is not active, even if we ask for it
		Client:  plainClient,
		Server:  compServer,
		Seek:    "Compression: NONE",
		Success: true,
	}, {
		Client:  plainClient,
		Server:  noTicketServer,
		Success: true,
	}, {
		Client:  plainClient,
		Server:  prefServer,
		Success: true,
	}, {
		Client:  plainClient,
		Server:  bugsServer,
		Success: true,
	}, {
		Client:  plainClient,
		Server:  sniServer,
		Success: false,
	},
	// plainServer with a variety of clients
	{
		Client:  tls1Client,
		Server:  plainServer,
		Success: false,
	}, {
		Client:  tls1_1Client,
		Server:  plainServer,
		Success: false,
	}, {
		Client:  tls1_2Client,
		Server:  plainServer,
		Success: true,
	}, {
		Client:  tls1_3Client,
		Server:  plainServer,
		Success: true,
	}, {
		Client:  bugsClient,
		Server:  plainServer,
		Success: true,
	}, {
		// TLS compression is insecure; we want to make sure
		// it is not active, even if we ask for it
		Client:  compClient,
		Server:  plainServer,
		Seek:    "Compression: NONE",
		Success: true,
	}, {
		Client:  alpnClient,
		Server:  plainServer,
		Seek:    "ALPN protocol: test_proto1",
		Success: false,
	}, {
		Client:  npnClient,
		Server:  plainServer,
		Seek:    "Next protocol: (1) proto_test1",
		Success: false,
	}, {
		// This works but shouldn't.  The problem, however, is on the
		// test.  The client is asking about SNI, the server responds
		// with its 'main' cert and responder, which are the same as
		// the ones accessible via SNI, so the client is fine with it.
		//
		// A proper test should have different certificate and responder
		// behind the 'main' and the 'SNI' names.  However, as we do
		// not support SNI at the moment, we're not putting time into that.
		Client:  sniClient,
		Server:  plainServer,
		Success: true,
	},
	// matching cases (ie both client and server are non-plain)
	{
		// TLS compression is insecure; we want to make sure
		// it is not active, even if we ask for it
		Client:  compClient,
		Server:  compServer,
		Seek:    "Compression: NONE",
		Success: true,
	}, {
		Client: alpnClient,
		Server: alpnServer,
		// TODO: this is a known error; change this once it is fixed
		Success: false,
		Seek:    "ALPN protocol: test_proto1",
	}, {
		Client: npnClient,
		Server: npnServer,
		// We do not support NPN.
		Success: false,
		Seek:    "Next protocol: (1) proto_test1",
	}, {
		Client:  sniClient,
		Server:  sniServer,
		Success: false,
	},
	// TODO: This often fails, and causes further tests to fail, if it comes
	//       first: why does that happen, and why does it not impact tests
	//       coming from the next job run?
	//       That's under investigation on skupper-router #864.  Once that
	//       is closed, move this to the top of the list and reactivate it.
	//	{
	//		Client:  reconnectClient,
	//		Server:  plainServer,
	//		Success: true,
	//	},
}

// This is a deployment of Skupper test image, running the different
// openssl s_server processes generated by testServers()
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
						ImagePullPolicy: k8s.GetTestImagePullPolicy(),
						Args: []string{
							"sh", "-c",
							testServers(),
						},
						Ports: serverPorts(),
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
	_ = pub1Cluster.VanClient.KubeClient.AppsV1().Deployments(pub1Cluster.Namespace).Delete(context.TODO(), Deployment.Name, metav1.DeleteOptions{})
}

func setup(ctx context.Context, t *testing.T, r base.ClusterTestRunner) {
	pub1Cluster, _ := r.GetPublicContext(1)
	publicDeploymentsClient := pub1Cluster.VanClient.KubeClient.AppsV1().Deployments(pub1Cluster.Namespace)

	// We need to create the service interface before the deployment, because
	// the deployment needs to mount the secret created by the service
	err := pub1Cluster.VanClient.ServiceInterfaceCreate(ctx, &service)
	assert.Assert(t, err)

	fmt.Println("Creating deployment...")
	result, err := publicDeploymentsClient.Create(context.TODO(), Deployment, metav1.CreateOptions{})
	assert.Assert(t, err)

	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())

	fmt.Printf("Listing deployments in namespace %q:\n", pub1Cluster.Namespace)
	list, err := publicDeploymentsClient.List(context.TODO(), metav1.ListOptions{})
	assert.Assert(t, err)

	for _, d := range list.Items {
		fmt.Printf(" * %s (%d replicas)\n", d.Name, *d.Spec.Replicas)
	}

	err = pub1Cluster.VanClient.ServiceInterfaceBind(ctx, &service, "deployment", "ssl-server", map[int]int{})
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
	jobCmd := []string{"/app/tls_test", "-test.run", "TestTlsJob", "-test.v"}

	// Note here we are executing the same test, but in two different
	// namespaces (or clusters); the same service will exist in both clusters
	// because of the skupper connections and the "skupper exposed"
	// deployment.
	_, err = k8s.CreateTestJobWithSecret(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, jobName, jobCmd, "skupper-tls-ssl-server")
	assert.Assert(t, err)

	_, err = k8s.CreateTestJobWithSecret(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, jobName, jobCmd, "skupper-tls-ssl-server")
	assert.Assert(t, err)

	endTime = time.Now().Add(constants.ImagePullingAndResourceCreationTimeout)

	job, err := k8s.WaitForJob(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, jobName, endTime.Sub(time.Now()))
	if err != nil || job.Status.Succeeded != 1 {
		logs, _ := k8s.GetJobsLogs(pub1Cluster.Namespace, pub1Cluster.VanClient.KubeClient, jobName, true)
		log.Printf("%s job output: %s", jobName, logs)
	}
	assert.Assert(t, err)
	pub1Cluster.KubectlExec("logs job/" + jobName)
	k8s.AssertJob(t, job)

	job, err = k8s.WaitForJob(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, jobName, endTime.Sub(time.Now()))
	if err != nil || job.Status.Succeeded != 1 {
		logs, _ := k8s.GetJobsLogs(prv1Cluster.Namespace, prv1Cluster.VanClient.KubeClient, jobName, true)
		log.Printf("%s job output: %s", jobName, logs)
	}
	assert.Assert(t, err)
	prv1Cluster.KubectlExec("logs job/" + jobName)
	k8s.AssertJob(t, job)

}

// openssl s_client sends a lot of output on connection to the stdout.  We
// cannot ignore it with -quiet, as the information there can be useful.
// However, we cannot send it to stderr either, as the command does not provide
// such functionality.  So, we just flush it on the log.  Network commands may
// take a while to show everything, so we give the command some time to
// complete.
// 'seek', if given, is a string to be searched for on the initial flush.  If
// given and not found, return an error.
func flushStdOut(outputCh <-chan string, seek string) error {
	log.Printf("Flushing stdout")

	var found bool

	if seek == "" {
		found = true
	}

	var count int

outer:
	for {
		count++
		timeoutCh := time.After(time.Second)
		var line string
		var ok bool

		select {
		case line, ok = <-outputCh:
		case <-timeoutCh:
			log.Printf("Flush complete")
			break outer
		}
		if !ok {
			return fmt.Errorf("command output finished during initial stdout flush")
		}
		log.Printf("  %v", line)
		if strings.Contains(line, seek) {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("stdout flush did not match string %q", seek)
	}
	return nil

}

// This is used by the job.  It creates the connection, flushes the
// initial output, sends the payload, reads and validates the
// response.
//
// The connection is created to `addr`, and additional options to
// s_client can be given in `options`.  If provided, the initial
// output will be searched for the `seek` string.  If that is not
// found, the function fails.
func SendReceive(addr string, options []string, seek string) error {
	defer func() {
		log.Println("SendReceive completed")
	}()
	cmdArgs := []string{
		"s_client",
		"-verify_return_error",
		"-connect",
		addr,
		"-CAfile",
		"/tmp/certs/skupper-tls-ssl-server/ca.crt",
		"-no_ign_eof",
		"-verify_hostname", "ssl-server",
		"-tlsextdebug",
	}
	cmdArgs = append(cmdArgs, options...)

	// there is no reason to keep the command waiting for output
	// buffer space to be available: at first, we want to flush it
	// all out, then we read just one line after one write.  So, we
	// read as much as we can without blocking.
	//
	// This channel will be closed on EOF or any other error when
	// reading the command's stdout.
	var outputCh = make(chan string, 100)

	doneCh := make(chan error)
	go func(doneCh chan error) {

		strEcho := "Halo\n"

		log.Println("Starting openssl s_client...")
		log.Printf("Executing command openssl %v", cmdArgs)
		cmd := exec.Command("openssl", cmdArgs...)
		// We're not processing stderr, so just send it to the main stderr;
		// we'll get it on the logs, then.
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
			doneCh <- fmt.Errorf("error starting command: %w", err)
		}
		defer func() {
			log.Printf("Closing stdin pipe...")
			pipeIn.Close()
			log.Printf("Waiting for the command to complete...")
			err := cmd.Wait()
			if err != nil {
				log.Printf("After wait, the command returned an error: %v", err)
			}
			log.Printf("...done")
		}()

		pReader := bufio.NewReader(pipeOut)

		go func() {
			for {
				line, err := pReader.ReadString('\n')
				outputCh <- line
				if err == io.EOF {
					log.Printf("Read EOF")
					close(outputCh)
					return
				}
				if err != nil {
					log.Printf("non-EOF error reading command output: %v", err)
					close(outputCh)
					return
				}
			}
		}()

		err = flushStdOut(outputCh, seek)
		if err != nil {
			doneCh <- err
			return
		}

		log.Println("Sending data")
		_, err = pipeIn.Write([]byte(strEcho))
		if err != nil {
			doneCh <- fmt.Errorf("write to server failed: %w", err)
			return
		}

		log.Println("Receiving reply")

		reply := <-outputCh

		log.Printf("Sent to server = %q", strEcho)
		log.Printf("Reply from server = %q", string(reply))

		if len(reply) == len(strEcho) {
			// We're using openssl s_server -rev, so we have to revert
			// what we send to check the response
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

	log.Print("Flushing stdout after test")
	// Otherwise, output gets mixed with the next test
	for {
		select {
		case line, ok := <-outputCh:
			if !ok {
				// We're done here, so we just return whatever err we got before
				return err
			}
			log.Printf("  %v", line)
		case <-timeoutCh:
			if err != nil {
				log.Printf("timed out waiting for SendReceive, but received an error befor: %v", err)
			}
			return fmt.Errorf("timed out waiting for SendReceive function to finish")
		}
	}
}
