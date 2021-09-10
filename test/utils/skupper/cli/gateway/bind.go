package gateway

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// BindTester runs `skupper gateway bind` and asserts that
// the gateway service is bound to a cluster service
type BindTester struct {
	Address         string
	Host            string
	EgressPort      string
	Name            string
	Protocol        string
	IsGatewayActive bool
}

func (b *BindTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "gateway", "bind", b.Address, b.Host, b.EgressPort)

	if b.Name != "" {
		args = append(args, "--name", b.Name)
	}
	if b.Protocol != "" {
		args = append(args, "--protocol", b.Protocol)
	}

	return args
}

func (b *BindTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute the gateway bind command
	stdout, stderr, err = cli.RunSkupperCli(b.Command(cluster))
	if err != nil {
		return
	}

	// Retrieving service info
	ctx := context.Background()
	var si *types.ServiceInterface
	si, err = cluster.VanClient.ServiceInterfaceInspect(ctx, b.Address)
	if err != nil {
		return
	}

	// Basic validation of the stdout (only when gateway is active)
	expectedOut := fmt.Sprintf(`.* CREATE .*%s:%d .*%s`, b.Address, si.Ports[0], b.EgressPort)
	matched, _ := regexp.MatchString(expectedOut, stderr)
	if b.IsGatewayActive && !matched {
		// Sample output
		// 2021/09/24 12:14:07 CREATE org.apache.qpid.dispatch.tcpConnector gw1-egress-tcp-echo-host                     map[address:tcp-echo-host:9090 host:0.0.0.0 name:gw1-egress-tcp-echo-host port:9090]
		err = fmt.Errorf("output does not contain expected content - found: %s", stderr)
		return
	}

	// Validating service bind definition
	gwList, err := cluster.VanClient.GatewayList(ctx)

	gwName := b.Name
	if gwName == "" {
		// If no gateway name provided and there are many gateways, no further validation can be done
		if len(gwList) > 1 {
			return
		}
		gwName = gwList[0].GatewayName
	}

	for _, gw := range gwList {
		if gwName != gw.GatewayName {
			continue
		}
		// finding the correct connector
		var bind types.GatewayEndpoint
		found := false
		for k, v := range gw.GatewayConnectors {
			if strings.HasSuffix(k, b.Address) {
				bind = v
				found = true
				break
			}
		}
		if !found {
			err = fmt.Errorf("service bind not bound")
			return
		}
		if bind.Service.Address != fmt.Sprintf("%s:%d", b.Address, si.Ports[0]) {
			err = fmt.Errorf("service address is incorrect - expected: %s:%d - found: %s", b.Address, si.Ports[0], bind.Service.Address)
		}
		if strconv.Itoa(bind.Service.Ports[0]) != b.EgressPort {
			err = fmt.Errorf("service port is incorrect - expected: %s - found: %d", b.EgressPort, bind.Service.Ports[0])
		}
		if bind.Host != b.Host {
			err = fmt.Errorf("service host is incorrect - expected: %s - found: %s", b.Host, bind.Host)
		}
	}

	return
}
