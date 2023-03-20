package service

import (
	"context"
	"log"

	"github.com/skupperproject/skupper/api/types"
	clientpodman "github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	skuppercli "github.com/skupperproject/skupper/test/utils/skupper/cli"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteTester runs `skupper service delete` and asserts service has been
// removed from the cluster and also from skupper resources.
type DeleteTester struct {
	Name string
}

func (s *DeleteTester) Command(platform types.Platform, cluster *base.ClusterContext) []string {
	args := skuppercli.SkupperCommonOptions(platform, cluster)
	args = append(args, "service", "delete", s.Name)
	return args
}

func (s *DeleteTester) Run(platform types.Platform, cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute service delete command
	stdout, stderr, err = skuppercli.RunSkupperCli(s.Command(platform, cluster))
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
		if platform.IsKubernetes() {
			_, err = cluster.VanClient.KubeClient.CoreV1().Services(cluster.Namespace).Get(ctx, s.Name, v1.GetOptions{})
		} else if platform == types.PlatformPodman {
			cli, _ := clientpodman.NewPodmanClient("", "")
			svcHandler := podman.NewServiceHandlerPodman(cli)
			_, err = svcHandler.Get(s.Name)
		}
		if err == nil {
			log.Printf("service %s still available", s.Name)
			return false, nil
		}
		return true, nil
	})
	return
}
