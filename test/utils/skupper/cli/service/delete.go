package service

import (
	"context"
	"log"

	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteTester runs `skupper service delete` and asserts service has been
// removed from the cluster and also from skupper resources.
type DeleteTester struct {
	Name string
}

func (s *DeleteTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "service", "delete", s.Name)
	return args
}

func (s *DeleteTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute service delete command
	stdout, stderr, err = cli.RunSkupperCli(s.Command(cluster))
	if err != nil {
		return
	}

	// Validating service has been deleted
	log.Printf("Validating 'skupper service delete'")
	ctx, cancelFn := context.WithTimeout(context.Background(), constants.ImagePullingAndResourceCreationTimeout)
	defer cancelFn()
	attempt := 0
	err = utils.RetryWithContext(ctx, constants.DefaultTick, func() (bool, error) {
		attempt++
		log.Printf("validating service deleted - attempt: %d", attempt)
		_, err := cluster.VanClient.KubeClient.CoreV1().Services(cluster.Namespace).Get(ctx, s.Name, v1.GetOptions{})
		if err == nil {
			log.Printf("service %s still available", s.Name)
			return false, nil
		}
		return true, nil
	})
	return
}
