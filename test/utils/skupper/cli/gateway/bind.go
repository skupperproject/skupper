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
	EgressPort      []string
	Name            string
	Protocol        string
	IsGatewayActive bool
}

func (b *BindTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "gateway", "bind", b.Address, b.Host)
	args = append(args, b.EgressPort...)

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
	for _, ingressPort := range si.Ports {
		expectedOut := fmt.Sprintf(`.* CREATE .*%s:%d .*%s`, b.Address, ingressPort, b.EgressPort)
		matched, _ := regexp.MatchString(expectedOut, stderr)
		if b.IsGatewayActive && !matched {
			// Sample output
			// 2021/10/26 17:10:25 CREATE io.skupper.router.tcpConnector fgiorget-fgiorget-egress-tcp-echo-host:9090 map[address:tcp-echo-host:9090 host:0.0.0.0 name:fgiorget-fgiorget-egress-tcp-echo-host:9090 port:46173 siteId:ee953910-681a-4bc5-b139-78bbbe45f6b3]
			err = fmt.Errorf("output does not contain expected content - found: %s", stderr)
			return
		}
	}

	// Validating service bind definition
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
		var bind types.GatewayEndpoint
		found := false
		for i, ingressPort := range si.Ports {
			for k, v := range gw.Connectors {
				if strings.Contains(k, b.Address+":"+strconv.Itoa(ingressPort)) {
					bind = v
					found = true
					break
				}
			}
			if !found {
				err = fmt.Errorf("service bind not found")
				return
			}
			if bind.Service.Address != fmt.Sprintf("%s:%d", b.Address, ingressPort) {
				err = fmt.Errorf("service address is incorrect - expected: %s:%d - found: %s", b.Address, ingressPort, bind.Service.Address)
			}
			if strconv.Itoa(bind.Service.Ports[i]) != b.EgressPort[i] {
				err = fmt.Errorf("service port is incorrect - expected: %s - found: %d", b.EgressPort[i], bind.Service.Ports[i])
			}
			if bind.Host != b.Host {
				err = fmt.Errorf("service host is incorrect - expected: %s - found: %s", b.Host, bind.Host)
			}
		}
	}

	return
}
