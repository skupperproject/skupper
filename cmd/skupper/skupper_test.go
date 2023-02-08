package main

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"

	"gotest.tools/assert"
)

func TestParseTargetTypeAndName(t *testing.T) {
	targetType, targetName := parseTargetTypeAndName([]string{"type", "name"})
	assert.Equal(t, targetType, "type")
	assert.Equal(t, targetName, "name")

	targetType, targetName = parseTargetTypeAndName([]string{"type/name"})
	assert.Equal(t, targetType, "type")
	assert.Equal(t, targetName, "name")
}

func TestBindArgs(t *testing.T) {
	s := &SkupperKubeService{}
	genericError := "Service name, target type and target name must all be specified (e.g. 'skupper bind <service-name> <target-type> <target-name>')"
	b := func(args []string) error {
		return s.bindArgs(nil, args)
	}

	assert.Error(t, b([]string{}), genericError)
	assert.Error(t, b([]string{"oneArg"}), genericError)
	assert.Error(t, b([]string{"one/Arg"}), genericError)
	assert.Error(t, b([]string{"one", "resource"}), genericError)

	// must this fail?
	// assert.Error(t, b([]string{"one/two", "resource/name"}), genericError)

	assert.Error(t, b([]string{"one", "resource/name"}), "target type must be one of: [deployment, statefulset, pods, service, deploymentconfig]")

	assert.Assert(t, b([]string{"one", "pods/name"}))
	assert.Assert(t, b([]string{"one", "pods", "name"}))

	// note  illegal vs extra
	assert.Error(t, b([]string{"one", "resource/name", "three"}), "extra argument: three")
	assert.Error(t, b([]string{"one", "resource/name", "three", "four"}), "illegal argument: four")
	assert.Error(t, b([]string{"one", "resource/name", "three", "four", "five"}), "illegal argument: four")

	assert.Error(t, b([]string{"one", "resource", "name", "four"}), "illegal argument: four")
	assert.Error(t, b([]string{"one", "resource", "name", "four", "five"}), "illegal argument: four")
}

func TestCreateServiceArgs(t *testing.T) {
	c := func(args []string) error {
		return createServiceArgs(nil, args)
	}

	assert.Error(t, c([]string{}), "Name and port(s) must be specified")
	assert.Error(t, c([]string{"noport"}), "Name and port(s) must be specified")

	assert.Assert(t, c([]string{"service:port"}))

	assert.Error(t, c([]string{"service:port", "other"}), "other is not a valid port")
	assert.Error(t, c([]string{"service:port", "other", "arg"}), "other is not a valid port")

	assert.Error(t, c([]string{"service", "port"}), "port is not a valid port")
	assert.Error(t, c([]string{"service", "port", "other"}), "port is not a valid port")
	assert.Error(t, c([]string{"service", "port", "other", "arg"}), "port is not a valid port")
}

func TestCreateServiceParseArgs(t *testing.T) {
	cmd := NewCmdCreateService(NewSkupperTestClient().Service())

	assert.Assert(t, cmd.ParseFlags([]string{}))
	assert.Equal(t, serviceToCreate.Protocol, "tcp")
	assert.Equal(t, serviceToCreate.TlsCredentials, "")

	cmdArgs := []string{"service:8080", "--protocol", "http2", "--enable-tls"}

	assert.Assert(t, cmd.ParseFlags(cmdArgs))
	assert.Equal(t, serviceToCreate.Protocol, "http2")

	// Deprecated flag should work until it is removed.
	cmdArgs = []string{"service:8080", "--mapping", "http2", "--enable-tls"}

	assert.Assert(t, cmd.ParseFlags(cmdArgs))
	assert.Equal(t, serviceToCreate.Protocol, "http2")

}

func TestExposeTargetArgs(t *testing.T) {
	s := &SkupperKubeService{}
	genericError := "expose target and name must be specified (e.g. 'skupper expose deployment <name>'"
	targetError := "target type must be one of: [deployment, statefulset, pods, service, deploymentconfig]"

	e := func(args []string) error {
		return s.ExposeArgs(nil, args)
	}

	assert.Error(t, e([]string{}), genericError)
	assert.Error(t, e([]string{"depl/name"}), targetError)

	assert.Error(t, e([]string{"depl/name", "two"}), "extra argument: two")
	assert.Error(t, e([]string{"depl/name", "two", "three"}), "illegal argument: three")
	assert.Error(t, e([]string{"depl/name", "two", "three", "four"}), "illegal argument: three")

	assert.Error(t, e([]string{"depl/name"}), targetError)
	assert.Error(t, e([]string{"anything", "name"}), targetError)

	assert.Error(t, e([]string{"deployment"}), genericError)

	assert.Assert(t, e([]string{"deployment", "name"}))

	assert.Error(t, e([]string{"deployment", "name", "three"}), "illegal argument: three")
	assert.Error(t, e([]string{"deployment", "name", "three", "four"}), "illegal argument: three")

	for _, target := range validExposeTargetsKube {
		assert.Assert(t, e([]string{target, "name"}))
	}
}

