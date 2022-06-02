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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ENV_POSTGRES_DURATION_SECS    = "POSTGRES_DURATION_SECS"
	ENV_POSTGRES_PARALLEL_CLIENTS = "POSTGRES_PARALLEL_CLIENTS"
	ENV_POSTGRES_TIMEOUT          = "POSTGRES_TIMEOUT"
	ENV_POSTGRES_CPU              = "POSTGRES_CPU"
	ENV_POSTGRES_MEMORY           = "POSTGRES_MEMORY"
)

type PostgresTest common.PerformanceApp

type postgresSettings struct {
	durationSecs int
	clients      []int
	cpu          string
	memory       string
	jobTimeout   time.Duration
	env          common.AppSettings
}

func TestPostgres(t *testing.T) {
	settings := parsePostgresSettings()
	p := &PostgresTest{
		Name:        "postgres",
		Description: "Postgres performance test using pgbench",
		Service: common.ServiceInfo{
			Address:  "postgres-server",
			Protocol: "tcp",
			Adaptor:  common.AdaptorTCP,
			Port:     5432,
		},
		Server:         getPostgresServerInfo(settings),
		Client:         getPostgresClientInfo(settings),
		ThroughputUnit: common.ThroughputUnitTps,
		LatencyUnit:    common.LatencyUnitMs,
	}
	assert.Assert(t, common.RunPerformanceTest(p))
}

func (p *PostgresTest) App() common.PerformanceApp {
	return common.PerformanceApp(*p)
}

func (p *PostgresTest) Validate(serverCluster, clientCluster *base.ClusterContext, job common.JobInfo) common.Result {
	res := common.Result{}

	log.Println("validating client result")
	// Saving job logs
	logs, err := k8s.GetJobLogs(clientCluster.Namespace, clientCluster.VanClient.KubeClient, job.Name)
	if err != nil {
		res.SetError(err)
		return res
	}

	latencyRegexp, _ := regexp.Compile(`^latency average = (\S+) ms`)
	tpsRegexp, _ := regexp.Compile(`^tps = (\S+) .*`)
	buf := bytes.NewBufferString(logs)
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
		if latencyRegexp.MatchString(line) {
			match := latencyRegexp.FindStringSubmatch(line)
			lat, err := strconv.ParseFloat(match[1], 64)
			if err == nil {
				log.Printf("latency = %.2f ms", lat)
				res.LatencyAvg = lat
				res.Latency50 = lat
				res.Latency99 = lat
			} else {
				res.SetError(fmt.Errorf("error parsing latency: %v", err))
				return res
			}
		}
		if tpsRegexp.MatchString(line) {
			log.Printf("tps match = %s", line)
			match := tpsRegexp.FindStringSubmatch(line)
			tps, err := strconv.ParseFloat(match[1], 64)
			if err == nil {
				log.Printf("throughput = %.2f tps", tps)
				res.Throughput = tps
			} else {
				res.SetError(fmt.Errorf("error parsing throughput: %v", err))
				return res
			}
		}
	}
	return res
}

func parsePostgresSettings() *postgresSettings {
	settings := &postgresSettings{
		env: map[string]string{},
	}

	// parsing parallel clients
	var parallelClients []int
	for _, parallelClientStr := range strings.Split(settings.env.AddEnvVar(ENV_POSTGRES_PARALLEL_CLIENTS, "1"), ",") {
		clients, err := strconv.Atoi(parallelClientStr)
		if err != nil {
			log.Printf("invalid value for %s (int csv expected): %s - default will be used: 1", ENV_POSTGRES_PARALLEL_CLIENTS, os.Getenv(ENV_POSTGRES_PARALLEL_CLIENTS))
			parallelClients = []int{1}
			break
		}
		parallelClients = append(parallelClients, clients)
	}
	settings.clients = parallelClients

	// duration
	var durationStr string
	if os.Getenv(ENV_POSTGRES_DURATION_SECS) == "" && common.DebugMode() {
		durationStr = "5"
	} else {
		durationStr = settings.env.AddEnvVar(ENV_POSTGRES_DURATION_SECS, "30")
	}
	duration, err := strconv.Atoi(durationStr)
	if err != nil {
		duration = 30
		log.Printf("invalid duration: %s - using default: %d", durationStr, duration)
	}
	settings.durationSecs = duration

	// memory
	settings.memory = settings.env.AddEnvVar(ENV_POSTGRES_MEMORY, "")
	// cpu
	settings.cpu = settings.env.AddEnvVar(ENV_POSTGRES_CPU, "")

	// timeout
	timeout := settings.env.AddEnvVar(ENV_POSTGRES_TIMEOUT, "10m")
	jobTimeout, err := time.ParseDuration(timeout)
	if err != nil {
		log.Printf("invalid value for %s: %s - %v", ENV_POSTGRES_TIMEOUT, os.Getenv(ENV_POSTGRES_TIMEOUT), err)
		log.Printf("the default timeout will be used: 10m")
		jobTimeout = time.Minute * 10
	}
	settings.jobTimeout = jobTimeout

	return settings
}

func getPostgresClientInfo(settings *postgresSettings) *common.ClientInfo {
	cli := &common.ClientInfo{
		Name: "pgbench",
		Resources: common.ResourceSettings{
			Memory: settings.memory,
			CPU:    settings.cpu,
		},
		Settings: settings.env,
		Timeout:  settings.jobTimeout,
		Jobs:     getPostgresJobs(settings),
	}
	return cli
}

func getPostgresJobs(settings *postgresSettings) []common.JobInfo {
	var jobs []common.JobInfo
	image := "postgres"
	for _, clients := range settings.clients {
		jobName := fmt.Sprintf("pgbench-clients-%d", clients)
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
						InitContainers: []corev1.Container{{
							Name: "pgbench-init", Image: image,
							Command: []string{"pgbench", "-i", "-h", "postgres-server", "-U", "admin", "perfdb"},
							Env:     []corev1.EnvVar{{Name: "PGPASSWORD", Value: "admin"}},
						}}, Containers: []corev1.Container{{
							Name: "pgbench", Image: image,
							Command: []string{"pgbench", "-h", "postgres-server", "--time",
								strconv.Itoa(settings.durationSecs), "--client",
								strconv.Itoa(clients), "-U", "admin", "perfdb"},
							Env: []corev1.EnvVar{{Name: "PGPASSWORD", Value: "admin"}},
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
	return jobs
}

func getPostgresServerInfo(settings *postgresSettings) *common.ServerInfo {
	return &common.ServerInfo{
		Name: "postgresql-95-rhel7",
		Resources: common.ResourceSettings{
			Memory: settings.memory,
			CPU:    settings.cpu,
		},
		Settings:   settings.env,
		Deployment: getPostgresDeployment(),
	}
}

func getPostgresDeployment() *appsv1.Deployment {
	dep, _ := k8s.NewDeployment("postgres-server", "", k8s.DeploymentOpts{
		Image:  "registry.access.redhat.com/rhscl/postgresql-95-rhel7",
		Labels: map[string]string{"app": "postgres-server"},
		EnvVars: []corev1.EnvVar{{
			Name:  "POSTGRES_HOST_AUTH_METHOD",
			Value: "trust",
		}, {
			Name:  "POSTGRESQL_USER",
			Value: "admin",
		}, {
			Name:  "POSTGRESQL_PASSWORD",
			Value: "admin",
		}, {
			Name:  "POSTGRESQL_DATABASE",
			Value: "perfdb",
		}},
	})
	return dep
}
