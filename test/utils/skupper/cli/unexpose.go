package cli

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	utils2 "github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UnexposeTester runs `skupper unexpose` and validates outcome
// and asserts service has been effectively removed
type UnexposeTester struct {
	TargetType string
	TargetName string
	Address    string
}

func (e *UnexposeTester) Command(cluster *base.ClusterContext) []string {
	args := SkupperCommonOptions(cluster)
	args = append(args, "unexpose", e.TargetType, e.TargetName)

	// Flags
	if e.Address != "" {
		args = append(args, "--address", e.Address)
	}
	return args
}

func (e *UnexposeTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute unexpose command
	stdout, stderr, err = RunSkupperCli(e.Command(cluster))
	if err != nil {
		return
	}

	// Validating stdout contains expected data
	log.Printf("Validating 'skupper unexpose'")
	expectedOut := fmt.Sprintf("%s %s unexposed", e.TargetType, e.TargetName)
	if !strings.Contains(stdout, expectedOut) {
		err = fmt.Errorf("expected: %s - found: %s", expectedOut, stdout)
		return
	}

	attempt := 0
	ctx, fn := context.WithTimeout(context.Background(), commandTimeout)
	defer fn()
	err = utils2.RetryWithContext(ctx, constants.DefaultTick, func() (bool, error) {
		attempt++
		log.Printf("validating service after unexpose completed - attempt: %d", attempt)
		// Service should have been removed
		expectedAddress := utils.StrDefault(e.Address, e.TargetName)
		log.Printf("validating service %s has been removed", expectedAddress)
		_, err = cluster.VanClient.KubeClient.CoreV1().Services(cluster.Namespace).Get(ctx, expectedAddress, v1.GetOptions{})
		if err == nil {
			log.Printf("service %s still exists", expectedAddress)
			return false, nil
		}
		// Service removed from config map
		log.Printf("validating service %s no longer exists in %s config map", expectedAddress, types.ServiceInterfaceConfigMap)
		cm, err := cluster.VanClient.KubeClient.CoreV1().ConfigMaps(cluster.Namespace).Get(ctx, types.ServiceInterfaceConfigMap, v1.GetOptions{})
		if err != nil {
			return true, fmt.Errorf("unable to find %s config map - %v", types.ServiceInterfaceConfigMap, err)
		}

		// retrieving data
		_, ok := cm.Data[expectedAddress]
		if ok {
			log.Printf("address %s is still defined at %s", expectedAddress, types.ServiceInterfaceConfigMap)
			return false, nil
		}

		return true, nil
	})

	return
}
