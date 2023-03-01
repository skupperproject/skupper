package gateway

import (
	"context"
	"fmt"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InitTester runs `skupper gateway init` and asserts that
// the gateway is defined accordingly
type InitTester struct {
	GeneratedName *string
	Type          string
}

func (i *InitTester) isService() bool {
	return i.Type == "" || i.Type == "service"
}

func (i *InitTester) Command(platform types.Platform, cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(platform, cluster)
	args = append(args, "gateway", "init")

	if i.Type != "" {
		args = append(args, "--type", i.Type)
	}

	return args
}

func (i *InitTester) Run(platform types.Platform, cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	//
	// Retrieve existing list of gateways
	//
	ctx := context.Background()
	existingGateways, err := cluster.VanClient.GatewayList(ctx)
	if err != nil {
		return
	}

	// Execute the gateway init command
	stdout, stderr, err = cli.RunSkupperCli(i.Command(platform, cluster))
	if err != nil {
		return
	}

	//
	// Retrieve updated list of gateways
	//
	var currentGateways []*types.GatewayInspectResponse
	err = utils.Retry(time.Second, 5, func() (bool, error) {
		currentGateways, err = cluster.VanClient.GatewayList(ctx)
		if err != nil {
			return false, err
		}
		if len(currentGateways) > len(existingGateways) {
			for _, gw := range currentGateways {
				if gw.Name != "" {
					return true, nil
				}
			}
		}
		return false, nil
	})
	if err != nil {
		return
	}

	var gatewayName string

	if len(currentGateways) == 1 {
		gatewayName = currentGateways[0].Name
	} else if len(currentGateways) > len(existingGateways) {
		for _, gw := range currentGateways {
			found := false
			for _, existingGw := range existingGateways {
				if existingGw.Name == gw.Name {
					found = true
				}
			}
			if !found {
				gatewayName = gw.Name
				break
			}
		}
		if gatewayName == "" {
			err = fmt.Errorf("unable to discover gateway name")
			return
		}
	} else {
		err = fmt.Errorf("could not find a new gateway")
		return
	}

	//
	// Retrieve ConfigMap with skupper.io/type: gateway-definition (label)
	//
	cmList, err := cluster.VanClient.KubeClient.CoreV1().ConfigMaps(cluster.Namespace).List(ctx, v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", types.SkupperTypeQualifier, "gateway-definition"),
	})
	if err != nil {
		return
	}

	cmName := ""
	for _, cm := range cmList.Items {
		gwName, ok := cm.Annotations["skupper.io/gateway-name"]
		if ok && gwName == gatewayName {
			cmName = cm.Name
		}
	}
	if cmName == "" {
		err = fmt.Errorf("unable to find gateway configmap")
		return
	}

	//
	// Retrieve Secret (token) with same ConfigMap name
	//
	_, err = cluster.VanClient.KubeClient.CoreV1().Secrets(cluster.Namespace).Get(ctx, cmName, v1.GetOptions{})
	if err != nil {
		return
	}

	expectAvailable := true
	if i.isService() {
		// Validating systemd user service created
		available := SystemdUnitAvailable(gatewayName)
		if available != expectAvailable {
			err = fmt.Errorf("systemd unit %s.service availability issue - available: %v - expected: %v", gatewayName, available, expectAvailable)
			return
		}

		// Validating systemd user service enabled
		enabled := SystemdUnitEnabled(gatewayName)
		if enabled != expectAvailable {
			err = fmt.Errorf("systemd unit %s.service availability issue - enabled: %v - expected: %v", gatewayName, enabled, expectAvailable)
			return
		}
	} else if i.Type == "docker" {
		available, _ := IsDockerContainerRunning(gatewayName)
		if available != expectAvailable {
			err = fmt.Errorf("docker container %s availability issue - enabled: %v - expected: %v", gatewayName, available, expectAvailable)
		}
	} else if i.Type == "podman" {
		available, _ := IsPodmanContainerRunning(gatewayName)
		if available != expectAvailable {
			err = fmt.Errorf("podman container %s availability issue - enabled: %v - expected: %v", gatewayName, available, expectAvailable)
		}
	}

	// Setting Generated Name
	if i.GeneratedName != nil {
		*i.GeneratedName = gatewayName
	}

	return
}
