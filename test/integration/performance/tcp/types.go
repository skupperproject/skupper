package tcp

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	IPERF_IMAGE = "quay.io/skupper/iperf3"
	TestPath    = "./tmp"
)

// IperfTuning represents possible iPerf3 customizations that can
// be done through the documented environment variables.
type IperfTuning struct {
	ParallelClients []int         `json:"parallelClients"`
	TransmitSizes   []string      `json:"transmitSizes"`
	WindowSize      int           `json:"windowSize"`
	Memory          string        `json:"memory"`
	Cpu             string        `json:"cpu"`
	JobTimeout      time.Duration `json:"jobTimeout"`
}

// ToArgs returns an argument list based on provided settings
func (i *IperfTuning) ToArgs(hostname string, size string, clients int) []string {
	params := []string{"-c", hostname, "-n", size}
	if clients > 0 {
		params = append(params, "-P", strconv.Itoa(clients))
	}
	if i.WindowSize > 0 {
		params = append(params, "-w", strconv.Itoa(i.WindowSize))
	}
	return params
}

// SkupperTuning represents Skupper settings that can be customized through
// environment variables.
type SkupperTuning struct {
	Sites            int    `json:"sites"`
	MaxFrameSize     int    `json:"maxFrameSize"`
	MaxSessionFrames int    `json:"maxSessionFrames"`
	Memory           string `json:"memory"`
	Cpu              string `json:"cpu"`
}

// IperfScenario represents each test iteration and it contains
// everything the test iteration needs
type IperfScenario struct {
	SkupperSites     int           `json:"skupperSites"`
	TransmitSize     string        `json:"transmitSize"`
	ParallelClients  int           `json:"parallelClients"`
	SkupperSettings  SkupperTuning `json:"skupperSettings"`
	IperfSettings    IperfTuning   `json:"iperfSettings"`
	testCtx          context.Context
	testRunner       *base.ClusterTestRunnerBase
	teardownClusters []*base.ClusterContext
}

// getTestName returns an identification for the test iteration
func (s *IperfScenario) getTestName() string {
	return fmt.Sprintf("skupper-iperf3-sites_%d-size_%s-clients_%d",
		s.SkupperSites, s.TransmitSize, s.ParallelClients)
}

// clustersNeeded returns number of clusters (contexts) needed by the iteration
func (s *IperfScenario) clustersNeeded() int {
	if s.SkupperSites == 0 {
		return 1
	}
	return s.SkupperSites
}

// tearDown removes created namespaces
func (s *IperfScenario) tearDown() {
	for _, ctx := range s.teardownClusters {
		_ = ctx.DeleteNamespace()
	}
	s.teardownClusters = []*base.ClusterContext{}
}

// String returns a JSON representation of the test scenario
func (s *IperfScenario) String() string {
	out, _ := json.MarshalIndent(s, "", "  ")
	return string(out)
}

