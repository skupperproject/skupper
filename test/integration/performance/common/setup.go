package common

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	pkgutils "github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	v1 "k8s.io/api/core/v1"
)

var (
	skupperSettings        *SkupperSettings
	skupperSites           int
	testRunner             *base.ClusterTestRunnerBase
	summary                = &resultSummary{}
	throughputHeaderFormat = "%-16s %-48s %-12s %-12s %31s %23s %18s %18s"
	throughputFormat       = "%-16s %-48s %-12d %-12d %22.2f %8s %19.4f %3s %14.4f %3s %14.4f %3s"
	debug                  bool
)

func DebugMode() bool {
	return debug
}

type resultInfo struct {
	job      JobInfo
	result   Result
	logFile  string
	jsonFile string
}

type resultSummary struct {
	apps map[string]PerformanceApp
	// results indexed by skupper site and resulting map by jobname
	results map[int]map[string]resultInfo
}

func (r *resultSummary) addResult(app PerformanceApp, result resultInfo) {
	if r.apps == nil {
		r.apps = map[string]PerformanceApp{}
	}
	if r.results == nil {
		r.results = map[int]map[string]resultInfo{}
	}
	if _, ok := r.apps[app.Name]; !ok {
		r.apps[app.Name] = app
	}
	if _, ok := r.results[result.result.Sites]; !ok {
		r.results[result.result.Sites] = map[string]resultInfo{}
	}
	siteResMap := r.results[result.result.Sites]
	siteResMap[result.job.Name] = result
}

func (r *resultSummary) appNames() []string {
	names := []string{}
	for appName, _ := range r.apps {
		names = append(names, appName)
	}
	sort.Strings(names)
	return names
}

func (r *resultSummary) jobNames(app string) []string {
	return r.apps[app].Client.JobNames()
}

func RunPerformanceTests(m *testing.M, debugMode bool) {
	var err error
	var rc int
	defer func(rc *int) {
		os.Exit(*rc)
	}(&rc)

	// Parsing flags
	base.ParseFlags()
	debug = debugMode

	// Parsing settings
	skupperSettings, err = parseSettings()
	if err != nil {
		log.Fatalf("error parsing skupper settings: %v", err)
	}
	settingsJson, _ := json.MarshalIndent(skupperSettings, "", "    ")
	log.Printf("Skupper settings: %s", string(settingsJson))

	// Running the tests
	rc, err = run(m, debugMode)
	if err != nil {
		log.Fatalf("error running performance tests: %v", err)
	}

	// Displaying summary
	displaySummary()
}

func displaySummary() {

	// Displaying log and json files generated for each app/job
	log.Println("Performance test execution summary")
	stepLog.Println("Generated log and json files")
	sublog := subStepLog(stepLog)
	logFormat := "%-16s %-8s %-48s %s"
	sublog.Printf(logFormat, "APP", "SITES", "JOB", "OUTPUT FILE")
	for _, appName := range summary.appNames() {
		app := summary.apps[appName]
		for _, sites := range skupperSettings.Sites {
			for _, jobName := range app.Client.JobNames() {
				res := summary.results[sites][jobName]
				sublog.Printf(logFormat, appName, strconv.Itoa(sites), jobName, res.logFile)
				sublog.Printf(logFormat, "", "", "", res.jsonFile)
			}
		}
	}

	// Displaying throughput for each app/job
	stepLog.Println("Throughput summary")
	sublog.Printf(throughputHeaderFormat, "APP", "JOB", "SITES", "CLIENTS", "THROUGHPUT", "LATENCY AVG", "LATENCY 50%", "LATENCY 99%")
	for _, appName := range summary.appNames() {
		app := summary.apps[appName]
		for _, sites := range skupperSettings.Sites {
			for _, jobName := range app.Client.JobNames() {
				res := summary.results[sites][jobName]
				jobRes := res.result
				sublog.Printf(throughputFormat, app.Name, jobName, sites, res.job.Clients, jobRes.Throughput,
					strings.ToLower(string(app.ThroughputUnit)), jobRes.LatencyAvg, strings.ToLower(string(app.LatencyUnit)),
					jobRes.Latency50, strings.ToLower(string(app.LatencyUnit)),
					jobRes.Latency99, strings.ToLower(string(app.LatencyUnit)))
			}
		}
	}
}

