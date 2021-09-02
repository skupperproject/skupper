package service

import (
	"context"
	"fmt"
	"log"
	"regexp"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
	utils2 "github.com/skupperproject/skupper/test/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// StatusTester runs `skupper service status` and asserts that its
// output contains the provided service interfaces (or until it
// times out).
type StatusTester struct {
	ServiceInterfaces []types.ServiceInterface
}

func (s *StatusTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "service", "status")
	return args
}

func (s *StatusTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// The service status command needs to be executed multiple times, till expected
	// results can be observed or until it times out
	ctx, cancelFn := context.WithTimeout(context.Background(), constants.ImagePullingAndResourceCreationTimeout)
	defer cancelFn()
	attempt := 0
	err = utils.RetryWithContext(ctx, constants.DefaultTick, func() (bool, error) {
		attempt++
		stdout, stderr, err = s.run(cluster)
		log.Printf("Validating 'skupper service status' - attempt %d", attempt)
		if err != nil {
			log.Printf("error executing service status command: %v", err)
			return false, nil
		}
		return true, nil
	})

	return
}

func (s *StatusTester) run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute service create command
	stdout, stderr, err = cli.RunSkupperCli(s.Command(cluster))
	if err != nil {
		return
	}

	// Iterating through provided service interfaces to validate stdout matches
	for _, svc := range s.ServiceInterfaces {
		serviceEntry := fmt.Sprintf(`.*%s \(%s port %d\)`, svc.Address, svc.Protocol, svc.Port)
		if len(svc.Targets) > 0 {
			serviceEntry += `\n.*Targets:`
		}
		r := regexp.MustCompile(serviceEntry)
		if !r.MatchString(stdout) {
			err = fmt.Errorf("expected: %s - found: %s", serviceEntry, stdout)
			return
		}

		// Validating if provided targets are showing up
		for _, target := range svc.Targets {
			targetRegex := regexp.MustCompile(fmt.Sprintf("%s name=%s", utils2.StrDefault(target.Service, ".*"), target.Name))
			if !targetRegex.MatchString(stdout) {
				err = fmt.Errorf("expected target not found - regexp: %s - stdout: %s", targetRegex.String(), stdout)
				return
			}
		}
	}

	return
}