// initializeClusters creates the namespaces, initializes Skupper,
// if using a skupper site, and get sites connected.
func (s *IperfScenario) initializeClusters(t *testing.T, debugMode bool) {
	needs := base.ClusterNeeds{
		NamespaceId:    "iperf",
		PublicClusters: s.clustersNeeded(),
	}
	s.testRunner = &base.ClusterTestRunnerBase{}
	if err := s.testRunner.Validate(needs); err != nil {
		t.Skipf("%s", err)
	}
	_, err := s.testRunner.Build(needs, nil)
	assert.Assert(t, err)

	routerOptions := types.RouterOptions{
		Tuning: types.Tuning{
			Cpu:    s.SkupperSettings.Cpu,
			Memory: s.SkupperSettings.Memory,
		},
		MaxFrameSize:     s.SkupperSettings.MaxFrameSize,
		MaxSessionFrames: s.SkupperSettings.MaxSessionFrames,
	}
	if debugMode {
		t.Logf("setting router debug options")
		routerOptions = constants.DefaultRouterOptions(&routerOptions)
	}

	if s.SkupperSites > 0 {
		var connectionToken string

		for i := 1; i <= s.clustersNeeded(); i++ {
			// Getting the cluster context info
			ctx, err := s.testRunner.GetPublicContext(i)
			assert.Assert(t, err)
			// Creating the namespace
			assert.Assert(t, ctx.CreateNamespace())
			s.teardownClusters = append(s.teardownClusters, ctx)
			siteConfigSpec := types.SiteConfigSpec{
				RouterMode:        "interior",
				EnableController:  true,
				EnableServiceSync: true,
				AuthMode:          "internal",
				User:              "admin",
				Password:          "admin",
				Ingress:           ctx.VanClient.GetIngressDefault(),
				Router:            routerOptions,
				Controller: types.ControllerOptions{
					Tuning: types.Tuning{
						Cpu:    s.SkupperSettings.Cpu,
						Memory: s.SkupperSettings.Memory,
					},
				},
			}
			// If running against a single cluster only, use none
			if !base.MultipleClusters() {
				siteConfigSpec.Ingress = types.IngressNoneString
			}
			// Initializing skupper
			t.Logf("Initializing Skupper at: %s", ctx.Namespace)
			siteConfig, err := ctx.VanClient.SiteConfigCreate(s.testCtx, siteConfigSpec)
			assert.Assert(t, err)
			assert.Assert(t, ctx.VanClient.RouterCreate(s.testCtx, *siteConfig))

			// If i > 1 (if site2 connects to site1, site3 connects to site 2 and so on)
			if connectionToken != "" {
				prevCtx, _ := s.testRunner.GetPublicContext(i - 1)
				t.Logf("Connecting Skupper at %s with Skupper at %s", ctx.Namespace, prevCtx.Namespace)
				_, err := ctx.VanClient.ConnectorCreateFromFile(s.testCtx, connectionToken, types.ConnectorCreateOptions{SkupperNamespace: ctx.Namespace})
				assert.Assert(t, err)
			}

			// Creating token
			if i != s.clustersNeeded() {
				t.Logf("Creating Skupper token to connect with %s", ctx.Namespace)
				connectionToken = fmt.Sprintf("%s/cluster-%d.yaml", TestPath, i)
				assert.Assert(t, ctx.VanClient.ConnectorTokenCreateFile(s.testCtx, types.DefaultVanName, connectionToken))
			}
		}

		// Wait for skupper sites to be connected
		if s.clustersNeeded() > 1 {
			totalConnections := s.clustersNeeded() - 1
			for i := 1; i <= s.clustersNeeded(); i++ {
				ctx, _ := s.testRunner.GetPublicContext(i)
				assert.Assert(t, base.WaitForSkupperConnectedSites(s.testCtx, ctx, totalConnections))
			}
		}
	} else {
		// When running without Skupper
		// Getting the cluster context info
		ctx, err := s.testRunner.GetPublicContext(1)
		assert.Assert(t, err)
		// Creating the namespace
		t.Logf("Creating namespace: %s (without Skupper)", ctx.Namespace)
		assert.Assert(t, ctx.CreateNamespace())
		s.teardownClusters = append(s.teardownClusters, ctx)
	}
}

