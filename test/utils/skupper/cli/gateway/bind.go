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
	Port            string
	Name            string
	Protocol        string
	IsGatewayActive bool
}

func (b *BindTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "gateway", "bind", b.Address, b.Host, b.Port)

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

	// Basic validation of the stdout (only when gateway is active)
	matched, _ := regexp.MatchString(fmt.Sprintf(`.* CREATE .*%s .*%s`, b.Address, b.Port), stderr)
	if !b.IsGatewayActive && !matched {
		// Sample output
		// 2021/07/28 14:10:49 CREATE org.apache.qpid.dispatch.tcpConnector localhost.localdomain-user-egress-tcp-go-host map[address:tcp-go-host host:0.0.0.0 name:localhost.localdomain-user-egress-tcp-go-host port:9090]
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
		if bind.Service.Address != b.Address {
			err = fmt.Errorf("service address is incorrect - expected: %s - found: %s", b.Address, bind.Service.Address)
		}
		if strconv.Itoa(bind.Service.Port) != b.Port {
			err = fmt.Errorf("service port is incorrect - expected: %s - found: %d", b.Port, bind.Service.Port)
		}
		if bind.Host != b.Host {
			err = fmt.Errorf("service host is incorrect - expected: %s - found: %s", b.Host, bind.Host)
		}
	}

	return
}
