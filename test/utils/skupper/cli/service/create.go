package service

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateTester runs `skupper service create` and asserts that
// the expected resources are defined in the cluster.
type CreateTester struct {
	Name    string
	Port    int
	Mapping string
}

func (s *CreateTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "service", "create", s.Name, strconv.Itoa(s.Port))

	if s.Mapping != "" {
		args = append(args, "--mapping", s.Mapping)
	}

	return args
}

func (s *CreateTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute service create command
	stdout, stderr, err = cli.RunSkupperCli(s.Command(cluster))
	if err != nil {
		return
	}

	// Validating service has been created
	log.Printf("Validating 'skupper service create'")
	ctx, cancelFn := context.WithTimeout(context.Background(), constants.ImagePullingAndResourceCreationTimeout)
	defer cancelFn()
	attempt := 0
	err = utils.RetryWithContext(ctx, constants.DefaultTick, func() (bool, error) {
		attempt++
		log.Printf("validating created service - attempt: %d", attempt)
		svc, err := cluster.VanClient.KubeClient.CoreV1().Services(cluster.Namespace).Get(s.Name, v1.GetOptions{})
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
	})
	return
}