func (s *IperfScenario) tailRouterLogs(t *testing.T, ctx context.Context, saveLogs *bool) *sync.WaitGroup {
	wg := &sync.WaitGroup{}
	basedir := fmt.Sprintf("tmp/%s", s.getTestName())
	err := os.MkdirAll(basedir, 0755)
	if err != nil {
		t.Logf("Unable to save router logs. Cannot create directory %s: %v", basedir, err)
		return wg
	}
	tailRouterLogs := func(cluster *base.ClusterContext) {
		defer wg.Done()
		cli := cluster.VanClient
		fileName := fmt.Sprintf("%s/%s-skupper-router.log", basedir, cli.Namespace)
		tgzFileName := fmt.Sprintf("%s.tar.gz", fileName)
		var pod *v1.Pod
		var err error
		t.Logf("Waiting for router pod to be running")
		err = utils.RetryWithContext(ctx, time.Second*5, func() (bool, error) {
			pod, err = kube.GetReadyPod(cli.Namespace, cli.KubeClient, "router")
			if pod != nil && err == nil {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			t.Logf("Error waiting for skupper-router pod to be running: %v", err)
			return
		}
		t.Logf("Buffering router logs for namespace '%s'", cli.Namespace)
		req := cli.KubeClient.CoreV1().Pods(cli.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{
			Follow:    true,
			Container: "router",
		})
		logsStream, err := req.Stream()
		if err != nil {
			t.Logf("Error getting router logs on namespace '%s': %v", cli.Namespace, err)
			return
		}
		buf := &bytes.Buffer{}
		wr, err := io.Copy(buf, logsStream)
		if err != nil {
			t.Logf("Error streaming router logs on namespace '%s': %v", cli.Namespace, err)
			return
		}
		if *saveLogs {
			t.Logf("Router logs buffer has been collected for namespace '%s': %d bytes", cli.Namespace, wr)
			tgz, err := os.Create(tgzFileName)
			if err != nil {
				t.Logf("Error creating tarball '%s': %v", tgzFileName, err)
				return
			}
			gz := gzip.NewWriter(tgz)
			defer gz.Close()
			tw := tar.NewWriter(gz)
			defer tw.Close()

			hdr := &tar.Header{
				Name: fileName,
				Mode: 0644,
				Size: wr,
			}
			err = tw.WriteHeader(hdr)
			if err != nil {
				t.Logf("Failed to write tar file header for '%s': %v", tgzFileName, err)
				return
			}
			_, err = tw.Write(buf.Bytes())
			if err != nil {
				t.Logf("Failed to write to tar archive '%s': %v", tgzFileName, err)
				return
			}
			tgzPath, _ := filepath.Abs(tgzFileName)
			t.Logf("Tarball has been saved: %s", tgzPath)
		}
	}
	if s.SkupperSites == 0 {
		return wg
	}
	for _, cluster := range s.testRunner.ClusterContexts {
		wg.Add(1)
		go tailRouterLogs(cluster)
	}
	return wg
}

// deployIperf3Server Deploys the iPerf3 server and wait for it to be running
func (s *IperfScenario) deployIperf3Server(t *testing.T) {
	t.Logf("Deploying iperf3-server")
	// Specify resource requirements (if requested)
	var resourceReqs v1.ResourceRequirements
	if s.IperfSettings.Memory != "" || s.IperfSettings.Cpu != "" {
		requests := map[v1.ResourceName]resource.Quantity{}
		if s.IperfSettings.Memory != "" {
			memoryQty, err := resource.ParseQuantity(s.IperfSettings.Memory)
			assert.Assert(t, err)
			requests[v1.ResourceMemory] = memoryQty
		}
		if s.IperfSettings.Cpu != "" {
			cpuQty, err := resource.ParseQuantity(s.IperfSettings.Cpu)
			assert.Assert(t, err)
			requests[v1.ResourceCPU] = cpuQty
		}
		resourceReqs.Requests = requests
	}

	// Deploy iPerf3 server
	iperfServerCluster, _ := s.testRunner.GetPublicContext(1)
	iperfServerDep, err := k8s.NewDeployment("iperf3-server", iperfServerCluster.Namespace, k8s.DeploymentOpts{
		Image:         IPERF_IMAGE,
		Labels:        map[string]string{"app": "iperf3-server"},
		RestartPolicy: v1.RestartPolicyAlways,
		Args:          []string{"-s"},
		ResourceReq:   resourceReqs,
	})
	assert.Assert(t, err)
	_, err = iperfServerCluster.VanClient.KubeClient.AppsV1().Deployments(iperfServerCluster.Namespace).Create(iperfServerDep)
	assert.Assert(t, err)
	// Waiting for iperf3-server to be ready
	_, err = kube.WaitDeploymentReadyReplicas(iperfServerDep.Name, iperfServerCluster.Namespace, 1,
		iperfServerCluster.VanClient.KubeClient, constants.SkupperServiceReadyPeriod, constants.DefaultTick)
	assert.Assert(t, err)
}

// exposeIperf3Server exposes the iperf3-server as a skupper service
// if using at least 1 skupper site, or creates a k8s service exposing
// the pods directly.
func (s *IperfScenario) exposeIperf3Server(t *testing.T) {
	// iPerf3 server always run on cluster 1
	iperfServerCluster, _ := s.testRunner.GetPublicContext(1)

	// If using skupper then create and expose the iperf3 service
	if s.SkupperSites > 0 {
		t.Logf("Exposing iperf3-server (service) through the Skupper network")
		// Creating a Skupper service
		iperf3Service := &types.ServiceInterface{
			Address:  "iperf3-server",
			Protocol: "tcp",
			Ports:    []int{5201},
		}
		assert.Assert(t, iperfServerCluster.VanClient.ServiceInterfaceCreate(s.testCtx, iperf3Service))
		// Binding the service to the deployment
		assert.Assert(t, iperfServerCluster.VanClient.ServiceInterfaceBind(s.testCtx, iperf3Service, "deployment", iperf3Service.Address, "tcp", map[int]int{5201: 5201}))

		// Waiting for service to be available across all namespaces/clusters
		for i := 1; i <= s.clustersNeeded(); i++ {
			ctx, _ := s.testRunner.GetPublicContext(i)
			_, err := k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(ctx.Namespace, ctx.VanClient.KubeClient, "iperf3-server")
			assert.Assert(t, err)
		}
	} else {
		t.Logf("Creating iperf3-server (service) without Skupper")
		// Create a simple k8s service
		_, err := iperfServerCluster.VanClient.KubeClient.CoreV1().Services(iperfServerCluster.Namespace).Create(&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "iperf3-server",
				Labels: map[string]string{"app": "iperf3-server"},
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{Port: 5201},
				},
				Selector: map[string]string{"app": "iperf3-server"},
			},
		})
		assert.Assert(t, err)
	}
}

