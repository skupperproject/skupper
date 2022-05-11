package tcp

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"gotest.tools/assert"
)

const (
	ENV_IPERF_PARALLEL_CLIENTS = "IPERF_PARALLEL_CLIENTS"
	ENV_IPERF_TRANSMIT_SIZES   = "IPERF_TRANSMIT_SIZES"
	ENV_IPERF_WINDOW_SIZE      = "IPERF_WINDOW_SIZE"
	ENV_IPERF_MEMORY           = "IPERF_MEMORY"
	ENV_IPERF_CPU              = "IPERF_CPU"
	ENV_IPERF_JOB_TIMEOUT      = "IPERF_JOB_TIMEOUT"

	ENV_SKUPPER_SITES              = "SKUPPER_SITES"
	ENV_SKUPPER_MAX_FRAME_SIZE     = "SKUPPER_MAX_FRAME_SIZE"
	ENV_SKUPPER_MAX_SESSION_FRAMES = "SKUPPER_MAX_SESSION_FRAMES"
	ENV_SKUPPER_MEMORY             = "SKUPPER_MEMORY"
	ENV_SKUPPER_CPU                = "SKUPPER_CPU"
)

func runIperfTest(t *testing.T, perf bool) {
	//
	// Parsing environment variables
	//
	iperfSettings := parseIperfSettings(t)
	skupperSettings := parseSkupperSettings(t)

	// Creating a local directory for storing the tokens and logs
	err := os.Mkdir(TestPath, 0755)
	if err != nil && !strings.Contains(err.Error(), "exists") {
		assert.Assert(t, err)
	}

	// Composing a test matrix based on number of sites to iterate and transmit sizes
	for site := 0; site <= skupperSettings.Sites; site++ {
		for _, size := range iperfSettings.TransmitSizes {
			for _, clients := range iperfSettings.ParallelClients {
				// Test context
				testCtx, cancelFn := context.WithTimeout(context.Background(), iperfSettings.JobTimeout)
				defer cancelFn()

				scenario := IperfScenario{
					SkupperSites:    site,
					TransmitSize:    size,
					ParallelClients: clients,
					SkupperSettings: skupperSettings,
					IperfSettings:   iperfSettings,
					testCtx:         testCtx,
				}

				t.Run(scenario.getTestName(), func(t *testing.T) {
					// Handling test interruptions and tearDown
					base.HandleInterruptSignal(func() {
						scenario.tearDown()
						t.FailNow()
					})

					// Logging scenario info
					t.Logf("iPerf scenario info: %s", scenario.String())

					// Initializing clusters and Skupper network (if site > 0)
					scenario.initializeClusters(t, !perf)

					// Tailing router logs (to get the full log)
					saveRouterLogs := true
					wg := &sync.WaitGroup{}
					if !perf {
						wg = scenario.tailRouterLogs(t, testCtx, &saveRouterLogs)
					}
					defer wg.Wait()
					defer scenario.tearDown()

					// Deploying the iPerf3 server
					scenario.deployIperf3Server(t)

					// Expose iperf3-server service (through Skupper or straight through k8s if not using Skupper)
					scenario.exposeIperf3Server(t)

					// Running the iPerf3 client
					scenario.runIperf3Client(t)

					// If reached this point, test has passed (full router logs will be discarded)
					saveRouterLogs = false
				})
			}
		}
	}
}

// parseIperfSettings parses environment variables to customize iPerf3
func parseIperfSettings(t *testing.T) IperfTuning {
	// IPERF_PARALLEL_CLIENTS
	var iperfParallelClients []int
	for _, parallelClientStr := range strings.Split(utils.StrDefault("1", os.Getenv(ENV_IPERF_PARALLEL_CLIENTS)), ",") {
		parallelClients, err := strconv.Atoi(parallelClientStr)
		assert.Assert(t, err, "invalid value for IPERF_PARALLEL_CLIENTS (int expected)")
		iperfParallelClients = append(iperfParallelClients, parallelClients)
	}
	// IPERF_TRANSMIT_SIZES
	iperfTransmitSizes := strings.Split(utils.StrDefault("1G", os.Getenv(ENV_IPERF_TRANSMIT_SIZES)), ",")
	// IPERF_WINDOW_SIZE
	iperfWindowSize, err := strconv.Atoi(utils.StrDefault("0", os.Getenv(ENV_IPERF_WINDOW_SIZE)))
	assert.Assert(t, err, "invalid value for IPERF_WINDOW_SIZE (int expected)")
	// IPERF_MEMORY
	iperfMemory := os.Getenv(ENV_IPERF_MEMORY)
	// IPERF_CPU
	iperfCpu := os.Getenv(ENV_IPERF_CPU)
	// IPERF_JOB_TIMEOUT
	iperfJobTimeout := utils.StrDefault("10m", os.Getenv(ENV_IPERF_JOB_TIMEOUT))
	jobTimeout, err := time.ParseDuration(iperfJobTimeout)
	assert.Assert(t, err, "invalid value for IPERF_JOB_TIMEOUT")

	return IperfTuning{
		ParallelClients: iperfParallelClients,
		TransmitSizes:   iperfTransmitSizes,
		WindowSize:      iperfWindowSize,
		Memory:          iperfMemory,
		Cpu:             iperfCpu,
		JobTimeout:      jobTimeout,
	}
}

// parseSkupperSettings parses environment variables to customize the Skupper scenarios
func parseSkupperSettings(t *testing.T) SkupperTuning {
	// SKUPPER_SITES
	skupperSites, err := strconv.Atoi(utils.StrDefault("1", os.Getenv(ENV_SKUPPER_SITES)))
	assert.Assert(t, err, "invalid value for SKUPPER_SITES (int expected)")
	// SKUPPER_MAX_FRAME_SIZE
	skupperMaxFrameSize, err := strconv.Atoi(utils.StrDefault(strconv.Itoa(types.RouterMaxFrameSizeDefault), os.Getenv(ENV_SKUPPER_MAX_FRAME_SIZE)))
	assert.Assert(t, err, "invalid value for SKUPPER_MAX_FRAME_SIZE (int expected)")
	// SKUPPER_MAX_SESSION_FRAMES
	skupperMaxSessionFrames, err := strconv.Atoi(utils.StrDefault(strconv.Itoa(types.RouterMaxSessionFramesDefault), os.Getenv(ENV_SKUPPER_MAX_SESSION_FRAMES)))
	assert.Assert(t, err, "invalid value for SKUPPER_MAX_SESSION_FRAMES (int expected)")
	// SKUPPER_MEMORY
	skupperMemory := os.Getenv(ENV_SKUPPER_MEMORY)
	// SKUPPER_CPU
	skupperCpu := os.Getenv(ENV_SKUPPER_CPU)

	return SkupperTuning{
		Sites:            skupperSites,
		MaxFrameSize:     skupperMaxFrameSize,
		MaxSessionFrames: skupperMaxSessionFrames,
		Memory:           skupperMemory,
		Cpu:              skupperCpu,
	}
}
