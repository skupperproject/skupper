package link

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	"log"
	"regexp"
	"strconv"
	"strings"
)

// StatusTester runs `skupper link status` based on given attributes
// and waits till output matches expected content or until it times out
type StatusTester struct {
	Name    string
	Wait    int
	Active  bool
	Failure ClaimFailure
}

type ClaimFailure string

const (
	ClaimInvalid ClaimFailure = "No such claim"
	ClaimRefused ClaimFailure = "Claim refused"
)

func (l *StatusTester) Command(platform types.Platform, cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(platform, cluster)
	args = append(args, "link", "status")

	if l.Name != "" {
		args = append(args, l.Name)
	}

	if l.Wait > 0 {
		args = append(args, "--wait", strconv.Itoa(l.Wait))
	}
	return args
}

func (l *StatusTester) Run(platform types.Platform, cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// The link status command needs to be executed multiple times, till expected
	// results can be observed or until it times out
	ctx, cancelFn := context.WithTimeout(context.Background(), constants.ImagePullingAndResourceCreationTimeout)
	defer cancelFn()
	attempt := 0
	err = utils.RetryWithContext(ctx, constants.DefaultTick, func() (bool, error) {
		if base.IsTestInterrupted() {
			err = fmt.Errorf("Test was interrupted")
			return false, err
		}
		if base.IsMaxStatusAttemptsReached(attempt) {
			return false, fmt.Errorf("Maximum attempts reached")
		}
		attempt++

		stdout, stderr, err = l.run(platform, cluster)
		log.Printf("Validating 'skupper link status' - attempt %d", attempt)
		if err != nil {
			log.Printf("error executing link status command: %v", err)
			return false, nil
		}
		return true, nil
	})

	return
}

func (l *StatusTester) run(platform types.Platform, cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute link status command
	stdout, stderr, err = cli.RunSkupperCli(l.Command(platform, cluster))
	if err != nil {
		return
	}

	// connection name
	connName := l.Name
	if connName == "" {
		connName = "link[0-9]+"
	}

	// prefix for expected connection outcome
	activePrefix := "is"
	if !l.Active {
		activePrefix = "not"
	}

	// strip \n from stdout
	lines := strings.Split(stdout, "\n")

	// the link status command is returning local links and remote links separated in two sections
	for _, line := range lines {
		if strings.Contains(line, "Link link") {
			stdout = line
			break
		}
	}

	// if a failure is expected
	failureStr := ""
	if string(l.Failure) != "" {
		failureStr = fmt.Sprintf(` \(Failed to redeem claim: %s\)`, l.Failure)
	}
	outRegex := regexp.MustCompile(fmt.Sprintf(`Link %s %s connected%s`, connName, activePrefix, failureStr))

	// Ensure stdout matches expected regexp
	if !outRegex.MatchString(stdout) {
		err = fmt.Errorf("expected output does not match - \nfound: \n%s\nregexp: \n%s", stdout, outRegex.String())
		return
	}

	return
}
