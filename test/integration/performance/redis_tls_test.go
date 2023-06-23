//go:build integration || performance
// +build integration performance

// This is a copy of the Redis test, adapted for services that are
// configured with --generate-tls-secrets
package performance

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/test/integration/performance/common"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RedisTestTls common.PerformanceApp

func TestRedisTls(t *testing.T) {
	settings := parseRedisSettings()
	a := &RedisTest{
		Name:        "redis-tls",
		Description: "Redis performance test using redis-benchmark with TLS",
		Service: common.ServiceInfo{
			Address:  "redis-server-tls",
			Protocol: "tcp",
			Adaptor:  common.AdaptorTCP,
			Port:     6379,
		},
		Server:           getRedisTlsServerInfo(settings),
		Client:           getRedisTlsClientInfo(settings),
		ThroughputUnit:   common.ThroughputUnitMsgs,
		LatencyUnit:      common.LatencyUnitMs,
		TlsCredentials:   "skupper-tls-redis-server-tls",
		TlsCertAuthority: "skupper-service-client",
	}
	assert.Assert(t, common.RunPerformanceTest(a))
}

func (p *RedisTestTls) App() common.PerformanceApp {
	return common.PerformanceApp(*p)
}

func (p *RedisTestTls) Validate(serverCluster, clientCluster *base.ClusterContext, job common.JobInfo) common.Result {
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

func getRedisTlsJobs(settings *redisSettings) []common.JobInfo {
	var jobs []common.JobInfo
	var testCount int
	var found bool
	image := "quay.io/dhashimo/redis"
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
			jobName := fmt.Sprintf("redis-tls-benchmark-%s-clients-%d", jobTestName, clients)
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
							Volumes: []corev1.Volume{
								{
									Name: "volume-cert",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "skupper-service-client",
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Name: "redis-benchmark", Image: image,
									Command: []string{
										"redis-benchmark",
										"-n", strconv.Itoa(settings.requests),
										"-c", strconv.Itoa(clients),
										"-t", testName,
										"-d", strconv.Itoa(settings.dataSize),
										"-h", "redis-server-tls",
										"--tls", "--cacert", "/cert/ca.crt",
										// We have no client certs
										// "--cert", "/cert/tls.crt",
										// "--key", "/cert/tls.key",
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "volume-cert",
											MountPath: "/cert",
										},
									},
								},
							},
							RestartPolicy: corev1.RestartPolicyNever,
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

func getRedisTlsClientInfo(settings *redisSettings) *common.ClientInfo {
	cli := &common.ClientInfo{
		Name: "redis-benchmark",
		Resources: common.ResourceSettings{
			Memory: settings.memory,
			CPU:    settings.cpu,
		},
		Settings: settings.env,
		Timeout:  settings.jobTimeout,
		Jobs:     getRedisTlsJobs(settings),
	}
	return cli
}

func getRedisTlsServerInfo(settings *redisSettings) *common.ServerInfo {
	return &common.ServerInfo{
		Name: "redis-server-tls",
		Resources: common.ResourceSettings{
			Memory: settings.memory,
			CPU:    settings.cpu,
		},
		Settings:   settings.env,
		Deployment: getRedisTlsDeployment(),
	}
}

func getRedisTlsDeployment() *appsv1.Deployment {
	dep, _ := k8s.NewDeployment(
		"redis-server-tls",
		"",
		k8s.DeploymentOpts{
			Image:  "quay.io/dhashimo/redis",
			Labels: map[string]string{"app": "redis-server-tls"},
			Args: []string{
				"redis-server",
				"--tls-auth-clients", "no", // We have no client certs
				"--tls-ca-cert-file", "/cert/ca.crt",
				"--tls-cert-file", "/cert/tls.crt",
				"--tls-key-file", "/cert/tls.key",
				"--tls-port", "6379", "--port", "0",
				"--stop-writes-on-bgsave-error", "no",
			},
			SecretMounts: []k8s.SecretMount{
				{
					Name:      "server-cert",
					MountPath: "/cert",
					Secret:    "skupper-tls-redis-server-tls",
				}, {
					// this is not necessary for the test, but helps debugging
					Name:      "client-cert",
					MountPath: "/client-cert",
					Secret:    "skupper-service-client",
				},
			},
		},
	)
	return dep
}
