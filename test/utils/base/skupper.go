package base

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/data"
	"github.com/skupperproject/skupper/pkg/flow"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/tools"
	corev1 "k8s.io/api/core/v1"
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

func WaitSkupperRunning(c *ClusterContext) error {
	for _, component := range []string{types.TransportComponentName, types.ControllerComponentName} {
		if err := WaitSkupperComponentRunning(c, component); err != nil {
			return err
		}
	}
	return nil
}

func WaitSkupperComponentRunning(c *ClusterContext, component string) error {
	tick := constants.DefaultTick
	timeout := constants.ImagePullingAndResourceCreationTimeout
	// wait for skupper component to be running
	selector := "skupper.io/component=" + component
	if err := kube.WaitForPodsStatus(c.Namespace, c.VanClient.KubeClient, selector, corev1.PodRunning, timeout, tick); err != nil {
		return err
	}
	return nil
}

// GetConsoleData returns the ConsoleData by emulating query to localhost:8080/DATA
// via vFlow api on service-controller vflow-collector sidecar
func GetConsoleData(cc *ClusterContext, consoleUser, consolePass string) (data.ConsoleData, error) {
	// TODO: replace console data with vFlow api
	const vFlowUrl = "https://127.0.0.1:8010/api/v1alpha1"
	var consoleData data.ConsoleData
	var sites []flow.SiteRecord
	var listeners []flow.ListenerRecord

	curlOpts := tools.CurlOpts{
		Silent:   true,
		Insecure: true,
		Username: consoleUser,
		Password: consolePass,
		Timeout:  10,
	}

	// runs inside skupper-controller's pod
	// retriev site list
	resp, err := tools.Curl(cc.VanClient.KubeClient, cc.VanClient.RestConfig, cc.Namespace, "", vFlowUrl+"/sites/", curlOpts)
	if err != nil {
		log.Printf("error executing curl: %s", err)
		return consoleData, err
	}

	// 4.2.1. Parsing sites response
	if err = json.Unmarshal([]byte(resp.Body), &sites); err != nil {
		if strings.HasPrefix(resp.Body, "Error") {
			log.Printf(resp.Body)
			return consoleData, fmt.Errorf(resp.Body)
		} else {
			log.Printf("error unmarshalling SiteRecords: %s", err)
			log.Printf("invalid response body: %s", resp.Body)
			return consoleData, err
		}
	}
	for _, site := range sites {
		consoleData.Sites = append(consoleData.Sites, data.Site{
			SiteId:    site.Identity,
			SiteName:  *site.Name,
			Namespace: *site.NameSpace,
		})
	}

	// retrieve listener list
	resp, err = tools.Curl(cc.VanClient.KubeClient, cc.VanClient.RestConfig, cc.Namespace, "", vFlowUrl+"/listeners/", curlOpts)
	if err != nil {
		log.Printf("error executing curl: %s", err)
		return consoleData, err
	}

	// 4.2.1. Parsing listeners response
	if err = json.Unmarshal([]byte(resp.Body), &listeners); err != nil {
		if strings.HasPrefix(resp.Body, "Error") {
			log.Printf(resp.Body)
			return consoleData, fmt.Errorf(resp.Body)
		} else {
			log.Printf("error unmarshalling ListenerRecords: %s", err)
			log.Printf("invalid response body: %s", resp.Body)
			return consoleData, err
		}
	}
	uniqueListeners := make(map[string]flow.ListenerRecord)
	for _, listener := range listeners {
		if listener.EndTime == 0 {
			if _, ok := uniqueListeners[*listener.Name]; !ok {
				uniqueListeners[*listener.Name] = listener
			}
		}
	}

	var ServiceByType []interface{}
	for _, listener := range uniqueListeners {
		switch *listener.Protocol {
		case "tcp":
			tcpsvc := data.TcpService{
				Service: data.Service{
					Address:  *listener.Address,
					Protocol: "tcp",
				},
			}
			ServiceByType = append(ServiceByType, tcpsvc)
		case "http1":
			httpsvc := data.HttpService{
				Service: data.Service{
					Address:  *listener.Address,
					Protocol: "http",
				},
			}
			ServiceByType = append(ServiceByType, httpsvc)
		default:
			fmt.Println("[GetConsoleData] - Unsupported protocol ", *listener.Protocol)
		}
	}

	// Replace Services in consoleData
	consoleData.Services = ServiceByType

	return consoleData, nil
}
