# TCP Performance Tests

The TCP performance suite is meant to run as an integration test through
CI regularly, but can be customized to run longer for Performance evaluation.

To allow that, this particular suite introduces a few environment variables that
can be customized in order to change the behavior and dynamically define the test
matrix that will be executed.

## iPerf test

The iPerf suite runs tests with a variable number of Skupper sites (and 
optionally multiple clusters), parallel iPerf clients and dynamic data size.

### Environment variables

**IPERF_PARALLEL_CLIENTS**

    The number of parallel iperf3 clients to run.
    Default is 1.

**IPERF_TRANSMIT_SIZES**

    Comma separated list of amount of data sizes to transmit, example: 10M,100M,1G
    The test will iterate through the comma separated list of values and compose a matrix.
    Default is: 100M,500M,1G

**IPERF_WINDOW_SIZE**

    Window size or socket buffer size.
    If not provided do not use it.

**IPERF_MEMORY**

    Memory specification for the iPerf pods (applies to both client and server pods).

**IPERF_CPU**

    CPU specification for the iPerf pods (applies to both client and server pods).

**IPERF_JOB_TIMEOUT**

    Amount of time in go duration format. Default: 10m.

**SKUPPER_SITES**

    The number of sites to iterate over (if 3 is given, test will iterate from 0 [no Skupper] to 3 [hops]).

**SKUPPER_MAX_FRAME_SIZE**

    Sets the maximum frame size for inter-router connections.

**SKUPPER_MAX_SESSION_FRAMES**

    Sets the maximum session frames for inter-router connections.

**SKUPPER_MEMORY**

    Memory specification for the Skupper and Router pods.

**SKUPPER_CPU**

    CPU specification for the Skupper and Router pods.

### Running the test

Test can be executed by default against a single cluster. In this case the test
suite will use multiple namespaces inside the same cluster.

#### Single cluster

You must have a `KUBECONFIG` environment variable set and referring to a
valid cluster connection, or you can specify the config file to be used by
adding the `--kubeconfig` flag to the test command shown below.

```
# Use this if you have KUBECONFIG env var exported
go test -v -count=1 -tags performance -timeout 30m ./test/integration/performance/tcp/

# Use this if you do not have it
go test -v -count=1 -tags performance -timeout 30m ./test/integration/performance/tcp/ --kubeconfig kubeconfig_file1
```

#### Multiple clusters

You must have a KUBECONFIG file set for each pair of cluster/context to be
used in order to run this test suite. If you are using multiple clusters, then
number of `SKUPPER_SITES` must be equal to the number of KUBECONFIG files you
have available. Then, all you need to do is run it like:

```
export SKUPPER_SITES=2
go test -v -count=1 -tags performance -timeout 30m ./test/integration/performance/tcp/ --kubeconfig ./kubeconfig_file1 --kubeconfig ./kubeconfig_file2 ...
```

This will render tests with:

* iPerf client -> iPerf server (no Skupper)
* iPerf client -> Skupper site 1 -> iPerf server
* iPerf client -> Skupper site 2 -> Skupper site 1 -> iPerf server

Along with the matrix above, the variable number of data sizes will multiply
the test scenarios. If you have defined:

```
export SKUPPER_SITES=2
export IPERF_TRANSMIT_SIZES=100M,500M,1G
```

... then the matrix will be composed of 3 (number of skupper sites + 1) * 3 (number of transmit sizes).