func parseSettings() (*SkupperSettings, error) {
	skupperTimeout := utils.StrDefault("60m", os.Getenv(ENV_SKUPPER_PERF_TIMEOUT))
	if _, err := time.ParseDuration(skupperTimeout); err != nil {
		return nil, fmt.Errorf("invalid timeout for performance tests: %v", err)
	}

	// SKUPPER_SITES
	skupperSites := strings.Split(utils.StrDefault("2", os.Getenv(ENV_SKUPPER_SITES)), ",")
	if os.Getenv(ENV_SKUPPER_SITES) == "" && DebugMode() {
		log.Printf("setting skupper sites to 1 (debug mode)")
		skupperSites = []string{"1"}
	}
	var skupperSitesInt []int
	for _, v := range skupperSites {
		iv, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid value for SKUPPER_SITES (int or int csv expected) - %v", err)
		}
		skupperSitesInt = append(skupperSitesInt, iv)
	}

	// SKUPPER_MAX_FRAME_SIZE
	skupperMaxFrameSize, err := strconv.Atoi(utils.StrDefault(strconv.Itoa(types.RouterMaxFrameSizeDefault), os.Getenv(ENV_SKUPPER_MAX_FRAME_SIZE)))
	if err != nil {
		return nil, fmt.Errorf("invalid value for SKUPPER_MAX_FRAME_SIZE (int expected) - %v", err)
	}
	// SKUPPER_MAX_SESSION_FRAMES
	skupperMaxSessionFrames, err := strconv.Atoi(utils.StrDefault(strconv.Itoa(types.RouterMaxSessionFramesDefault), os.Getenv(ENV_SKUPPER_MAX_SESSION_FRAMES)))
	if err != nil {
		return nil, fmt.Errorf("invalid value for SKUPPER_MAX_SESSION_FRAMES (int expected) - %v", err)
	}
	// SKUPPER_MEMORY
	skupperMemory := os.Getenv(ENV_SKUPPER_MEMORY)
	// SKUPPER_CPU
	skupperCpu := os.Getenv(ENV_SKUPPER_CPU)

	return &SkupperSettings{
		Sites: skupperSitesInt,
		Router: RouterSettings{
			MaxFrameSize:     skupperMaxFrameSize,
			MaxSessionFrames: skupperMaxSessionFrames,
			Resources: ResourceSettings{
				Memory: skupperMemory,
				CPU:    skupperCpu,
			},
		},
		Timeout: skupperTimeout,
	}, nil
}

func tearDown(r *base.ClusterTestRunnerBase) {
	wg := &sync.WaitGroup{}
	wg.Add(len(r.ClusterContexts))
	for _, c := range r.ClusterContexts {
		go func(c *base.ClusterContext) {
			defer wg.Done()
			log.Printf("Deleting namespace: %s", c.Namespace)
			_ = c.DeleteNamespace()
		}(c)
	}
	wg.Wait()
}

func run(m *testing.M, debugMode bool) (int, error) {
	// Creating a local directory for storing the tokens and logs
	err := os.Mkdir(TestPath, 0755)
	if err != nil && !strings.Contains(err.Error(), "exists") {
		return 1, fmt.Errorf("error creating token directory: %v", err)
	}

	timeout, _ := time.ParseDuration(skupperSettings.Timeout)
	for _, skupperSites = range skupperSettings.Sites {

		OutputPath = fmt.Sprintf("%s/sites-%d", TestPath, skupperSites)
		_ = os.Mkdir(OutputPath, 0755)
		// Test context
		testCtx, cancelFn := context.WithTimeout(context.Background(), timeout)
		defer cancelFn()

		clustersNeeded := skupperSites
		if clustersNeeded == 0 {
			clustersNeeded = 1
		}
		needs := base.ClusterNeeds{
			NamespaceId:    "perf",
			PublicClusters: clustersNeeded,
		}

		testRunner = &base.ClusterTestRunnerBase{}
		if err := testRunner.Validate(needs); err != nil {
			log.Printf("Skipping: %v", err)
			continue
		}
		_, _ = testRunner.Build(needs, nil)
		base.HandleInterruptSignal(func() {
			cancelFn()
			tearDown(testRunner)
		})
		err = initializeSkupper(testCtx, testRunner, skupperSites, debugMode)
		if err != nil {
			tearDown(testRunner)
			return 1, fmt.Errorf("error initializing skupper: %v", err)
		}
		skupperSitesStr := "without skupper"
		if skupperSites > 0 {
			skupperSitesStr = fmt.Sprintf("with %d linked skupper sites", skupperSites)
		}

		// Tailing router logs (to get the full log)
		saveRouterLogs := true
		wg := &sync.WaitGroup{}
		if debugMode {
			wg = tailRouterLogs(testCtx, &saveRouterLogs)
		}
		defer wg.Wait()

		log.Printf("Running performance tests %s", skupperSitesStr)
		rc := m.Run()

		if rc != 0 {
			tearDown(testRunner)
			return rc, nil
		}

		saveRouterLogs = false
		tearDown(testRunner)
	}

	return 0, nil
}

