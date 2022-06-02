//go:build integration || performance
// +build integration performance

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
	"time"

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
	ENV_AMQP_DURATION_SECS = "AMQP_DURATION_SECS"
	ENV_AMQP_TIMEOUT       = "AMQP_TIMEOUT"
	ENV_AMQP_CPU           = "AMQP_CPU"
	ENV_AMQP_MEMORY        = "AMQP_MEMORY"
)

type AmqpTest common.PerformanceApp

type amqpSettings struct {
	durationSecs int
	cpu          string
	memory       string
	jobTimeout   time.Duration
	env          common.AppSettings
}

func TestAmqp(t *testing.T) {
	settings := parseAmqpSettings()
	a := &AmqpTest{
		Name:        "amqp",
		Description: "AMQP performance test using quiver and dispatch router",
		Service: common.ServiceInfo{
			Address:  "amqp-server",
			Protocol: "tcp",
			Adaptor:  common.AdaptorTCP,
			Port:     5672,
		},
		Server:         getAmqpServerInfo(settings),
		Client:         getAmqpClientInfo(settings),
		ThroughputUnit: common.ThroughputUnitMsgs,
		LatencyUnit:    common.LatencyUnitMs,
	}
	assert.Assert(t, common.RunPerformanceTest(a))
}

func (p *AmqpTest) App() common.PerformanceApp {
	return common.PerformanceApp(*p)
}

func (p *AmqpTest) Validate(serverCluster, clientCluster *base.ClusterContext, job common.JobInfo) common.Result {
	res := common.Result{}

	log.Println("validating client result")
	// Saving job logs
	logs, err := k8s.GetJobLogs(clientCluster.Namespace, clientCluster.VanClient.KubeClient, job.Name)
	if err != nil {
		res.SetError(err)
		return res
	}
	buf := bytes.NewBufferString(logs)

	throughputRegex, _ := regexp.Compile(`^End-to-end rate\s\.+\s(\d+(,\d+)*) messages/s`)
	latencyAvgRegexp, _ := regexp.Compile(`^(\s*\S+){10}\s+(\d+)\s*$`)
	latency50Regexp, _ := regexp.Compile(`50% \.* (\d+) ms\s+99.90%`)
	latency99Regexp, _ := regexp.Compile(` ms\s+99.00% \.* (\d+) ms`)
	var line string
	var latSum float64
	var latCount int
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
		if latencyAvgRegexp.MatchString(line) {
			match := latencyAvgRegexp.FindStringSubmatch(line)
			lat, err := strconv.Atoi(match[2])
			if err != nil {
				res.SetError(fmt.Errorf("error parsing latency average: %v", err))
				return res
			}
			latSum += float64(lat)
			latCount++
		} else if latency50Regexp.MatchString(line) {
			match := latency50Regexp.FindStringSubmatch(line)
			res.Latency50, err = strconv.ParseFloat(match[1], 64)
			if err != nil {
				res.SetError(fmt.Errorf("error parsing latency 50%%: %v", err))
				return res
			}
		} else if latency99Regexp.MatchString(line) {
			match := latency99Regexp.FindStringSubmatch(line)
			res.Latency99, err = strconv.ParseFloat(match[1], 64)
			if err != nil {
				res.SetError(fmt.Errorf("error parsing latency 99%%: %v", err))
				return res
			}
		} else if throughputRegex.MatchString(line) {
			match := throughputRegex.FindStringSubmatch(line)
			res.Throughput, err = strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
			if err != nil {
				res.SetError(fmt.Errorf("error parsing throughput: %v", err))
				return res
			}
		}
	}
	res.LatencyAvg = latSum / float64(latCount)
	return res
}

func parseAmqpSettings() *amqpSettings {
	settings := &amqpSettings{
		env: map[string]string{},
	}

	// duration
	durationStr := settings.env.AddEnvVar(ENV_AMQP_DURATION_SECS, "30")
	duration, err := strconv.Atoi(durationStr)
	if err != nil {
		duration = 30
		log.Printf("invalid duration: %s - using default %d", durationStr, duration)
	}
	settings.durationSecs = duration

	// memory
	settings.memory = settings.env.AddEnvVar(ENV_AMQP_MEMORY, "")
	// cpu
	settings.cpu = settings.env.AddEnvVar(ENV_AMQP_CPU, "")

	// timeout
	timeout := settings.env.AddEnvVar(ENV_AMQP_TIMEOUT, "10m")
	jobTimeout, err := time.ParseDuration(timeout)
	if err != nil {
		log.Printf("invalid value for %s: %v", ENV_AMQP_TIMEOUT, err)
		log.Printf("the default timeout will be used: 10m")
		jobTimeout = time.Minute * 10
	}
	settings.jobTimeout = jobTimeout

	return settings
}

func getAmqpClientInfo(settings *amqpSettings) *common.ClientInfo {
	cli := &common.ClientInfo{
		Name: "quiver",
		Resources: common.ResourceSettings{
			Memory: settings.memory,
			CPU:    settings.cpu,
		},
		Settings: settings.env,
		Timeout:  settings.jobTimeout,
		Jobs:     getAmqpJobs(settings),
	}
	return cli
}

func getAmqpJobs(settings *amqpSettings) []common.JobInfo {
	var jobs []common.JobInfo
	image := "ssorj/quiver"
	jobName := fmt.Sprintf("quiver")
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
						Name: "quiver", Image: image,
						Command: []string{"quiver", "-d", strconv.Itoa(settings.durationSecs), "amqp://amqp-server:5672/quiver"},
					}}, RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
	jobs = append(jobs, common.JobInfo{
		Name:    jobName,
		Clients: 1,
		Job:     job,
	})
	return jobs
}

func getAmqpServerInfo(settings *amqpSettings) *common.ServerInfo {
	return &common.ServerInfo{
		Name: "amqp-server",
		Resources: common.ResourceSettings{
			Memory: settings.memory,
			CPU:    settings.cpu,
		},
		Settings:   settings.env,
		Deployment: getAmqpDeployment(),
	}
}

func getAmqpDeployment() *appsv1.Deployment {
	dep, _ := k8s.NewDeployment("amqp-server", "", k8s.DeploymentOpts{
		Image:  "quay.io/skupper/skupper-router:main",
		Labels: map[string]string{"app": "amqp-server"},
	})
	return dep
}
