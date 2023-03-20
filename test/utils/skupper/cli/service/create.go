package service

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	clientpodman "github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

type PodmanCreateOptions struct {
	ContainerName string
	HostIp        string
	HostPorts     []string
	Labels        map[string]string
}

// CreateTester runs `skupper service create` and asserts that
// the expected resources are defined in the cluster.
type CreateTester struct {
	Name            string
	Port            int
	Mapping         string
	PolicyProhibits bool
	Podman          PodmanCreateOptions
}

func (s *CreateTester) Command(platform types.Platform, cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(platform, cluster)
	args = append(args, "service", "create", s.Name, strconv.Itoa(s.Port))

	if s.Mapping != "" {
		args = append(args, "--protocol", s.Mapping)
	}

	//
	// podman options
	//
	if s.Podman.ContainerName != "" {
		args = append(args, "--container-name", s.Podman.ContainerName)
	}
	if s.Podman.HostIp != "" {
		args = append(args, "--host-ip", s.Podman.HostIp)
	}
	for _, port := range s.Podman.HostPorts {
		args = append(args, "--host-port", port)
	}
	for key, value := range s.Podman.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", key, value))
	}

	return args
}

func (s *CreateTester) Run(platform types.Platform, cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute service create command
	stdout, stderr, err = cli.RunSkupperCli(s.Command(platform, cluster))
	if err != nil {
		if s.PolicyProhibits {
			err = cli.Expect{
				StdErr: []string{
					"Error: Policy validation error: service",
					s.Name,
					"cannot be created",
				},
			}.Check(stdout, stderr)
			return
		}
		return
	} else {
		if s.PolicyProhibits {
			err = fmt.Errorf("Policy error was expected, but not encountered")
			return
		}
	}

	// Validating service has been created
	log.Printf("Validating 'skupper service create'")
	ctx, cancelFn := context.WithTimeout(context.Background(), constants.ImagePullingAndResourceCreationTimeout)
	defer cancelFn()
	attempt := 0
	err = utils.RetryWithContext(ctx, constants.DefaultTick, func() (bool, error) {
		if base.IsTestInterrupted() {
			return false, fmt.Errorf("Test interrupted")
		}
		if base.IsMaxStatusAttemptsReached(attempt) {
			// Even though this is a createTester, we're running a
			// status step, so we check for maximum attempts configuration
			return false, fmt.Errorf("Maximum attempts reached")
		}
		attempt++

		log.Printf("validating created service - attempt: %d", attempt)
		if platform == types.PlatformPodman {
			return s.validatePodman()
		}
		return s.validateKubernetes(cluster)
	})
	return
}

func (s *CreateTester) validateKubernetes(cluster *base.ClusterContext) (bool, error) {
	svc, err := cluster.VanClient.KubeClient.CoreV1().Services(cluster.Namespace).Get(context.TODO(), s.Name, v1.GetOptions{})
	if err != nil {
		log.Printf("service %s not available yet", s.Name)
		return false, nil
	}
	for _, port := range svc.Spec.Ports {
		if s.Port != int(port.Port) {
			return true, fmt.Errorf("incorrect port defined on created service - expected: %d - found: %d",
				s.Port, port.Port)
		}
	}
	return true, nil
}

func (s *CreateTester) validatePodman() (bool, error) {
	cli, err := clientpodman.NewPodmanClient("", "")
	if err != nil {
		return true, err
	}
	containerName := utils.DefaultStr(s.Podman.ContainerName, s.Name)
	c, err := cli.ContainerInspect(containerName)
	if err != nil {
		return false, nil
	}

	// verifying service definition
	svcHandler := podman.NewServiceHandlerPodman(cli)
	svc, err := svcHandler.Get(s.Name)
	if err != nil {
		return true, fmt.Errorf("service definition not found - %w", err)
	}

	// validating service ports
	if s.Port != svc.GetPorts()[0] {
		return true, fmt.Errorf("incorrect ports defined - expecting: [%d] - found: %v", s.Port, svc.GetPorts())
	}

	// validate host binding
	if s.Podman.HostIp != "" {
		if s.Podman.HostIp != c.Ports[0].HostIP {
			return true, fmt.Errorf("host ip does not match - expecting: %s - found: %s", s.Podman.HostIp, c.Ports[0].HostIP)
		}
	}

	// validate host ports
	if len(s.Podman.HostPorts) > 0 {
		for _, hostPort := range s.Podman.HostPorts {
			hostPortSlice := strings.Split(hostPort, ":")
			hostPort := hostPortSlice[0]
			if len(hostPortSlice) > 1 {
				hostPort = hostPortSlice[1]
			}
			svcPort := hostPortSlice[0]

			found := false
			for _, cPort := range c.Ports {
				if cPort.Target == svcPort && cPort.Host == hostPort {
					found = true
					break
				}
			}
			if !found {
				return true, fmt.Errorf("host port binding not found: %s - container ports: %v", hostPort, c.Ports)
			}
		}
	}

	// validating labels
	if len(s.Podman.Labels) > 0 {
		for k, v := range s.Podman.Labels {
			if cv, ok := c.Labels[k]; !ok {
				return true, fmt.Errorf("container label is missing: %s", k)
			} else if v != cv {
				return true, fmt.Errorf("container label does not match - expected: %s=%s - found: %s=%s", k, v, k, cv)
			}
		}
	}
	return true, nil
}
