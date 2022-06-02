//go:build integration || performance
// +build integration performance

package performance

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/test/integration/performance/common"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

type IperfTest common.PerformanceApp

var (
	iperfSettings IperfTuning
)

const (
	IPERF_IMAGE = "quay.io/skupper/iperf3"

	ENV_IPERF_PARALLEL_CLIENTS = "IPERF_PARALLEL_CLIENTS"
	ENV_IPERF_TRANSMIT_SIZES   = "IPERF_TRANSMIT_SIZES"
	ENV_IPERF_WINDOW_SIZE      = "IPERF_WINDOW_SIZE"
	ENV_IPERF_MEMORY           = "IPERF_MEMORY"
	ENV_IPERF_CPU              = "IPERF_CPU"
	ENV_IPERF_JOB_TIMEOUT      = "IPERF_JOB_TIMEOUT"
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
	Env             common.AppSettings
}

// ToArgs returns an argument list based on provided settings
func (i *IperfTuning) ToArgs(hostname string, size string, clients int) []string {
	params := []string{"-c", hostname, "-n", size, "-f", "g"}
	if clients > 0 {
		params = append(params, "-P", strconv.Itoa(clients))
	}
	if i.WindowSize > 0 {
		params = append(params, "-w", strconv.Itoa(i.WindowSize))
	}
	return params
}

func TestIperf3(t *testing.T) {
	// Parsing iperfSettings
	parseIperfSettings()

	i := &IperfTest{
		Name:           "iperf3",
		Description:    "iPerf3 TCP performance",
		Server:         getIperfServerInfo(),
		Service:        getIperfServiceDef(),
		Client:         getIperfClientInfo(),
		ThroughputUnit: common.ThroughputUnitGbps,
		LatencyUnit:    common.LatencyUnitMs,
	}
	assert.Assert(t, common.RunPerformanceTest(i))
}

func (i *IperfTest) App() common.PerformanceApp {
	return common.PerformanceApp(*i)
}

func (i *IperfTest) Validate(serverCluster, clientCluster *base.ClusterContext, job common.JobInfo) common.Result {
	res := common.Result{}

	log.Println("validating client result")
	// Saving job logs
	logs, err := k8s.GetJobLogs(clientCluster.Namespace, clientCluster.VanClient.KubeClient, job.Name)
	if err != nil {
		res.SetError(err)
		return res
	}

	// Parsing throughput
	// group 2 is the throughput and 3 is the unit
	exp, _ := regexp.Compile(`^\[\s*[0-9]+](\s+\S+){4}\s+(\S+)\s+(\S+)\s+\S*\s+(sender|receiver)`)
	if job.Clients > 1 {
		exp, _ = regexp.Compile(`^\[SUM](\s+\S+){4}\s+(\S+)\s+(\S+)\s+\S*\s+(sender|receiver)`)
	}

	buf := bytes.NewBufferString(logs)
	var line string
	avg := 0.0
	matches := 0
	for {
		line, err = buf.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				res.SetError(fmt.Errorf("error reading file: %v", err))
				return res
			} else {
				break
			}
		}
		if exp.MatchString(line) {
			match := exp.FindStringSubmatch(line)
			tp, err := strconv.ParseFloat(match[2], 64)
			if err == nil {
				avg += tp
				matches += 1
			} else {
				res.SetError(fmt.Errorf("error parsing throughput: %v", err))
				return res
			}
		}
	}

	if matches > 0 {
		avg = avg / float64(matches)
	}

	res.Throughput = avg
	log.Printf("throughput: %.2f %s", res.Throughput, i.App().ThroughputUnit)

	return res
}

func getIperfServerInfo() *common.ServerInfo {
	return &common.ServerInfo{
		Name:       "iperf3-server",
		Resources:  getIperfResources(),
		Settings:   getIperfServerSettings(),
		Deployment: getIperfDeployment(),
	}
}

func getIperfDeployment() *appsv1.Deployment {
	// Deploy iPerf3 server
	iperfServerDep, _ := k8s.NewDeployment("iperf3-server", "", k8s.DeploymentOpts{
		Image:         IPERF_IMAGE,
		Labels:        map[string]string{"app": "iperf3-server"},
		RestartPolicy: v1.RestartPolicyAlways,
		Args:          []string{"-s"},
	})
	return iperfServerDep
}

