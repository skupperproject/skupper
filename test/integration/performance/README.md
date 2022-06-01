# Skupper performance tests

The Skupper performance test suite is meant to help evaluate the throughput
and latency of applications through a Skupper network. By default, one Skupper
site (or hop) is used, but it can be customized to multiple hops in a single
cluster or even against multiple clusters.

A summary will be displayed when performance tests complete along with a set of
log and json files (results) for each executed test.

## Mechanism

When performance tests are executed, a generic mechanism will iterate through
a list of topologies (default: 2 sites) and then it will initialize
skupper on static namespaces: `public-perf-1` and `public-perf-2`. 

If multiple number of sites are provided (see: [Environment variables](#environment-variables) section),
performance mechanism will iterate through list of sites and run all performance
tests for each number of linked Skupper sites (hops).

The skupper site(s) will be linked to each other before performance tests are
executed.

The performance tests will then call `RunPerformanceTest` to run the specific test
collecting all results. At the end a summary will be displayed and teardown (removal
of all namespaces) will be executed.

## Adding a new performance test

To write a new performance test, all you need to do is implement the `PerformanceTest` interface
and inside your go test functions, just call `RunPerformanceTest`.

The `PerformanceTest` interface defines the following methods:

```go
type PerformanceTest interface {
	App() PerformanceApp
	Validate(serverCluster, clientCluster *base.ClusterContext, job JobInfo) Result
}
```

Basically the `App()` method returns the details for the application to be tested, including its:
* Name and description
* Server deployment information
* Client jobs to run

The `Validate()` method is a custom parser that basically collects latency and throughput for
the application being tested.

## Existing tests

#### AMQP

The AMQP test runs an AMQP Router server and the [Quiver](https://github.com/ssorj/quiver) performance tool.

#### HTTP

It runs a standard nginx server and creates two services:
* http-server
* http-server-tcp

The first uses HTTP protocol adaptor and the second uses a TCP adaptor.

For each service, it runs two HTTP benchmark clients: [wrk](https://github.com/wg/wrk) and [hey](https://github.com/rakyll/hey).

#### iPerf3

Collects the TCP throughput using iperf3.

#### Postgres

Uses pgbench to benchmark number of transactions per second against a postgres server.

#### Redis

This test uses redis-benchmark tool to evaluate messages per second.

## Environment variables

The behavior and iterations to run can be modified by customization of environment
variables before running the performance tests.

### Common

The following environment variables are common to all tests, as they are used
to define which topologies will be tested and skupper specific settings.

| Name                       | Type        | Description                                                                                     |  Default value | Sample |
|:---------------------------|:------------|:------------------------------------------------------------------------------------------------|---------------:|-------:|
| SKUPPER_SITES              | []int (CSV) | List with number of skupper sites to iterate through.                                           |              2 |    1,2 |
| SKUPPER_MAX_FRAME_SIZE     | int         | Sets the maximum frame size for inter-router connections.                                       |          16384 |        |
| SKUPPER_MAX_SESSION_FRAMES | int         | Sets the maximum session frames for inter-router connections.                                   |            640 |        |
| SKUPPER_MEMORY             | string      | Memory specification for the Skupper and Router pods.                                           |                | 2000Mi |
| SKUPPER_CPU                | string      | CPU specification for the Skupper and Router pods.                                              |                |   500m |
| SKUPPER_PERF_TIMEOUT       | string      | Timeout duration used to wait for Skupper network initialization and for capturing router logs. |            60m |        |

### Test specific

#### AMQP

| Name               | Type   | Description                                          | Default value | Sample |
|:-------------------|:-------|:-----------------------------------------------------|--------------:|-------:|
| AMQP_DURATION_SECS | int    | Duration in seconds for the performance test to run. |            30 |        |
| AMQP_TIMEOUT       | string | Time duration to wait for benchmark job to complete. |           10m |        |
| AMQP_MEMORY        | string | Memory specification for server and client pods.     |               | 2000Mi |
| AMQP_CPU           | string | CPU specification for server and client pods.        |               |   500m |

#### HTTP

| Name                  | Type        | Description                                                                                      | Default value | Sample |
|:----------------------|:------------|:-------------------------------------------------------------------------------------------------|--------------:|-------:|
| HTTP_DURATION_SECS    | int         | Duration in seconds for the performance test to run.                                             |            30 |        |
| HTTP_PARALLEL_CLIENTS | []int (CSV) | List with number of clients to use. A performance test will be executed for each value provided. |             2 |   2,10 |
| HTTP_CONNECTIONS      | int         | Number of client connections to keep open. Used by wrk only.                                     |            10 |        |
| HTTP_RATE             | string      | Rate limit in requests per second. Used by wrk2 and hey clients.                                 |               |   1000 |
| HTTP_MEMORY           | string      | Memory specification for server and client pods.                                                 |               | 2000Mi |
| HTTP_CPU              | string      | CPU specification for server and client pods.                                                    |               |   500m |
| HTTP_TIMEOUT          | string      | Time duration to wait for benchmark job to complete.                                             |           10m |        |

#### iPerf3

| Name                   | Type        | Description                                                                                      | Default value |  Sample |
|:-----------------------|:------------|:-------------------------------------------------------------------------------------------------|--------------:|--------:|
| IPERF_PARALLEL_CLIENTS | []int (CSV) | List with number of clients to use. A performance test will be executed for each value provided. |             1 |         |
| IPERF_TRANSMIT_SIZES   | []int (CSV) | List with transmit sizes to use. A performance test will be executed for each value provided.    |            1G | 100M,1G |
| IPERF_WINDOW_SIZE      | int         | Window size or socket buffer size.                                                               |             0 |         |
| IPERF_MEMORY           | string      | Memory specification for server and client pods.                                                 |               | 2000Mi |
| IPERF_CPU              | string      | CPU specification for server and client pods.                                                    |               |   500m |
| IPERF_TIMEOUT          | string      | Time duration to wait for benchmark job to complete.                                             |           10m |        |

#### Postgres

| Name                      | Type        | Description                                                                                      |                              Default value | Sample |
|:--------------------------|:------------|:-------------------------------------------------------------------------------------------------|-------------------------------------------:|-------:|
| POSTGRES_PARALLEL_CLIENTS | []int (CSV) | List with number of clients to use. A performance test will be executed for each value provided. |                                          1 |        |
| POSTGRES_DURATION_SECS    | int         | Duration in seconds for the performance test to run.                                             | 30 (performance tag) / 5 (integration tag) |        |
| POSTGRES_MEMORY           | string      | Memory specification for server and client pods.                                                 |                                            | 2000Mi |
| POSTGRES_CPU              | string      | CPU specification for server and client pods.                                                    |                                            |   500m |
| POSTGRES_TIMEOUT          | string      | Time duration to wait for benchmark job to complete.                                             |                                        10m |        |

#### Redis

| Name                   | Type           | Description                                                                                                                                     |                                    Default value |       Sample |
|:-----------------------|:---------------|:------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------:|-------------:|
| REDIS_NUMBER_REQUESTS  | int            | Number of requests to send.                                                                                                                     | 25000 (performance tag) / 1000 (integration tag) |              |
| REDIS_PARALLEL_CLIENTS | []int (CSV)    | List with number of clients to use. A performance test will be executed for each value provided.                                                |                                               50 |              |
| REDIS_TESTS            | []string (CSV) | List with redis-benchamrk tests to run. A performance test will be executed for each test provided. If empty, all operations will be performed. |                                                  | GET,SET,MSET |
| REDIS_DATASIZE         | int            | Data size in bytes.                                                                                                                             |                                                3 |         1024 |
| REDIS_MEMORY           | string         | Memory specification for server and client pods.                                                                                                |                                                  |       2000Mi |
| REDIS_CPU              | string         | CPU specification for server and client pods.                                                                                                   |                                                  |         500m |
| REDIS_TIMEOUT          | string         | Time duration to wait for benchmark job to complete.                                                                                            |                                              10m |              |

## How to run the performance tests

The TCP performance suite is meant to run as an integration test through
CI regularly, but can be customized to run longer for Performance evaluation.

This is controlled based on the build tag used to run the tests.
You can use the following build tags: `integration` or `performance`.

When `integration` tag is used, the router will run with trace level and the
entire router log will be saved (performance penalty).
Some tests might use different settings when running using integration mode,
to avoid impacts to the CI.

The recommended way to run is using the `performance` tag, which will run the
router in regular mode without trace of capturing the entire log.

### Iterations

Test can be executed by default against a single or multiple clusters, in a single
or multiple namespaces, depending on value provided through `SKUPPER_SITES` environment
variable (default is **1**) and the `--kubeconfig` test flags provided.

The namespaces to be used are static and will have the prefix `public-perf-#`. The # will
be replaced with the site number. Example given: if using SKUPPER_SITES=3, the following
namespaces will be created:
* public-perf-1
* public-perf-2
* public-perf-3

Remember that the `SKUPPER_SITES` environment variable accepts a list of int values,
so it will iterate through the individual set of values to create linear topologies
with distinct sizes, then run tests against it.

Example:

SKUPPER_SITES=1,2

Will generate two iterations. One with a single Skupper site and a second iteration
with 2 linked Skupper sites.

Server apps will run in one end and the client apps will run in the other end of the
topology. Example with 2 sites:

    [Server App] ----- [Skupper site 1] ----- [Skupper site 2] ----- [Client App]

#### Single cluster

You must have a `KUBECONFIG` environment variable set and referring to a
valid cluster connection, or you can specify the config file to be used by
adding the `--kubeconfig` flag to the test command shown below.

```
# Use this if you have KUBECONFIG env var exported
go test -v -count=1 -p=1 -tags=performance -timeout 30m ./test/integration/performance/

# Use this if you do not have it
go test -v -count=1 -p=1 -tags=performance -timeout 30m ./test/integration/performance/ --kubeconfig kubeconfig_file1
```

#### Multiple clusters

You must have a KUBECONFIG file set for each pair of cluster/context to be
used in order to run this test suite. If you are using multiple clusters, then
number of `SKUPPER_SITES` must be equal to the number of KUBECONFIG files you
have available. Then, all you need to do is run it like:

```
export SKUPPER_SITES=2
go test -v -count=1 -p=1 -tags=performance -timeout 30m ./test/integration/performance/ --kubeconfig ./kubeconfig_file1 --kubeconfig ./kubeconfig_file2
```

In this case, the namespace `public-perf-1` will be created at the cluster referenced by `kubeconfig_file1`.
And the namespace `public-perf-2` will be created at the cluster referenced by `kubeconfig_file2`.
