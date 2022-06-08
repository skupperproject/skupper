package service

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

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
	// TODO REVIEW rename and use []types.ServiceInterface
	Absent bool

	// By default, if the service interface includes no targets, the test is
	// going to be successful, even if the actual response from the command
	// indicates it has targets (bindings).  This changes that behavior: if
	// the requested ServiceInterface had an empty Targets slice, it will be
	// an error if the command lists a bound target (or if the target lists
	// differ)
	CheckNotBound bool

	// By default, unauthorized interfaces count as good on ServiceInterfaces;
	// if this is set to true, then a service listed in ServiceInterfaces that
	// is reported as unauthorized will be reported as an error
	CheckAuthorization bool

	// By default, the command checks that what it has configured in
	// ServiceInterfaces is listed on the output.  If the option below is set
	// to true, it will also ensure that it is the whole list, and no other
	// interfaces are listed on the output
	StrictInterfaceListCheck bool
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
		log.Printf("Validating 'skupper service status' - attempt %d", attempt)
		if base.IsTestInterrupted() {
			return false, fmt.Errorf("Test interrupted")
		}
		if base.IsMaxStatusAttemptsReached(attempt) {
			return false, fmt.Errorf("Maximum attempts reached")
		}
		attempt++

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

	if s.CheckNotBound || s.StrictInterfaceListCheck {
		err = s.checkBindings(stdout)
		if err != nil {
			return
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

// As we do not have JSON output, for ensuring the output is as
// expected, we have to parse it, line by line, and create a
// corresponding struct.  An alternative would be to get this
// information from `get services -o json`, but that would not
// be validating the command's output
//
// This ties this test very tightly to the command output, which
// may make the tests fragile.
func (s *StatusTester) checkBindings(stdout string) (err error) {
	listedServices, err := s.parseBindings(stdout)
	if err != nil {
		return
	}

	err = s.compareBindings(listedServices)

	return
}

// This compares the listedServices to s.ServiceInterfaces; it expects that
// both the list of services be the same, and the list of targets on each
// service be the same.
//
// It currently does not check ports or protocols, only addresses and names
func (s *StatusTester) compareBindings(listedServices []*types.ServiceInterface) (err error) {
	structMap := map[string]*types.ServiceInterface{}
	listedMap := map[string]*types.ServiceInterface{}

	for i, iface := range s.ServiceInterfaces {
		structMap[iface.Address] = &s.ServiceInterfaces[i]
	}
	for _, iface := range listedServices {
		listedMap[iface.Address] = iface
	}

	for n, structInterface := range structMap {
		listedInterface, ok := listedMap[n]
		if !ok {
			return fmt.Errorf("Interface %v was expected, but not listed", n)
		}

		structTargetMap := map[string]types.ServiceInterfaceTarget{}
		listedTargetMap := map[string]types.ServiceInterfaceTarget{}
		for _, t := range structInterface.Targets {
			structTargetMap[t.Name] = t
		}
		for _, t := range listedInterface.Targets {
			listedTargetMap[t.Name] = t
		}

		for tn := range structTargetMap {
			_, ok := listedTargetMap[tn]
			if !ok {
				return fmt.Errorf("Target %v was expected on interface %v, but not listed", tn, n)
			}
			delete(listedTargetMap, tn)
		}
		if len(listedTargetMap) > 0 {
			remainingList := make([]string, 0, len(listedTargetMap))
			for tn := range listedTargetMap {
				remainingList = append(remainingList, tn)
			}
			return fmt.Errorf("The following targets were listed for interface %v, but were not expected: %v", n, strings.Join(remainingList, ", "))
		}
		delete(listedMap, n)
	}
	if len(listedMap) > 0 && s.StrictInterfaceListCheck {
		remainingList := make([]string, 0, len(listedMap))
		for tn := range listedMap {
			remainingList = append(remainingList, tn)
		}
		return fmt.Errorf("The following interfaces were listed, but were not expected: %v", strings.Join(remainingList, ", "))
	}

	return
}

var secondLevel = regexp.MustCompile("^( |│)  (╰|├).*")
var thirdLevel = regexp.MustCompile("^( |│)  ( |│)  (╰|├).*")

// This does the actual parsing of the command's output
//
// It returns []*types.ServiceInterface, but with incomplete information,
// based on what the status command shows
//
// The parsing is incomplete: it does not deal with the situation where
// no services are listed at all, for example.  In the future, it may be
// extended to include that and other scenarios, and perhaps replace the
// regexp matches from s.run(), but for now it provides only the bits
// necessary for the policy testing.
func (s *StatusTester) parseBindings(stdout string) (ifaces []*types.ServiceInterface, err error) {

	var scanner = bufio.NewScanner(strings.NewReader(stdout))
	var line int
	var iface *types.ServiceInterface

	for scanner.Scan() {
		line++
		text := scanner.Text()
		if strings.HasPrefix(text, "Services exposed") {
			if line != 1 {
				err = fmt.Errorf("Header found in unexpected place (line %v) - parsing failed", line)
				return
			}
			// First line, nothing to see here
			continue
		}
		if strings.HasPrefix(text, "├") || strings.HasPrefix(text, "╰") {
			// First level definition: service
			pieces := strings.Split(text, " ")
			if pieces[3] != "port" {
				err = fmt.Errorf("Parsing failed due to unexpected service output on line %v: %v", line, text)
				return
			}
			port, converr := strconv.Atoi(pieces[4][:len(pieces[4])-1])
			if err != nil {
				err = fmt.Errorf("Failed to parse service port: %w", converr)
				return
			}
			iface = &types.ServiceInterface{
				Address:  pieces[1],
				Protocol: pieces[2][1:],
				Ports:    []int{port},
				Targets:  []types.ServiceInterfaceTarget{},
			}
			ifaces = append(ifaces, iface)
			continue
		}
		if secondLevel.MatchString(text) {
			// I'm only expecting the line "Targets" to be on this indentation level,
			// so fail otherwise
			if !strings.HasSuffix(text, "Targets:") {
				err = fmt.Errorf("Unexpected output where 'Targets:' was expected on line %v: %v", line, text)
				return
			}
			continue
		}
		if thirdLevel.MatchString(text) {
			// Third level definition: target
			if iface == nil {
				err = fmt.Errorf("Failed parsing line %v - target definition without a preceding service definition: %v", line, text)
				return
			}
			// We first get what's to the right of ╰─
			pieces := strings.Split(strings.Trim(text, " "), "─ ")
			if len(pieces) != 2 {
				err = fmt.Errorf("Parsing failed due to unexpected target output on line %v: %v", line, text)
				return
			}
			// Then we get the second item, that should be the name
			pieces = strings.Split(strings.Trim(pieces[1], " "), " ")
			if len(pieces) != 2 {
				err = fmt.Errorf("Parsing failed due to unexpected target output format on line %v: %v", line, text)
				return
			}
			id := strings.SplitN(pieces[1], "=", 2)
			if len(id) > 2 || id[0] != "name" {
				err = fmt.Errorf("Parsing failed due to unexpected target specification on line %v: %v", line, text)
				return
			}

			target := types.ServiceInterfaceTarget{
				Name: id[1],
			}

			iface.Targets = append(iface.Targets, target)
			continue
		}
		err = fmt.Errorf("Parsing failed on line %v (unknown): %v", line, text)
	}

	return

}