func initializeSkupper(testCtx context.Context, testRunner *base.ClusterTestRunnerBase, sites int, debugMode bool) error {

	routerOptions := types.RouterOptions{
		Tuning: types.Tuning{
			Cpu:    skupperSettings.Router.Resources.CPU,
			Memory: skupperSettings.Router.Resources.Memory,
		},
		MaxFrameSize:     skupperSettings.Router.MaxFrameSize,
		MaxSessionFrames: skupperSettings.Router.MaxSessionFrames,
	}
	if debugMode {
		log.Printf("setting router debug options")
		routerOptions = constants.DefaultRouterOptions(&routerOptions)
	}

	if sites > 0 {
		var connectionToken string
		log.Printf("Initializing Skupper network with %d sites", sites)

		for i := 1; i <= testRunner.Needs.PublicClusters; i++ {
			// Getting the cluster context info
			ctx, err := testRunner.GetPublicContext(i)
			if err != nil {
				return fmt.Errorf("error getting public context %d - %v", i, err)
			}
			// Creating the namespace
			if err = ctx.CreateNamespace(); err != nil {
				return err
			}
			siteConfigSpec := types.SiteConfigSpec{
				RouterMode:        "interior",
				EnableController:  true,
				EnableServiceSync: true,
				AuthMode:          "internal",
				User:              "admin",
				Password:          "admin",
				Ingress:           ctx.VanClient.GetIngressDefault(),
				Router:            routerOptions,
			}
			// If running against a single cluster only, use none
			if !base.MultipleClusters() {
				siteConfigSpec.Ingress = types.IngressNoneString
			}
			// Initializing skupper
			log.Printf("Initializing Skupper at: %s", ctx.Namespace)
			siteConfig, err := ctx.VanClient.SiteConfigCreate(testCtx, siteConfigSpec)
			if err != nil {
				return fmt.Errorf("error creating site: %v", err)
			}
			if err = ctx.VanClient.RouterCreate(testCtx, *siteConfig); err != nil {
				return fmt.Errorf("error creating router: %v", err)
			}

			// If i > 1 (site2 connects to site1, site3 connects to site 2 and so on)
			if connectionToken != "" {
				prevCtx, _ := testRunner.GetPublicContext(i - 1)
				log.Printf("Connecting Skupper at %s with Skupper at %s", ctx.Namespace, prevCtx.Namespace)
				if _, err := ctx.VanClient.ConnectorCreateFromFile(testCtx, connectionToken, types.ConnectorCreateOptions{SkupperNamespace: ctx.Namespace}); err != nil {
					return fmt.Errorf("error linking %s to %s - %v", ctx.Namespace, prevCtx.Namespace, err)
				}
			}

			// Creating token
			if i != testRunner.Needs.PublicClusters {
				log.Printf("Creating Skupper token to connect with %s", ctx.Namespace)
				connectionToken = fmt.Sprintf("%s/cluster-%d.yaml", OutputPath, i)
				if err = ctx.VanClient.ConnectorTokenCreateFile(testCtx, types.DefaultVanName, connectionToken); err != nil {
					return fmt.Errorf("error creating token for %s - %v", ctx.Namespace, err)
				}
			}
		}

		// Waiting for skupper to be ready
		for i := 1; i <= testRunner.Needs.PublicClusters; i++ {
			cc, _ := testRunner.GetPublicContext(i)
			log.Printf("Waiting for skupper to be running at: %s", cc.Namespace)
			if err := base.WaitSkupperRunning(cc); err != nil {
				return fmt.Errorf("error waiting for skupper to be running at %s: %v", cc.Namespace, err)
			}
		}

		// Wait for skupper sites to be connected
		if testRunner.Needs.PublicClusters > 1 {
			totalConnections := testRunner.Needs.PublicClusters - 1
			for i := 1; i <= testRunner.Needs.PublicClusters; i++ {
				ctx, _ := testRunner.GetPublicContext(i)
				if err := base.WaitForSkupperConnectedSites(testCtx, ctx, totalConnections); err != nil {
					return fmt.Errorf("error waiting for sites to be connected: %v", err)
				}
			}
		}
	} else {
		// When running without Skupper
		// Getting the cluster context info
		ctx, err := testRunner.GetPublicContext(1)
		if err != nil {
			return fmt.Errorf("error getting public context 1 - %v", err)
		}
		// Creating the namespace
		log.Printf("Creating namespace: %s (without Skupper)", ctx.Namespace)
		if err = ctx.CreateNamespace(); err != nil {
			return fmt.Errorf("error creating namespace: %v", err)
		}
	}

	return nil
}

