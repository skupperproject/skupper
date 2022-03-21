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

func podFailureDebug(pod corev1.Pod, ctx *ClusterContext, containerStatus *corev1.ContainerStatus) error {

	if containerStatus != nil {
		ctx.KubectlExec(fmt.Sprintf("logs %v -c %v", pod.Name, containerStatus.Name))
	}
	ctx.KubectlExec("get pods")
	return fmt.Errorf("At least on Skupper component failed to start: %v", pod.Name)

}

func WaitSkupperComponentRunning(c *ClusterContext, component string) error {
	tick := constants.DefaultTick
	timeout := constants.ImagePullingAndResourceCreationTimeout
	// wait for skupper component to be running
	selector := "skupper.io/component=" + component
	if err := kube.WaitForPodsStatus(c.Namespace, c.VanClient.KubeClient, selector, corev1.PodRunning, timeout, tick); err != nil {
		return err
	}

	// We recheck, and after a tick, because sometimes the pods come up and
	// then immediatelly down.  If it restarted right after deployment,
	// it's most probably not coming up anyway

	time.Sleep(tick)
	podList, err := kube.GetPods(selector, c.Namespace, c.VanClient.KubeClient)

	if err != nil {
		return err
	}
	for _, pod := range podList {
		if pod.Status.Phase != corev1.PodRunning {
			return podFailureDebug(pod, c, nil)
		}
		for _, container := range pod.Status.ContainerStatuses {
			if container.RestartCount > 0 {
				return podFailureDebug(pod, c, &container)
			}
		}
		fmt.Println(pod.Status)
	}
	if err := kube.WaitForPodsStatus(c.Namespace, c.VanClient.KubeClient, selector, corev1.PodRunning, 30*time.Second, tick); err != nil {
		return err
	}
	return nil
}

// GetConsoleData returns the ConsoleData by querying localhost:8080/DATA
// on Skupper's proxy controller pod
func GetConsoleData(cc *ClusterContext, consoleUser, consolePass string) (data.ConsoleData, error) {
	const dataEndpoint = "https://127.0.0.1:8080/DATA"
	var consoleData data.ConsoleData

	curlOpts := tools.CurlOpts{
		Silent:   true,
		Insecure: true,
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

	var ServiceByType []interface{}
	var tcpsvc data.TcpService
	var httpsvc data.HttpService

	// Iterate over Services
	for _, elem := range consoleData.Services {

		svcmap, ok := elem.(map[string]interface{})
		if !ok {
			log.Printf("[GetConsoleData] - Unable to determine protocol")
			continue
		}

		svcProto, ok := svcmap["protocol"]
		if !ok {
			log.Println("[GetConsoleData] - Unable to determine protocol ", svcProto)
			continue
		}

		// Marshal the element
		svcmarsh, err := json.Marshal(elem)
		if err != nil {
			log.Println("[GetConsoleData] - Error marshalling svc", err)
			break
		}

		// HTTP Service
		if svcProto == "http" {
			if err = json.Unmarshal(svcmarsh, &httpsvc); err == nil {
				ServiceByType = append(ServiceByType, httpsvc)
			}
			// TCP Service
		} else if svcProto == "tcp" {
			if err = json.Unmarshal(svcmarsh, &tcpsvc); err == nil {
				ServiceByType = append(ServiceByType, tcpsvc)
			}
			// Protocol not HTTP nor TCP
		} else {
			fmt.Println("[GetConsoleData] - Unsupported protocol ", svcProto)
		}
	}

	// Replace Services in consoleData
	consoleData.Services = ServiceByType

	return consoleData, nil
}
