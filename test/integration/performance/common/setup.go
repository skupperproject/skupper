package common

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
)

var (
	skupperSettings *SkupperSettings
	skupperSites    int
	testRunner      *base.ClusterTestRunnerBase
)

func RunPerformanceTests(m *testing.M, debugMode bool) {
	var err error
	var rc int
	defer func(rc *int) {
		os.Exit(*rc)
	}(&rc)

	// Parsing flags
	base.ParseFlags()

	// Parsing settings
	skupperSettings, err = parseSettings()
	if err != nil {
		log.Fatalf("error parsing skupper settings: %v", err)
	}

	// Running the tests
	rc, err = run(m, debugMode)
	if err != nil {
		log.Fatalf("error running performance tests: %v", err)
	}
}

func parseSettings() (*SkupperSettings, error) {
	skupperTimeout := utils.StrDefault("10m", os.Getenv(ENV_SKUPPER_PERF_TIMEOUT))
	if _, err := time.ParseDuration(skupperTimeout); err != nil {
		return nil, fmt.Errorf("invalid timeout for performance tests: %v", err)
	}

	// SKUPPER_SITES
	skupperSites := strings.Split(utils.StrDefault("1", os.Getenv(ENV_SKUPPER_SITES)), ",")
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
		rc := m.Run()

		if rc != 0 {
			tearDown(testRunner)
			return rc, nil
		}
		tearDown(testRunner)
	}

	// TODO Results summary (parse all JSON files and print to output)

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

			// TODO ADD LOGIC TO TAIL ROUTER LOGS (on debug mode)

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