func getIperfServerSettings() common.AppSettings {
	return iperfSettings.Env
}

func getIperfResources() common.ResourceSettings {
	return common.ResourceSettings{
		Memory: iperfSettings.Memory,
		CPU:    iperfSettings.Cpu,
	}
}

func getIperfServiceDef() common.ServiceInfo {
	return common.ServiceInfo{
		Address:  "iperf3-server",
		Protocol: "tcp",
		Adaptor:  common.AdaptorTCP,
		Port:     5201,
	}
}

func getIperfClientInfo() *common.ClientInfo {
	return &common.ClientInfo{
		Name:      "iperf3-client",
		Resources: getIperfResources(),
		Settings:  getIperfClientSettings(),
		Jobs:      getIperfJobs(),
		Timeout:   iperfSettings.JobTimeout,
	}
}

func getIperfJobs() []common.JobInfo {
	var jobs []common.JobInfo
	clients := iperfSettings.ParallelClients
	sizes := iperfSettings.TransmitSizes
	for _, client := range clients {
		for _, size := range sizes {
			jobName := strings.ToLower(fmt.Sprintf("iperf3-clients-%d-size-%s", client, size))
			clientJob := k8s.NewJob(jobName, "", k8s.JobOpts{
				Image:        IPERF_IMAGE,
				BackoffLimit: 10,
				Restart:      v1.RestartPolicyNever,
				Labels:       map[string]string{"job": jobName},
				Args:         iperfSettings.ToArgs("iperf3-server", size, client),
			})
			jobs = append(jobs, common.JobInfo{
				Name:    jobName,
				Clients: client,
				Job:     clientJob,
			})
		}
	}
	return jobs
}

func getIperfClientSettings() common.AppSettings {
	return iperfSettings.Env
}

func parseIperfSettings() {
	iperfSettings = IperfTuning{
		Env: map[string]string{},
	}

	// IPERF_PARALLEL_CLIENTS
	var iperfParallelClients []int
	for _, parallelClientStr := range strings.Split(iperfSettings.Env.AddEnvVar(ENV_IPERF_PARALLEL_CLIENTS, "1"), ",") {
		parallelClients, err := strconv.Atoi(parallelClientStr)
		if err != nil {
			iperfParallelClients = []int{1}
			log.Printf("invalid value for IPERF_PARALLEL_CLIENTS (int expected): %s - using default: 1", parallelClientStr)
			break
		}
		iperfParallelClients = append(iperfParallelClients, parallelClients)
	}
	iperfSettings.ParallelClients = iperfParallelClients

	// IPERF_TRANSMIT_SIZES
	iperfSettings.TransmitSizes = strings.Split(iperfSettings.Env.AddEnvVar(ENV_IPERF_TRANSMIT_SIZES, "1G"), ",")

	// IPERF_WINDOW_SIZE
	iperfWindowSize, err := strconv.Atoi(iperfSettings.Env.AddEnvVar(ENV_IPERF_WINDOW_SIZE, "0"))
	if err != nil {
		log.Printf("invalid value for IPERF_WINDOW_SIZE (int expected): %s - using default: 0", os.Getenv(ENV_IPERF_WINDOW_SIZE))
		iperfWindowSize = 0
	}
	iperfSettings.WindowSize = iperfWindowSize

	// IPERF_MEMORY
	iperfSettings.Memory = iperfSettings.Env.AddEnvVar(ENV_IPERF_MEMORY, "")
	// IPERF_CPU
	iperfSettings.Cpu = iperfSettings.Env.AddEnvVar(ENV_IPERF_CPU, "")

	// IPERF_JOB_TIMEOUT
	iperfJobTimeout := iperfSettings.Env.AddEnvVar(ENV_IPERF_JOB_TIMEOUT, "10m")
	jobTimeout, err := time.ParseDuration(iperfJobTimeout)
	if err != nil {
		log.Printf("invalid value for IPERF_JOB_TIMEOUT: %s - using default: 10m", iperfJobTimeout)
		jobTimeout = time.Minute * 10
	}
	iperfSettings.JobTimeout = jobTimeout
}
