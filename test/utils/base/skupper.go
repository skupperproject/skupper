package base

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/skupperproject/skupper/pkg/data"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/tools"
)

// WaitForSkupperConnectedSites waits till total number of sites are connected
// for the provided ClusterContext. If a timeout occurs or context is closed,
// an error will be returned
func WaitForSkupperConnectedSites(ctx context.Context, cc *ClusterContext, sitesTotal int) error {
	tick := time.Tick(constants.DefaultTick)
	timeout := time.After(constants.ImagePullingAndResourceCreationTimeout)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context has been canceled")
		case <-timeout:
			return fmt.Errorf("timed out waiting for sites to be connected")
		case <-tick:
			vir, err := cc.VanClient.RouterInspect(ctx)
			if err == nil && vir.Status.ConnectedSites.Total == sitesTotal {
				log.Println("VAN sites connected!")
				return nil
			} else {
				log.Printf("connection not ready yet, current pods state:")
				cc.KubectlExec("get pods -o wide")
			}
		}
	}
}

// GetConsoleData returns the ConsoleData by querying localhost:8080/DATA
// on Skupper's proxy controller pod
func GetConsoleData(cc *ClusterContext, consoleUser, consolePass string) (data.ConsoleData, error) {
	const dataEndpoint = "http://127.0.0.1:8080/DATA"
	var consoleData data.ConsoleData

	curlOpts := tools.CurlOpts{
		Silent:   true,
		Username: consoleUser,
		Password: consolePass,
		Timeout:  10,
	}

	// runs inside skupper-controller's pod
	resp, err := tools.Curl(cc.VanClient.KubeClient, cc.VanClient.RestConfig, cc.Namespace, "", dataEndpoint, curlOpts)
	if err != nil {
		log.Printf("error executing curl: %s", err)
		return consoleData, err
	}

	// 4.2.1. Parsing ConsoleData response
	if err = json.Unmarshal([]byte(resp.Body), &consoleData); err != nil {
		if strings.HasPrefix(resp.Body, "Error") {
			log.Printf(resp.Body)
			return consoleData, fmt.Errorf(resp.Body)
		} else {
			log.Printf("error unmarshalling ConsoleData: %s", err)
			log.Printf("invalid response body: %s", resp.Body)
			return consoleData, err
		}
	}

	return consoleData, nil
}
