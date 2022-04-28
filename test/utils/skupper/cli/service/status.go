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
	ServiceInterfaces             []types.ServiceInterface
	UnauthorizedServiceInterfaces []types.ServiceInterface
	Absent                        bool

	// By default, unauthorized interfaces count as good on ServiceInterfaces;
	// if this is set to true, then a service listed in ServiceInterfaces that
	// is reported as unauthorized will be reported as an error
	CheckAuthorization bool
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
		log.Printf("Validating 'skupper service status' - attempt %d", attempt)
		if base.IsTestInterrupted() {
			return false, fmt.Errorf("Test interrupted")
		}
		if base.IsMaxStatusAttemptsReached(attempt) {
			return false, fmt.Errorf("Maximum attempts reached")
		}
		stdout, stderr, err = s.run(cluster)
		if err != nil {
			log.Printf("error executing service status command: %v\nstdout:\n %s\nstderr:\n %s", err, stdout, stderr)
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
		serviceEntry := fmt.Sprintf(`.*%s \(%s port %d\)`, svc.Address, svc.Protocol, svc.Ports[0])
		if len(svc.Targets) > 0 && !s.Absent {
			serviceEntry += `\n.*Targets:`
		}
		r := regexp.MustCompile(serviceEntry)
		if r.MatchString(stdout) == s.Absent {
			err = fmt.Errorf("expected:\n%s\nAbsent: %t\nfound:\n%s\n", serviceEntry, s.Absent, stdout)
			return
		}

		if !s.Absent {
			// Validating if provided targets are showing up
			for _, target := range svc.Targets {
				targetRegex := regexp.MustCompile(fmt.Sprintf("%s name=%s", utils2.StrDefault(target.Service, ".*"), target.Name))
				if !targetRegex.MatchString(stdout) {
					err = fmt.Errorf("expected target not found - regexp: %s - stdout: %s", targetRegex.String(), stdout)
					return
				}
			}
			// Confirming that it is not unauthorized
			if s.CheckAuthorization {
				authCheck := serviceEntry + " - not authorized"
				rAuthCheck := regexp.MustCompile(authCheck)
				if rAuthCheck.MatchString(stdout) {
					err = fmt.Errorf("service was expected to be authorized, but it is not.\nregexp: %s\nstdout: %s", rAuthCheck.String(), stdout)
					return
				}
			}
		}
	}

	for _, svc := range s.UnauthorizedServiceInterfaces {
		serviceEntry := fmt.Sprintf(`.*%s \(%s port %d\) - not authorized`, svc.Address, svc.Protocol, svc.Ports[0])
		r := regexp.MustCompile(serviceEntry)
		if !r.MatchString(stdout) {
			err = fmt.Errorf("expected unauthorized service not found:\n%s\nstdout:\n%s\n", serviceEntry, stdout)
			return
		}
	}

	return
}