func tailRouterLogs(ctx context.Context, saveLogs *bool) *sync.WaitGroup {
	wg := &sync.WaitGroup{}
	tailRouterLogs := func(cluster *base.ClusterContext) {
		defer wg.Done()
		cli := cluster.VanClient
		fileName := fmt.Sprintf("%s/%s-skupper-router.log", OutputPath, cli.Namespace)
		tgzFileName := fmt.Sprintf("%s.tar.gz", fileName)
		var pod *v1.Pod
		var err error
		log.Printf("Waiting for router pod to be running")
		err = pkgutils.RetryWithContext(ctx, time.Second*5, func() (bool, error) {
			pod, err = kube.GetReadyPod(cli.Namespace, cli.KubeClient, "router")
			if pod != nil && err == nil {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			log.Printf("Error waiting for skupper-router pod to be running: %v", err)
			return
		}
		log.Printf("Buffering router logs for namespace '%s'", cli.Namespace)
		req := cli.KubeClient.CoreV1().Pods(cli.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{
			Follow:    true,
			Container: "router",
		})
		logsStream, err := req.Stream()
		if err != nil {
			log.Printf("Error getting router logs on namespace '%s': %v", cli.Namespace, err)
			return
		}
		buf := &bytes.Buffer{}
		wr, err := io.Copy(buf, logsStream)
		if err != nil {
			log.Printf("Error streaming router logs on namespace '%s': %v", cli.Namespace, err)
			return
		}
		if *saveLogs {
			log.Printf("Router logs buffer has been collected for namespace '%s': %d bytes", cli.Namespace, wr)
			tgz, err := os.Create(tgzFileName)
			if err != nil {
				log.Printf("Error creating tarball '%s': %v", tgzFileName, err)
				return
			}
			gz := gzip.NewWriter(tgz)
			defer gz.Close()
			tw := tar.NewWriter(gz)
			defer tw.Close()

			hdr := &tar.Header{
				Name: fileName,
				Mode: 0644,
				Size: wr,
			}
			err = tw.WriteHeader(hdr)
			if err != nil {
				log.Printf("Failed to write tar file header for '%s': %v", tgzFileName, err)
				return
			}
			_, err = tw.Write(buf.Bytes())
			if err != nil {
				log.Printf("Failed to write to tar archive '%s': %v", tgzFileName, err)
				return
			}
			tgzPath, _ := filepath.Abs(tgzFileName)
			log.Printf("Tarball has been saved: %s", tgzPath)
		}
	}
	if skupperSites == 0 {
		return wg
	}
	for _, cluster := range testRunner.ClusterContexts {
		wg.Add(1)
		go tailRouterLogs(cluster)
	}
	return wg
}