func TestExposeParseArgs(t *testing.T) {
	cmd_args := []string{"deployment/name", "--address", "theAddress"}
	cmd := NewCmdExpose(NewSkupperTestClient().Service())

	assert.Assert(t, cmd.ParseFlags([]string{}))
	assert.Equal(t, exposeOpts.Address, "")

	assert.Assert(t, cmd.ParseFlags(cmd_args))
	assert.Equal(t, exposeOpts.Address, "theAddress")

	cmdArgs := []string{"deployment/name", "--address", "theAddress", "--protocol", "http2", "--enable-tls"}

	assert.Assert(t, cmd.ParseFlags(cmdArgs))
	assert.Equal(t, exposeOpts.GeneratedCerts, true)
	assert.Equal(t, exposeOpts.Protocol, "http2")
	assert.Equal(t, exposeOpts.TlsCredentials, "")
}

func TestExposePublishNotReadyAddressesParseArgs(t *testing.T) {
	cmdArgs := []string{"deployment/name", "--publish-not-ready-addresses"}
	cmd := NewCmdExpose(NewSkupperTestClient().Service())

	assert.Assert(t, cmd.ParseFlags(cmdArgs))
	assert.Equal(t, exposeOpts.PublishNotReadyAddresses, true)

	cmdArgs2 := []string{"statefulset/web", "--headless", "--publish-not-ready-addresses"}
	cmd2 := NewCmdExpose(NewSkupperTestClient().Service())
	assert.Assert(t, cmd2.ParseFlags(cmdArgs2))
	assert.Equal(t, exposeOpts.Headless, true)
	assert.Equal(t, exposeOpts.PublishNotReadyAddresses, true)
}

var clusterRun = flag.Bool("use-cluster", false, "run tests against a configured cluster")

func TestBindGatewayArgs(t *testing.T) {
	genericError := "Service address, target host and port must all be specified"
	b := func(args []string) error {
		return bindGatewayArgs(nil, args)
	}

	assert.Error(t, b([]string{}), genericError)
	assert.Error(t, b([]string{"oneArg"}), genericError)
	assert.Error(t, b([]string{"oneArg", "twoArg"}), genericError)
	assert.Error(t, b([]string{"oneArg", "twoArg", "threeArg"}), "threeArg is not a valid port")

	assert.Assert(t, b([]string{"oneArg", "twoArg", "8080"}))
	assert.Assert(t, b([]string{"oneArg", "twoArg", "8080", "9090"}))
	assert.Assert(t, b([]string{"oneArg", "twoArg:threeArg"}))

	// note  illegal vs extra
	assert.Error(t, b([]string{"oneArg", "twoArg:threeArg", "fourArg"}), "extra argument: fourArg")
	assert.Error(t, b([]string{"oneArg", "twoArg", "threeArg", "fourArg"}), "threeArg is not a valid port")
	assert.Error(t, b([]string{"oneArg", "twoArg", "threeArg", "fourArg", "fiveArg"}), "threeArg is not a valid port")
}

func TestExposeGatewayArgs(t *testing.T) {
	genericError := "Gateway service address, target host and port must all be specified"
	b := func(args []string) error {
		return exposeGatewayArgs(nil, args)
	}

	assert.Error(t, b([]string{}), genericError)
	assert.Error(t, b([]string{"oneArg"}), genericError)
	assert.Error(t, b([]string{"oneArg", "twoArg"}), genericError)
	assert.Error(t, b([]string{"oneArg", "twoArg", "threeArg"}), "threeArg is not a valid port")
	assert.Error(t, b([]string{"oneArg", "twoArg", "8080:threeArg"}), "8080:threeArg is not a valid port")
	assert.Error(t, b([]string{"oneArg", "twoArg", "threeArg:8080"}), "threeArg:8080 is not a valid port")

	assert.Assert(t, b([]string{"oneArg", "twoArg:threeArg"}))
	assert.Assert(t, b([]string{"oneArg", "twoArg", "8080"}))
	assert.Assert(t, b([]string{"oneArg", "twoArg", "8080", "9090"}))
	assert.Assert(t, b([]string{"oneArg", "twoArg", "8080:8081", "9090"}))
	assert.Assert(t, b([]string{"oneArg", "twoArg", "8080", "9090:9191"}))
	assert.Assert(t, b([]string{"oneArg", "twoArg", "8080:8081", "9090:9191"}))

	// note  illegal vs extra
	assert.Error(t, b([]string{"oneArg", "twoArg:threeArg", "fourArg"}), "extra argument: fourArg")
	assert.Error(t, b([]string{"oneArg", "twoArg", "threeArg", "fourArg"}), "threeArg is not a valid port")
	assert.Error(t, b([]string{"oneArg", "twoArg", "threeArg", "fourArg", "fiveArg"}), "threeArg is not a valid port")
}

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestSkupperInitConfigSyncParseArgs(t *testing.T) {
	cmd := NewCmdInit(NewSkupperTestClient().Site())

	assert.Assert(t, cmd.ParseFlags([]string{}))
	assert.Equal(t, routerCreateOpts.ConfigSync.Cpu, "")
	assert.Equal(t, routerCreateOpts.ConfigSync.CpuLimit, "")
	assert.Equal(t, routerCreateOpts.ConfigSync.Memory, "")
	assert.Equal(t, routerCreateOpts.ConfigSync.MemoryLimit, "")
	assert.Equal(t, routerCreateOpts.Controller.NodeSelector, "")
	assert.Equal(t, routerCreateOpts.Controller.Affinity, "")
	assert.Equal(t, routerCreateOpts.Controller.AntiAffinity, "")

	cmdArgs := []string{"--config-sync-cpu", "1", "--config-sync-memory", "2G", "--config-sync-cpu-limit", "2", "--config-sync-memory-limit", "3G"}

	assert.Assert(t, cmd.ParseFlags(cmdArgs))
	assert.Equal(t, routerCreateOpts.ConfigSync.Cpu, "1")
	assert.Equal(t, routerCreateOpts.ConfigSync.CpuLimit, "2")
	assert.Equal(t, routerCreateOpts.ConfigSync.Memory, "2G")
	assert.Equal(t, routerCreateOpts.ConfigSync.MemoryLimit, "3G")
	assert.Equal(t, routerCreateOpts.Controller.NodeSelector, "")
	assert.Equal(t, routerCreateOpts.Controller.Affinity, "")
	assert.Equal(t, routerCreateOpts.Controller.AntiAffinity, "")

}

