//go:build performance
// +build performance

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
	"github.com/skupperproject/skupper/test/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ENV_HTTP_DURATION = "HTTP_DURATION_SECS"
	ENV_HTTP_CLIENTS  = "HTTP_PARALLEL_CLIENTS"
	ENV_HTTP_TIMEOUT  = "HTTP_TIMEOUT"
	ENV_HTTP_CPU      = "HTTP_CPU"
	ENV_HTTP_MEMORY   = "HTTP_MEMORY"
)

type HttpTest common.PerformanceApp

type httpSettings struct {
	duration   int
	clients    []int
	cpu        string
	memory     string
	jobTimeout time.Duration
	env        common.AppSettings
}

func TestHttp(t *testing.T) {
	settings := parseHttpSettings()
	a := &HttpTest{
		Name:        "http",
		Description: "Http performance test through HTTP adaptor",
		Service: common.ServiceInfo{
			Address:  "http-server",
			Protocol: "tcp",
			Adaptor:  common.AdaptorHTTP,
			Port:     8080,
		},
		Server:         getHttpServerInfo(settings),
		Client:         getHttpClientInfo(settings, "http-server"),
		ThroughputUnit: common.ThroughputUnitReqs,
		LatencyUnit:    common.LatencyUnitMs,
	}
	assert.Assert(t, common.RunPerformanceTest(a))
}

func TestHttpOverTcp(t *testing.T) {
	settings := parseHttpSettings()
	a := &HttpTest{
		Name:        "http-tcp",
		Description: "Http performance test through TCP adaptor",
		Service: common.ServiceInfo{
			Address:  "http-server-tcp",
			Protocol: "tcp",
			Adaptor:  common.AdaptorTCP,
			Port:     8080,
		},
		Server:         getHttpServerInfo(settings),
		Client:         getHttpClientInfo(settings, "http-server-tcp"),
		ThroughputUnit: common.ThroughputUnitReqs,
		LatencyUnit:    common.LatencyUnitMs,
	}
	assert.Assert(t, common.RunPerformanceTest(a))
}

func (p *HttpTest) App() common.PerformanceApp {
	return common.PerformanceApp(*p)
}

func (p *HttpTest) Validate(serverCluster, clientCluster *base.ClusterContext, job common.JobInfo) common.Result {
	var res common.Result
	if strings.Contains(job.Name, "wrk") {
		res = validateWrk(serverCluster, clientCluster, job)
	} else if strings.Contains(job.Name, "hey") {
		res = validateHey(serverCluster, clientCluster, job)
	} else {
		res.SetError(fmt.Errorf("unable to parse http client result for job: %s", job.Name))
	}
	return res
}

func validateWrk(serverCluster *base.ClusterContext, clientCluster *base.ClusterContext, job common.JobInfo) common.Result {
	res := common.Result{}

	log.Println("validating wrk client result")
	// Saving job logs
	logs, err := k8s.GetJobLogs(clientCluster.Namespace, clientCluster.VanClient.KubeClient, job.Name)
	if err != nil {
		res.SetError(err)
		return res
	}
	buf := bytes.NewBufferString(logs)

	throughputRegex, _ := regexp.Compile(`^Requests/sec:\s+(\d+\.\d+)\s*$`)
	latencyAvgRegex, _ := regexp.Compile(`\s+Latency\s+(\d+\.\d+)ms`)
	latency50Regex, _ := regexp.Compile(`\s+50%\s+(\d+\.\d+)ms`)
	latency99Regex, _ := regexp.Compile(`\s+99%\s+(\d+\.\d+)ms`)

	var line string
	for {
		line, err = buf.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				res.SetError(fmt.Errorf("error parsing results: %v", err))
				return res
			} else {
				break
			}
		}
		if throughputRegex.MatchString(line) {
			match := throughputRegex.FindStringSubmatch(line)
			if res.Throughput, err = strconv.ParseFloat(match[1], 64); err != nil {
				res.SetError(fmt.Errorf("error parsing throughput: %v", err))
				return res
			}
		} else if latencyAvgRegex.MatchString(line) {
			match := latencyAvgRegex.FindStringSubmatch(line)
			if res.LatencyAvg, err = strconv.ParseFloat(match[1], 64); err != nil {
				res.SetError(fmt.Errorf("error parsing latency average: %v", err))
				return res
			}
		} else if latency50Regex.MatchString(line) {
			match := latency50Regex.FindStringSubmatch(line)
			if res.Latency50, err = strconv.ParseFloat(match[1], 64); err != nil {
				res.SetError(fmt.Errorf("error parsing latency 50%%: %v", err))
				return res
			}
		} else if latency99Regex.MatchString(line) {
			match := latency99Regex.FindStringSubmatch(line)
			if res.Latency99, err = strconv.ParseFloat(match[1], 64); err != nil {
				res.SetError(fmt.Errorf("error parsing latency 99%%: %v", err))
				return res
			}
		}
	}

	return res
}

