package cli

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteTester allows running and validating `skupper delete`.
type DeleteTester struct {
	// This will ignore the error condition where delete failed because skupper
	// was not installed in the first place.  It can be used in the situations
	// where you just want to make sure skupper is not on the namespace
	IgnoreNotInstalled bool
}

func (d *DeleteTester) Command(platform types.Platform, cluster *base.ClusterContext) []string {
	args := SkupperCommonOptions(platform, cluster)
	args = append(args, "delete")
	return args
}

func (d *DeleteTester) Run(platform types.Platform, cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute delete command
	stdout, stderr, err = RunSkupperCli(d.Command(platform, cluster))
	if err != nil {
		notInstalledMsg := strings.Contains(stderr, "Skupper not installed") || strings.Contains(stderr, "Skupper is not enabled for")
		if d.IgnoreNotInstalled && notInstalledMsg {
			err = nil
		}
		return
	}

	// expected output: Skupper is now removed from '<NAMESPACE>'.
	log.Printf("Validating 'skupper delete'")
	var expectedOutput string
	if platform == types.PlatformPodman {
		// TODO Removed broken v1 implementation
		err = fmt.Errorf("broken implementation")
		return

	} else {
		expectedOutput = fmt.Sprintf("Skupper is now removed from '%s'.", cluster.Namespace)
	}
	if !strings.Contains(stdout, expectedOutput) {
		err = fmt.Errorf("expected: %s - found: %s", expectedOutput, stdout)
		return
	}

	// retry until all main resources have gone
	ctx, fn := context.WithTimeout(context.Background(), constants.NamespaceDeleteTimeout)
	defer fn()
	attempt := 0
	err = utils.RetryWithContext(ctx, constants.DefaultTick, func() (bool, error) {
		attempt++
		log.Printf("validating skupper resources have been removed - attempt: %d", attempt)
		if platform == types.PlatformPodman {
			return d.validatePodmanRemoved()
		}
		return d.validateKubeRemoved(ctx, cluster)
	})

	return
}

func (d *DeleteTester) validateKubeRemoved(ctx context.Context, cluster *base.ClusterContext) (bool, error) {
	// site config is gone
	_, err := cluster.VanClient.KubeClient.CoreV1().ConfigMaps(cluster.Namespace).Get(ctx, types.SiteConfigMapName, metav1.GetOptions{})
	if err == nil {
		log.Printf("skupper-site config map still exists")
		return false, nil
	}

	// router config is gone
	_, err = kube.GetConfigMap(types.TransportConfigMapName, cluster.Namespace, cluster.VanClient.KubeClient)
	if err == nil {
		log.Printf("%s config map still exists", types.TransportConfigMapName)
		return false, nil
	}

	// router deployment is gone
	_, err = cluster.VanClient.KubeClient.AppsV1().Deployments(cluster.Namespace).Get(ctx, types.TransportDeploymentName, metav1.GetOptions{})
	if err == nil {
		log.Printf("%s deployment still exists", types.TransportDeploymentName)
		return false, nil
	}

	// controller deployment is gone
	_, err = cluster.VanClient.KubeClient.AppsV1().Deployments(cluster.Namespace).Get(ctx, types.ControllerDeploymentName, metav1.GetOptions{})
	if err == nil {
		log.Printf("%s deployment still exists", types.ControllerDeploymentName)
		return false, nil
	}

	return true, nil
}

func (d *DeleteTester) validatePodmanRemoved() (bool, error) {
	// TODO Removed broken v1 implementation
	return false, fmt.Errorf("broken implementation")
}
