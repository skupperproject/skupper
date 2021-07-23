package gateway

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
)

// ForwardTester runs `skupper gateway forward` and asserts that
// the a local port is now forwarding requests to the cluster
type ForwardTester struct {
	Address         string
	Port            string
	Loopback        bool
	Mapping         string
	Name            string
	IsGatewayActive bool
}

func (f *ForwardTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "gateway", "forward", f.Address, f.Port)

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

	// Basic validation of the stdout (only valid for active gateways)
	matched, _ := regexp.MatchString(fmt.Sprintf(`.* CREATE .*%s .*%s`, f.Address, f.Port), stderr)
	if !f.IsGatewayActive && !matched {
		// Sample output
		// 2021/07/28 18:24:44 CREATE org.apache.qpid.dispatch.tcpListener localhost.localdomain-user-ingress-tcp-echo-cluster map[address:tcp-echo-cluster host:0.0.0.0 name:localhost.localdomain-user-ingress-tcp-echo-cluster port:9090]
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
		gwName = gwList[0].GatewayName
	}

	for _, gw := range gwList {
		if gwName != gw.GatewayName {
			continue
		}
		// finding the correct listener
		var forward types.GatewayEndpoint
		found := false
		for k, v := range gw.TcpListeners {
			if strings.HasSuffix(k, f.Address) {
				forward = v
				found = true
				break
			}
		}
		if !found {
			err = fmt.Errorf("service forward not bound")
			return
		}
		if forward.Address != f.Address {
			err = fmt.Errorf("service address is incorrect - expected: %s - found: %s", f.Address, forward.Address)
		}
		if forward.Port != f.Port {
			err = fmt.Errorf("service port is incorrect - expected: %s - found: %s", f.Port, forward.Port)
		}
		expectedHost := "0.0.0.0"
		if f.Loopback {
			expectedHost = "127.0.0.1"
		}
		if forward.Host != expectedHost {
			err = fmt.Errorf("service host is incorrect - expected: %s - found: %s", expectedHost, forward.Host)
		}
	}

	return
}