func validateHey(serverCluster *base.ClusterContext, clientCluster *base.ClusterContext, job common.JobInfo) common.Result {
	res := common.Result{}

	log.Println("validating hey client result")
	// Saving job logs
	logs, err := k8s.GetJobLogs(clientCluster.Namespace, clientCluster.VanClient.KubeClient, job.Name)
	if err != nil {
		res.SetError(err)
		return res
	}
	buf := bytes.NewBufferString(logs)

	throughputRegex, _ := regexp.Compile(`\s+Requests/sec:\s+(\d+\.\d+)\s*$`)
	latencyAvgRegex, _ := regexp.Compile(`\s+Average:\s+(\d+\.\d+) secs`)
	latency50Regex, _ := regexp.Compile(`\s+50% in (\d+\.\d+) secs`)
	latency99Regex, _ := regexp.Compile(`\s+99% in (\d+\.\d+) secs`)

	var line string
	for {
		line, err = buf.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				res.SetError(fmt.Errorf("error parsing results: %v", err))
				return res
			} else {
				break
			}
		}
		if throughputRegex.MatchString(line) {
			match := throughputRegex.FindStringSubmatch(line)
			if res.Throughput, err = strconv.ParseFloat(match[1], 64); err != nil {
				res.SetError(fmt.Errorf("error parsing throughput: %v", err))
				return res
			}
		} else if latencyAvgRegex.MatchString(line) {
			match := latencyAvgRegex.FindStringSubmatch(line)
			if res.LatencyAvg, err = strconv.ParseFloat(match[1], 64); err != nil {
				res.SetError(fmt.Errorf("error parsing latency average: %v", err))
				return res
			}
			res.LatencyAvg = res.LatencyAvg * 1000
		} else if latency50Regex.MatchString(line) {
			match := latency50Regex.FindStringSubmatch(line)
			if res.Latency50, err = strconv.ParseFloat(match[1], 64); err != nil {
				res.SetError(fmt.Errorf("error parsing latency 50%%: %v", err))
				return res
			}
			res.Latency50 = res.Latency50 * 1000
		} else if latency99Regex.MatchString(line) {
			match := latency99Regex.FindStringSubmatch(line)
			if res.Latency99, err = strconv.ParseFloat(match[1], 64); err != nil {
				res.SetError(fmt.Errorf("error parsing latency 99%%: %v", err))
				return res
			}
			res.Latency99 = res.Latency99 * 1000
		}
	}

	return res
}