func TestSkupperInitControllerParseArgs(t *testing.T) {
	cmd := NewCmdInit(NewSkupperTestClient().Site())

	assert.Assert(t, cmd.ParseFlags([]string{}))
	assert.Equal(t, routerCreateOpts.Controller.Cpu, "")
	assert.Equal(t, routerCreateOpts.Controller.CpuLimit, "")
	assert.Equal(t, routerCreateOpts.Controller.Memory, "")
	assert.Equal(t, routerCreateOpts.Controller.MemoryLimit, "")
	assert.Equal(t, routerCreateOpts.Controller.NodeSelector, "")
	assert.Equal(t, routerCreateOpts.Controller.Affinity, "")
	assert.Equal(t, routerCreateOpts.Controller.AntiAffinity, "")

	assert.Equal(t, routerCreateOpts.Controller.IngressHost, "")
	assert.Equal(t, len(routerCreateOpts.Controller.ServiceAnnotations), 0)
	assert.Equal(t, routerCreateOpts.Controller.LoadBalancerIp, "")

	cmdArgs := []string{"--controller-cpu", "1", "--controller-memory", "2G", "--controller-cpu-limit", "2", "--controller-memory-limit", "3G"}

	assert.Assert(t, cmd.ParseFlags(cmdArgs))
	assert.Equal(t, routerCreateOpts.Controller.Cpu, "1")
	assert.Equal(t, routerCreateOpts.Controller.CpuLimit, "2")
	assert.Equal(t, routerCreateOpts.Controller.Memory, "2G")
	assert.Equal(t, routerCreateOpts.Controller.MemoryLimit, "3G")
	assert.Equal(t, routerCreateOpts.Controller.NodeSelector, "")
	assert.Equal(t, routerCreateOpts.Controller.Affinity, "")
	assert.Equal(t, routerCreateOpts.Controller.AntiAffinity, "")

	assert.Equal(t, routerCreateOpts.Controller.IngressHost, "")
	assert.Equal(t, len(routerCreateOpts.Controller.ServiceAnnotations), 0)
	assert.Equal(t, routerCreateOpts.Controller.LoadBalancerIp, "")

}

func TestSkupperInitTimeoutParseArgs(t *testing.T) {
	cmd := NewCmdInit(NewSkupperTestClient().Site())

	assert.Assert(t, cmd.ParseFlags([]string{}))
	assert.Equal(t, LoadBalancerTimeout, types.DefaultTimeoutDuration)

	cmdArgs := []string{"--timeout", "3m"}

	assert.Assert(t, cmd.ParseFlags(cmdArgs))
	assert.Equal(t, LoadBalancerTimeout, time.Minute*3)

}

func TestSkupperInitRunAsParseArgs(t *testing.T) {
	cmd := NewCmdInit(NewSkupperTestClient().Site())

	assert.Assert(t, cmd.ParseFlags([]string{}))
	assert.Equal(t, routerCreateOpts.RunAsUser, int64(0))
	assert.Equal(t, routerCreateOpts.RunAsGroup, int64(0))

	cmdArgs := []string{"--run-as-user", "1000"}

	assert.Assert(t, cmd.ParseFlags(cmdArgs))
	assert.Equal(t, routerCreateOpts.RunAsUser, int64(1000))

	cmdArgs = []string{"--run-as-group", "2000"}

	assert.Assert(t, cmd.ParseFlags(cmdArgs))
	assert.Equal(t, routerCreateOpts.RunAsGroup, int64(2000))
}
