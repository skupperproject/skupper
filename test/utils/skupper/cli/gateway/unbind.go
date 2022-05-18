package gateway

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// UnbindTester runs `skupper gateway unbind` and asserts that
// the gateway service is no longer bound to a cluster service
type UnbindTester struct {
	Address  string
	Name     string
	Protocol string
}

func (b *UnbindTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "gateway", "unbind", b.Address)

	if b.Name != "" {
		args = append(args, "--name", b.Name)
	}
	if b.Protocol != "" {
		args = append(args, "--protocol", b.Protocol)
	}

	return args
}

func (b *UnbindTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute the gateway unbind command
	stdout, stderr, err = cli.RunSkupperCli(b.Command(cluster))
	if err != nil {
		return
	}

	// Basic validation of the stdout
	if matched, _ := regexp.MatchString(fmt.Sprintf(`.* DELETE .*%s`, b.Address), stderr); !matched {
		// Sample output
		// 2021/07/28 23:28:04 DELETE io.skupper.router.tcpConnector localhost.localdomain-user-egress-mongo-host
		err = fmt.Errorf("output does not contain expected content - found: %s", stdout)
		return
	}

	// Validating service bind definition
	ctx := context.Background()
	gwList, err := cluster.VanClient.GatewayList(ctx)

	gwName := b.Name
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
		// finding the correct connector
		found := false
		for k, _ := range gw.Connectors {
			if strings.HasSuffix(k, b.Address) {
				found = true
				break
			}
		}
		if found {
			err = fmt.Errorf("service bind not removed")
			return
		}
	}

	return
}
