package cli

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
)

// StatusTester runs `skupper status` and validates outcome defined
// attributes. It runs continuously till output matches expected
// content or until it times out.
type StatusTester struct {
	RouterMode             string
	SiteName               string
	ConnectedSites         int
	ConnectedSitesIndirect int
	ExposedServices        int
	ConsoleEnabled         bool
	ConsoleAuthInternal    bool
	NotEnabled             bool
	PolicyEnabled          *bool
}

func (s *StatusTester) Command(cluster *base.ClusterContext) []string {
	args := SkupperCommonOptions(cluster)
	return append(args, "status")
}

func (s *StatusTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {

	// The status command needs to be executed multiple times, till expected
	// results can be observed or until it times out
	ctx, cancelFn := context.WithTimeout(context.Background(), constants.ImagePullingAndResourceCreationTimeout)
	defer cancelFn()
	attempt := 0
	err = utils.RetryWithContext(ctx, constants.DefaultTick, func() (bool, error) {
		if base.IsTestInterrupted() {
			return false, fmt.Errorf("Test interrupted")
		}
		if base.IsMaxStatusAttemptsReached(attempt) {
			return false, fmt.Errorf("Maximum attempts reached")
		}
		attempt++

		stdout, stderr, err = s.run(cluster)
		log.Printf("Validating 'skupper status' - attempt %d", attempt)
		if err != nil {
			log.Printf("error executing status command: %v", err)
			return false, nil
		}
		return true, nil
	})

	return
}

func (s *StatusTester) run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {

	stdout, stderr, err = RunSkupperCli(s.Command(cluster))
	if err != nil {
		return
	}

	// Validating main output
	if err = s.validateMainContent(cluster, stdout); err != nil {
		return
	}

	// Validating Console Enabled
	if err = s.validateConsoleEnabled(cluster, stdout); err != nil {
		return
	}

	// Validating Console Auth Internal
	if err = s.validateConsoleAuthInternal(cluster, stdout); err != nil {
		return
	}

	return
}

func (s *StatusTester) validateMainContent(cluster *base.ClusterContext, stdout string) error {

	log.Println("Validating main content")

	// Composing how output should be validated (based on provided attributes)
	mainContent := []string{}
	notExpected := []regexp.Regexp{}

	// Main info
	mainContent = append(mainContent, fmt.Sprintf("Skupper is enabled for namespace \"%s\"", cluster.Namespace))
	if s.NotEnabled {
		notEnabledContent := fmt.Sprintf("Skupper is not enabled in namespace '%s'", cluster.Namespace)
		if !strings.Contains(stdout, notEnabledContent) {
			return fmt.Errorf("error validating not enabled message - expected: %s - stdout: %s", notEnabledContent, stdout)
		}
		// when not enabled, there is nothing else to validate
		return nil
	}

	// Site name variant
	if s.SiteName != "" {
		mainContent = append(mainContent, fmt.Sprintf("with site name \"%s\"", s.SiteName))
	}

	// Router mode
	routerMode := "interior"
	if s.RouterMode != "" {
		routerMode = s.RouterMode
	}
	mainContent = append(mainContent, fmt.Sprintf("in %s mode", routerMode))

	// Policy checking, if defined
	if s.PolicyEnabled != nil {
		if *s.PolicyEnabled {
			mainContent = append(mainContent, "(with policies)")
		} else {
			notExpected = append(notExpected, *regexp.MustCompile("(with policies)"))
		}
	}

	// Connected sites variant
	connectedSites := "It is not connected to any other sites."
	if s.ConnectedSites == 1 {
		connectedSites = fmt.Sprintf("It is connected to %d other site.", s.ConnectedSites)
	} else if s.ConnectedSites > 1 && s.ConnectedSitesIndirect == 0 {
		connectedSites = fmt.Sprintf("It is connected to %d other sites.", s.ConnectedSites)
	} else if s.ConnectedSites > 1 && s.ConnectedSitesIndirect > 0 {
		connectedSites = fmt.Sprintf("It is connected to %d other sites (%d indirectly).", s.ConnectedSites, s.ConnectedSitesIndirect)
	}
	mainContent = append(mainContent, connectedSites)

	// Exposed services variant
	exposedServices := "It has no exposed services."
	if s.ExposedServices == 1 {
		exposedServices = "It has 1 exposed service."
	} else if s.ExposedServices > 1 {
		exposedServices = fmt.Sprintf("It has %d exposed services.", s.ExposedServices)
	}
	mainContent = append(mainContent, exposedServices)

	expect := Expect{
		StdOut:      mainContent,
		StdOutReNot: notExpected,
	}

	return expect.Check(stdout, "")
}

func (s *StatusTester) validateConsoleEnabled(cluster *base.ClusterContext, stdout string) error {
	if !s.ConsoleEnabled {
		return nil
	}

	log.Println("Validating console enabled info")
	if !strings.Contains(stdout, "The site console url is: ") {
		return fmt.Errorf("site console url is missing")
	}

	return nil
}

func (s *StatusTester) validateConsoleAuthInternal(cluster *base.ClusterContext, stdout string) error {
	if !s.ConsoleEnabled {
		return nil
	}

	log.Println("Validating console auth internal info")
	if !strings.Contains(stdout, "The credentials for internal console-auth mode are held in secret: 'skupper-console-users'") {
		return fmt.Errorf("credentials info for internal console-auth mode is missing")
	}

	return nil
}