// runIperf3Client runs the iperf3-client as a job
// then saves its logs to a local file.
func (s *IperfScenario) runIperf3Client(t *testing.T) {
	t.Logf("Running iperf3-client")
	// Specify resource requirements (if requested)
	var resourceReqs v1.ResourceRequirements
	if s.IperfSettings.Memory != "" || s.IperfSettings.Cpu != "" {
		requests := map[v1.ResourceName]resource.Quantity{}
		if s.IperfSettings.Memory != "" {
			memoryQty, err := resource.ParseQuantity(s.IperfSettings.Memory)
			assert.Assert(t, err)
			requests[v1.ResourceMemory] = memoryQty
		}
		if s.IperfSettings.Cpu != "" {
			cpuQty, err := resource.ParseQuantity(s.IperfSettings.Cpu)
			assert.Assert(t, err)
			requests[v1.ResourceCPU] = cpuQty
		}
		resourceReqs.Requests = requests
	}

	// Running the iperf3 client
	clientCluster, _ := s.testRunner.GetPublicContext(1)
	if s.SkupperSites > 0 {
		clientCluster, _ = s.testRunner.GetPublicContext(s.clustersNeeded())
	}
	clientJob := k8s.NewJob("iperf3-client", clientCluster.Namespace, k8s.JobOpts{
		Image:        IPERF_IMAGE,
		BackoffLimit: 10,
		Restart:      v1.RestartPolicyNever,
		Labels:       map[string]string{"job": "iperf3-client"},
		Args:         s.IperfSettings.ToArgs("iperf3-server", s.TransmitSize, s.ParallelClients),
		ResourceReq:  resourceReqs,
	})

	// Running job with multiple retries
	_, err := clientCluster.VanClient.KubeClient.BatchV1().Jobs(clientCluster.Namespace).Create(clientJob)
	assert.Assert(t, err)
	// Waiting for the job to complete successfully
	_, jobErr := k8s.WaitForJob(clientCluster.Namespace, clientCluster.VanClient.KubeClient, clientJob.Name, s.IperfSettings.JobTimeout)
	if jobErr != nil {
		s.testRunner.DumpTestInfo(s.getTestName())
	}
	// Saving job logs
	logs, err := k8s.GetJobLogs(clientCluster.Namespace, clientCluster.VanClient.KubeClient, clientJob.Name)
	assert.Assert(t, err)

	// Writing job logs to file before asserting if job has passed
	logFileName := fmt.Sprintf("%s/%s.log", TestPath, s.getTestName())
	fullLogFileName, _ := filepath.Abs(logFileName)
	t.Logf("Writing iperf3-client logs at: %s", fullLogFileName)
	logFile, err := os.Create(logFileName)
	assert.Assert(t, err)
	defer logFile.Close()
	_, err = logFile.WriteString(logs)
	assert.Assert(t, err)

	// Assert job has completed
	assert.Assert(t, jobErr)
}
