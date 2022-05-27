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

	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/integration/performance/common"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ENV_REDIS_REQUESTS = "REDIS_NUMBER_REQUESTS"
	ENV_REDIS_CLIENTS  = "REDIS_PARALLEL_CLIENTS"
	ENV_REDIS_TESTS    = "REDIS_TESTS"
	ENV_REDIS_DATASIZE = "REDIS_DATASIZE"
	ENV_REDIS_TIMEOUT  = "REDIS_TIMEOUT"
	ENV_REDIS_CPU      = "REDIS_CPU"
	ENV_REDIS_MEMORY   = "REDIS_MEMORY"
)

var (
	redisDfltRequests = "25000"
	redisDfltTests    = []string{"PING_INLINE", "PING_MBULK", "SET", "GET", "INCR", "LPUSH", "RPUSH", "LPOP", "RPOP",
		"SADD", "HSET", "SPOP", "ZADD", "ZPOPMIN", "LPUSH", "LRANGE_100", "LRANGE_300", "LRANGE_500", "LRANGE_600", "MSET"}
)

type RedisTest common.PerformanceApp

type redisSettings struct {
	requests   int
	clients    []int
	tests      []string
	dataSize   int
	cpu        string
	memory     string
	jobTimeout time.Duration
	env        common.AppSettings
}

func TestRedis(t *testing.T) {
	settings := parseRedisSettings()
	a := &RedisTest{
		Name:        "redis",
		Description: "Redis performance test using redis-benchmark",
		Service: common.ServiceInfo{
			Address:  "redis-server",
			Protocol: "tcp",
			Adaptor:  common.AdaptorTCP,
			Port:     6379,
		},
		Server:         getRedisServerInfo(settings),
		Client:         getRedisClientInfo(settings),
		ThroughputUnit: common.ThroughputUnitMsgs,
		LatencyUnit:    common.LatencyUnitMs,
	}
	assert.Assert(t, common.RunPerformanceTest(a))
}

func (p *RedisTest) App() common.PerformanceApp {
	return common.PerformanceApp(*p)
}

func (p *RedisTest) Validate(serverCluster, clientCluster *base.ClusterContext, job common.JobInfo) common.Result {
	res := common.Result{}

	log.Println("validating client result")
	// Saving job logs
	logs, err := k8s.GetJobLogs(clientCluster.Namespace, clientCluster.VanClient.KubeClient, job.Name)
	if err != nil {
		res.SetError(err)
		return res
	}
	buf := bytes.NewBufferString(logs)

	throughputRegex, _ := regexp.Compile(`throughput summary: (\d+\.\d+) requests per second`)
	latencyRegex, _ := regexp.Compile(`\s+(\d+\.\d+)\s+\d+\.\d+\s+(\d+\.\d+)\s+\d+\.\d+\s+(\d+\.\d+)\s+\d+\.\d+`)

	var line string
	for {
		line, err = buf.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				res.SetError(fmt.Errorf("error reading logs: %v", err))
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
		} else if latencyRegex.MatchString(line) {
			match := latencyRegex.FindStringSubmatch(line)
			if res.LatencyAvg, err = strconv.ParseFloat(match[1], 64); err != nil {
				res.SetError(fmt.Errorf("error parsing latency average: %v", err))
				return res
			}
			if res.Latency50, err = strconv.ParseFloat(match[2], 64); err != nil {
				res.SetError(fmt.Errorf("error parsing latency 50%%: %v", err))
				return res
			}
			if res.Latency99, err = strconv.ParseFloat(match[3], 64); err != nil {
				res.SetError(fmt.Errorf("error parsing latency 99%%: %v", err))
				return res
			}
		}
	}

	return res
}

