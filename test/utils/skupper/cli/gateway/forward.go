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

// ForwardTester runs `skupper gateway forward` and asserts that
// the a local port is now forwarding requests to the cluster
type ForwardTester struct {
	Address         string
	Port            []string
	Loopback        bool
	Mapping         string
	Name            string
	IsGatewayActive bool
}

func (f *ForwardTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "gateway", "forward", f.Address)
	args = append(args, f.Port...)

	if f.Loopback {
		args = append(args, "--loopback")
	}
	if f.Mapping != "" {
		args = append(args, "--mapping", f.Mapping)
	}
	if f.Name != "" {
		args = append(args, "--name", f.Name)
	}

	return args
}

func (f *ForwardTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute the gateway forward command
	stdout, stderr, err = cli.RunSkupperCli(f.Command(cluster))
	if err != nil {
		return
	}

	// Validating service bind definition
	ctx := context.Background()
	var si *types.ServiceInterface
	si, err = cluster.VanClient.ServiceInterfaceInspect(ctx, f.Address)
	if err != nil {
		return
	}

	// Basic validation of the stdout (only valid for active gateways)
	for _, ingressPort := range si.Ports {
		expectedOut := fmt.Sprintf(`.* CREATE .*%s:%d .*%s`, f.Address, ingressPort, f.Port)
		matched, _ := regexp.MatchString(expectedOut, stderr)
		if f.IsGatewayActive && !matched {
			// Sample output
			// 2021/10/26 17:22:51 CREATE io.skupper.router.tcpListener fgiorget-fgiorget-ingress-tcp-echo-cluster:9090 map[address:tcp-echo-cluster:9090 host:0.0.0.0 name:fgiorget-fgiorget-ingress-tcp-echo-cluster:9090 port:40373 siteId:dcd0eed1-5c44-4817-b409-6b0cdf18dae4]
			err = fmt.Errorf("output does not contain expected content - found: %s", stderr)
			return
		}
	}

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
		for i, ingressPort := range si.Ports {
			// finding the correct listener
			var forward types.GatewayEndpoint
			found := false
			for k, v := range gw.Listeners {
				if strings.HasSuffix(k, f.Address+":"+strconv.Itoa(ingressPort)) {
					forward = v
					found = true
					break
				}
			}
			if !found {
				err = fmt.Errorf("service forward not found")
				return
			}
			if forward.Service.Address != fmt.Sprintf("%s:%d", f.Address, ingressPort) {
				err = fmt.Errorf("service address is incorrect - expected: %s:%d - found: %s", f.Address, ingressPort, forward.Service.Address)
			}
			if strconv.Itoa(forward.Service.Ports[i]) != f.Port[i] {
				err = fmt.Errorf("service port is incorrect - expected: %s - found: %d", f.Port[i], forward.Service.Ports[i])
			}
			expectedHost := ""
			if f.Loopback {
				expectedHost = "127.0.0.1"
			}
			if forward.Host != expectedHost {
				err = fmt.Errorf("service host is incorrect - expected: %s - found: %s", expectedHost, forward.Host)
			}
		}
	}

	return
}