func parseHttpSettings() *httpSettings {
	settings := &httpSettings{
		env: map[string]string{},
	}

	// duration
	duration, err := strconv.Atoi(utils.StrDefault("30", os.Getenv(ENV_HTTP_DURATION)))
	if err != nil {
		duration = 30
	}
	settings.duration = duration
	settings.env.AddEnvVar(ENV_HTTP_DURATION)

	// parsing parallel clients
	var parallelClients []int
	for _, parallelClientStr := range strings.Split(utils.StrDefault("10", os.Getenv(ENV_HTTP_CLIENTS)), ",") {
		clients, err := strconv.Atoi(parallelClientStr)
		if err != nil {
			log.Printf("invalid value for %s (int csv expected) - default will be used: 10", ENV_HTTP_CLIENTS)
			clients = 10
		}
		parallelClients = append(parallelClients, clients)
	}
	settings.clients = parallelClients
	settings.env.AddEnvVar(ENV_HTTP_CLIENTS)

	// memory
	settings.memory = os.Getenv(ENV_HTTP_MEMORY)
	settings.env.AddEnvVar(ENV_HTTP_MEMORY)

	// cpu
	settings.cpu = os.Getenv(ENV_HTTP_CPU)
	settings.env.AddEnvVar(ENV_HTTP_CPU)

	// timeout
	timeout := utils.StrDefault("10m", os.Getenv(ENV_HTTP_TIMEOUT))
	jobTimeout, err := time.ParseDuration(timeout)
	if err != nil {
		log.Printf("invalid value for %s: %v", ENV_HTTP_TIMEOUT, err)
		log.Printf("the default timeout will be used: 10m")
		jobTimeout = time.Minute * 10
	}
	settings.jobTimeout = jobTimeout
	settings.env.AddEnvVar(ENV_HTTP_TIMEOUT)

	return settings
}

func getHttpClientInfo(settings *httpSettings, serviceName string) *common.ClientInfo {
	cli := &common.ClientInfo{
		Name: "http-client",
		Resources: common.ResourceSettings{
			Memory: settings.memory,
			CPU:    settings.cpu,
		},
		Settings: settings.env,
		Timeout:  settings.jobTimeout,
		Jobs:     getHttpJobs(settings, serviceName),
	}
	return cli
}

func getHttpJobs(settings *httpSettings, serviceName string) []common.JobInfo {
	var jobs []common.JobInfo
	imageWrk := "quay.io/skupper/wrk"
	imageHey := "quay.io/skupper/hey"
	url := fmt.Sprintf("http://%s:8080", serviceName)
	jobPrefix := "http"
	if strings.Contains(serviceName, "tcp") {
		jobPrefix += "-tcp"
	}
	for _, clients := range settings.clients {
		// wrk job
		jobWrkName := fmt.Sprintf("%s-wrk-clients-%d", jobPrefix, clients)
		labelsWrk := map[string]string{"job": jobWrkName}
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: jobWrkName, Labels: labelsWrk},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Name: jobWrkName, Labels: labelsWrk},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: "http-client-wrk", Image: imageWrk,
							Args: []string{"wrk", "-d", strconv.Itoa(settings.duration) + "s",
								"-c", strconv.Itoa(clients), "--latency", url},
						}}, RestartPolicy: corev1.RestartPolicyNever,
					},
				},
			},
		}
		jobs = append(jobs, common.JobInfo{
			Name:    jobWrkName,
			Clients: clients,
			Job:     job,
		})

		// hey job
		jobHeyName := fmt.Sprintf("%s-hey-clients-%d", jobPrefix, clients)
		labelsHey := map[string]string{"job": jobHeyName}
		job = &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: jobHeyName, Labels: labelsHey},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Name: jobHeyName, Labels: labelsHey},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: "http-client-hey", Image: imageHey,
							Args: []string{"-z", strconv.Itoa(settings.duration) + "s",
								"-c", strconv.Itoa(clients), url},
						}}, RestartPolicy: corev1.RestartPolicyNever,
					},
				},
			},
		}
		jobs = append(jobs, common.JobInfo{
			Name:    jobHeyName,
			Clients: clients,
			Job:     job,
		})

	}
	return jobs
}

func getHttpServerInfo(settings *httpSettings) *common.ServerInfo {
	return &common.ServerInfo{
		Name: "http-server",
		Resources: common.ResourceSettings{
			Memory: settings.memory,
			CPU:    settings.cpu,
		},
		Settings:   settings.env,
		Deployment: getHttpDeployment(),
	}
}

func getHttpDeployment() *appsv1.Deployment {
	dep, _ := k8s.NewDeployment("http-server", "", k8s.DeploymentOpts{
		Image:  "nginxinc/nginx-unprivileged:stable-alpine",
		Labels: map[string]string{"app": "http-server"},
	})
	return dep
}
