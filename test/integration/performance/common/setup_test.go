//go:build manual
// +build manual

package common

import (
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	amqpJob1 = JobInfo{"quiver", 1, &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "quiver",
		},
	}}
	amqpApp = PerformanceApp{Name: "amqp", Description: "amqp Application", Service: ServiceInfo{
		Address:  "amqp-server",
		Protocol: "amqp",
		Adaptor:  AdaptorTCP,
		Port:     5672,
	}, Server: &ServerInfo{
		Name: "skupper-router",
	}, Client: &ClientInfo{
		Name: "quiver",
		Jobs: []JobInfo{
			amqpJob1,
		},
	}, ThroughputUnit: ThroughputUnitMsgs, LatencyUnit: LatencyUnitMs}

	pgJob1 = JobInfo{"pgbench", 1, &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pgbench",
		},
	}}
	pgApp = PerformanceApp{Name: "postgres", Description: "Postgres Application", Service: ServiceInfo{
		Address:  "postgres-server",
		Protocol: "tcp",
		Adaptor:  AdaptorTCP,
		Port:     5432,
	}, Server: &ServerInfo{
		Name: "postgres",
	}, Client: &ClientInfo{
		Name: "pgbench",
		Jobs: []JobInfo{
			pgJob1,
		},
	}, ThroughputUnit: ThroughputUnitTps, LatencyUnit: LatencyUnitMs}
)

func TestDisplaySummary(t *testing.T) {
	skupperSettings = &SkupperSettings{
		Sites: []int{0, 1, 2},
	}
	addAmqpResults()
	addPostgresResults()

	displaySummary()
}

func addAmqpResults() {
	summary.addResult(amqpApp, resultInfo{
		job: amqpJob1,
		result: Result{
			App:        amqpApp,
			Sites:      0,
			Job:        amqpJob1,
			Throughput: 30000,
			LatencyAvg: 5,
			Latency50:  3,
			Latency99:  24,
		},
		logFile:  "/home/skupper/amqp_0/quiver.log",
		jsonFile: "/home/skupper/amqp_0/quiver.json",
	})
	summary.addResult(amqpApp, resultInfo{
		job: amqpJob1,
		result: Result{
			App:        amqpApp,
			Sites:      1,
			Job:        amqpJob1,
			Throughput: 25635,
			LatencyAvg: 5.20,
			Latency50:  4,
			Latency99:  25,
		},
		logFile:  "/home/skupper/amqp_1/quiver.log",
		jsonFile: "/home/skupper/amqp_1/quiver.json",
	})
	summary.addResult(amqpApp, resultInfo{
		job: amqpJob1,
		result: Result{
			App:        amqpApp,
			Sites:      2,
			Job:        amqpJob1,
			Throughput: 26000,
			LatencyAvg: 5.10,
			Latency50:  4.1,
			Latency99:  23.5,
		},
		logFile:  "/home/skupper/amqp_2/quiver.log",
		jsonFile: "/home/skupper/amqp_2/quiver.json",
	})
}

func addPostgresResults() {
	summary.addResult(pgApp, resultInfo{
		job: pgJob1,
		result: Result{
			App:        pgApp,
			Sites:      0,
			Job:        pgJob1,
			Throughput: 1000,
			LatencyAvg: 10,
			Latency50:  0,
			Latency99:  0,
		},
		logFile:  "/home/skupper/postgres_0/pgbench.log",
		jsonFile: "/home/skupper/postgres_0/pgbench.json",
	})
	summary.addResult(pgApp, resultInfo{
		job: pgJob1,
		result: Result{
			App:        pgApp,
			Sites:      1,
			Job:        pgJob1,
			Throughput: 900,
			LatencyAvg: 10.5,
			Latency50:  0,
			Latency99:  0,
		},
		logFile:  "/home/skupper/postgres_1/pgbench.log",
		jsonFile: "/home/skupper/postgres_1/pgbench.json",
	})
	summary.addResult(pgApp, resultInfo{
		job: pgJob1,
		result: Result{
			App:        pgApp,
			Sites:      2,
			Job:        pgJob1,
			Throughput: 910,
			LatencyAvg: 10.1,
			Latency50:  0,
			Latency99:  0,
		},
		logFile:  "/home/skupper/postgres_2/pgbench.log",
		jsonFile: "/home/skupper/postgres_2/pgbench.json",
	})
}
