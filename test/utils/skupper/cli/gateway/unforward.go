package gateway

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// UnforwardTester runs `skupper gateway unforward` and asserts that
// the a local port is no longer forwarding requests to the cluster
type UnforwardTester struct {
	Address  string
	Protocol string
	Name     string
}

func (f *UnforwardTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "gateway", "unforward", f.Address)

	if f.Protocol != "" {
		args = append(args, "--protocol", f.Protocol)
	}
	if f.Name != "" {
		args = append(args, "--name", f.Name)
	}

	return args
}

func (f *UnforwardTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute the gateway unforward command
	stdout, stderr, err = cli.RunSkupperCli(f.Command(cluster))
	if err != nil {
		return
	}

	// Basic validation of the stdout
	if matched, _ := regexp.MatchString(fmt.Sprintf(`.* DELETE .*%s`, f.Address), stderr); !matched {
		// Sample output
		// 2021/07/28 20:16:14 DELETE io.skupper.router.tcpListener localhost.localdomain-user-ingress-tcp-echo-cluster
		err = fmt.Errorf("output does not contain expected content - found: %s", stdout)
		return
	}

	// Validating service bind definition
	ctx := context.Background()
	gwList, err := cluster.VanClient.GatewayList(ctx)

	gwName := f.Name
	if gwName == "" {
		// If no gateway name provided and there are many gateways, no further validation can be done
		if len(gwList) > 1 {
			return
		}
		gwName = gwList[0].Name
	}

	for _, gw := range gwList {
		if gwName != gw.Name {
			continue
		}
		// finding the correct listener
		found := false
		for k := range gw.Listeners {
			if strings.HasSuffix(k, f.Address) {
				found = true
				break
			}
		}
		if found {
			err = fmt.Errorf("service forward not removed")
			return
		}
	}

	return
}
