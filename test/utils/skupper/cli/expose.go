package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	utils2 "github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExposeTester runs `skupper expose` and validates service has
// been exposed accordingly.
type ExposeTester struct {
	TargetType      string
	TargetName      string
	Address         string
	Headless        bool
	Port            int
	Protocol        string
	TargetPort      int
	PolicyProhibits bool
	Podman          PodmanExposeOptions
}

type PodmanExposeOptions struct {
	ContainerName string
	HostIp        string
	HostPorts     []string
	Labels        map[string]string
}

func (e *ExposeTester) Command(platform types.Platform, cluster *base.ClusterContext) []string {
	args := SkupperCommonOptions(platform, cluster)
	args = append(args, "expose", e.TargetType, e.TargetName)

	// Flags
	if e.Address != "" {
		args = append(args, "--address", e.Address)
	}
	if e.Headless {
		args = append(args, "--headless")
	}
	if e.Port > 0 {
		args = append(args, "--port", strconv.Itoa(e.Port))
	}
	if e.Protocol != "" {
		args = append(args, "--protocol", e.Protocol)
	}
	if e.TargetPort > 0 {
		args = append(args, "--target-port", strconv.Itoa(e.Port))
	}

	//
	// podman options
	//
	if e.Podman.ContainerName != "" {
		args = append(args, "--container-name", e.Podman.ContainerName)
	}
	if e.Podman.HostIp != "" {
		args = append(args, "--host-ip", e.Podman.HostIp)
	}
	for _, port := range e.Podman.HostPorts {
		args = append(args, "--host-port", port)
	}
	for key, value := range e.Podman.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", key, value))
	}

	return args
}

func (e *ExposeTester) Run(platform types.Platform, cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute expose command
	stdout, stderr, err = RunSkupperCli(e.Command(platform, cluster))
	if err != nil {
		if e.PolicyProhibits {
			expect := Expect{
				StdErr: []string{
					"Error: Policy validation error:",
					fmt.Sprintf("%v/%v", e.TargetType, e.TargetName),
					"cannot be exposed",
				},
			}
			err = expect.Check(stdout, stderr)
			return
		}
		return
	} else {
		if e.PolicyProhibits {
			err = fmt.Errorf("Policy error was expected, but not encountered")
			return
		}
	}

	// Validating stdout contains expected data
	log.Printf("Validating 'skupper expose'")
	if platform.IsKubernetes() {
		expectedOut := fmt.Sprintf("%s %s exposed as %s", e.TargetType, e.TargetName, utils.StrDefault(e.Address, e.TargetName))
		if !strings.Contains(stdout, expectedOut) {
			err = fmt.Errorf("expected: %s - found: %s", expectedOut, stdout)
			return
		}
	}

	attempt := 0
	expectedAddress := utils.StrDefault(e.Address, e.TargetName)
	ctx, fn := context.WithTimeout(context.Background(), commandTimeout)
	defer fn()
	err = utils2.RetryWithContext(ctx, constants.DefaultTick, func() (bool, error) {
		attempt++
		log.Printf("validating service after expose completed - attempt: %d", attempt)

		if platform == types.PlatformPodman {
			// TODO Removed broken v1 implementation
			return false, fmt.Errorf("broken implementation")
		}
		return e.validateKubernetes(ctx, platform, expectedAddress, cluster)
	})

	return
}

func (e *ExposeTester) validateKubernetes(ctx context.Context, platform types.Platform, expectedAddress string, cluster *base.ClusterContext) (bool, error) {
	var err error

	log.Printf("validating service %s exists", expectedAddress)
	_, err = cluster.VanClient.KubeClient.CoreV1().Services(cluster.Namespace).Get(ctx, expectedAddress, v1.GetOptions{})
	if err != nil {
		log.Printf("service %s does not exist - %v", expectedAddress, err)
		return false, nil
	}

	log.Printf("validating service %s exists in %s config map", expectedAddress, types.ServiceInterfaceConfigMap)
	cm, err := cluster.VanClient.KubeClient.CoreV1().ConfigMaps(cluster.Namespace).Get(ctx, types.ServiceInterfaceConfigMap, v1.GetOptions{})
	if err != nil {
		log.Printf("unable to find %s config map - %v", types.ServiceInterfaceConfigMap, err)
		return false, nil
	}

	// retrieving data
	svcStr, ok := cm.Data[expectedAddress]
	if !ok {
		return true, fmt.Errorf("address %s is not defined at %s", expectedAddress, types.ServiceInterfaceConfigMap)
	}

	// Unmarshalling and verifying targets
	var svc types.ServiceInterface
	err = json.Unmarshal([]byte(svcStr), &svc)
	if err != nil {
		return true, fmt.Errorf("unable to unmarshal service interface")
	}

	return e.validateTargets(platform, svc)

}

func (e *ExposeTester) validateTargets(platform types.Platform, svc types.ServiceInterface) (bool, error) {
	// No targets found
	if len(svc.Targets) == 0 {
		svcStr, _ := json.MarshalIndent(svc, "", "    ")
		return true, fmt.Errorf("expose command failed as service interface has no targets - found: %s", string(svcStr))
	}

	// Validating target name exists
	found := false
	for _, target := range svc.Targets {
		expectedTargetName := e.TargetName
		if platform.IsKubernetes() {
			if target.Name == expectedTargetName {
				found = true
				break
			}
		} else if platform == types.PlatformPodman {
			if target.Service == e.Address {
				found = true
				break
			}
		}
	}
	if !found {
		return true, fmt.Errorf("no target has been found for given target name")
	}

	return true, nil
}