func parseRedisSettings() *redisSettings {
	settings := &redisSettings{
		env: map[string]string{},
	}

	// requests
	if common.DebugMode() {
		redisDfltRequests = "1000"
		redisDfltTests = []string{"MSET"}
	}
	requests, err := strconv.Atoi(settings.env.AddEnvVar(ENV_REDIS_REQUESTS, redisDfltRequests))
	if err != nil {
		requests, _ = strconv.Atoi(redisDfltRequests)
		log.Printf("invalid value for requests: %s - using default: %d", os.Getenv(ENV_REDIS_REQUESTS), requests)
	}
	settings.requests = requests

	// parsing parallel clients
	var parallelClients []int
	for _, parallelClientStr := range strings.Split(settings.env.AddEnvVar(ENV_REDIS_CLIENTS, "50"), ",") {
		clients, err := strconv.Atoi(parallelClientStr)
		if err != nil {
			log.Printf("invalid value for %s (int csv expected): %s - default will be used: 50", ENV_REDIS_CLIENTS, os.Getenv(ENV_REDIS_CLIENTS))
			parallelClients = []int{50}
			break
		}
		parallelClients = append(parallelClients, clients)
	}
	settings.clients = parallelClients

	// parsing redis tests to run
	if os.Getenv(ENV_REDIS_TESTS) == "" {
		settings.tests = redisDfltTests
		settings.env.AddEnvVar(ENV_REDIS_TESTS, strings.Join(redisDfltTests, ","))
	} else {
		settings.tests = strings.Split(settings.env.AddEnvVar(ENV_REDIS_TESTS, ""), ",")
		for _, testName := range settings.tests {
			if !utils.StringSliceContains(redisDfltTests, testName) {
				log.Printf("invalid redis test name: %s - using default list: %v", testName, redisDfltTests)
				settings.tests = redisDfltTests
				break
			}
		}
	}

	// data size (for GET/SET tests)
	dataSize := settings.env.AddEnvVar(ENV_REDIS_DATASIZE, "3")
	if settings.dataSize, err = strconv.Atoi(dataSize); err != nil {
		settings.dataSize = 3
		log.Printf("invalid data size: %s - using default 3", dataSize)
	}

	// memory
	settings.memory = settings.env.AddEnvVar(ENV_REDIS_MEMORY, "")
	// cpu
	settings.cpu = settings.env.AddEnvVar(ENV_REDIS_CPU, "")

	// timeout
	timeout := settings.env.AddEnvVar(ENV_REDIS_TIMEOUT, "10m")
	jobTimeout, err := time.ParseDuration(timeout)
	if err != nil {
		log.Printf("invalid value for %s: %s - %v", ENV_REDIS_TIMEOUT, os.Getenv(ENV_REDIS_TIMEOUT), err)
		log.Printf("the default timeout will be used: 10m")
		jobTimeout = time.Minute * 10
	}
	settings.jobTimeout = jobTimeout

	return settings
}

func getRedisClientInfo(settings *redisSettings) *common.ClientInfo {
	cli := &common.ClientInfo{
		Name: "redis-benchmark",
		Resources: common.ResourceSettings{
			Memory: settings.memory,
			CPU:    settings.cpu,
		},
		Settings: settings.env,
		Timeout:  settings.jobTimeout,
		Jobs:     getRedisJobs(settings),
	}
	return cli
}

func getRedisJobs(settings *redisSettings) []common.JobInfo {
	var jobs []common.JobInfo
	var testCount int
	var found bool
	image := "redis"
	for _, clients := range settings.clients {
		testNameCount := map[string]int{}
		for _, testName := range settings.tests {
			jobTestName := strings.ToLower(testName)
			jobTestName = strings.ReplaceAll(jobTestName, "_", "")
			if testCount, found = testNameCount[jobTestName]; found {
				testCount++
				testNameCount[jobTestName] = testCount
				jobTestName += "-" + strconv.Itoa(testCount)
			} else {
				testNameCount[jobTestName] = 1
				jobTestName += "-1"
			}
			jobName := fmt.Sprintf("redis-benchmark-%s-clients-%d", jobTestName, clients)
			labels := map[string]string{"job": jobName}
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:   jobName,
					Labels: labels,
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name:   jobName,
							Labels: labels,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name: "redis-benchmark", Image: image,
								Command: []string{"redis-benchmark", "-n", strconv.Itoa(settings.requests),
									"-c", strconv.Itoa(clients), "-t", testName, "-d", strconv.Itoa(settings.dataSize),
									"-h", "redis-server"},
							}}, RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			}
			jobs = append(jobs, common.JobInfo{
				Name:    jobName,
				Clients: clients,
				Job:     job,
			})
		}
	}
	return jobs
}

func getRedisServerInfo(settings *redisSettings) *common.ServerInfo {
	return &common.ServerInfo{
		Name: "redis-server",
		Resources: common.ResourceSettings{
			Memory: settings.memory,
			CPU:    settings.cpu,
		},
		Settings:   settings.env,
		Deployment: getRedisDeployment(),
		PostInitCommands: [][]string{
			{"redis-cli", "config", "set", "stop-writes-on-bgsave-error", "no"},
		},
	}
}

func getRedisDeployment() *appsv1.Deployment {
	dep, _ := k8s.NewDeployment("redis-server", "", k8s.DeploymentOpts{
		Image:  "redis",
		Labels: map[string]string{"app": "redis-server"},
	})
	return dep
}
