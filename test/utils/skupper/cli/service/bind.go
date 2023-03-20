package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/skupperproject/skupper/api/types"
	clientpodman "github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BindTester runs `skupper service bind` and validate skupper resources
// to assert that service has the corresponding target
type BindTester struct {
	ServiceName string
	TargetType  string
	TargetName  string
	TargetPort  int

	ExpectServiceNotFound bool
	PolicyProhibits       bool
}

func (s *BindTester) Command(platform types.Platform, cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(platform, cluster)
	args = append(args, "service", "bind", s.ServiceName, s.TargetType, s.TargetName)

	if s.TargetPort > 0 {
		args = append(args, "--target-port", strconv.Itoa(s.TargetPort))
	}

	return args
}

func (s *BindTester) Run(platform types.Platform, cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute service bind command
	stdout, stderr, err = cli.RunSkupperCli(s.Command(platform, cluster))
	if err != nil {
		if s.ExpectServiceNotFound {
			err = cli.Expect{
				StdErr: []string{"Error: Service", "not found"},
			}.Check(stdout, stderr)
			// if string found (err==nil), then no service.  We're good and nothing else to check
			// if string not found (err!=nil), service is there, and we did not expect it; fail
			// in either case, nothing else to do here
			return
		}
		if s.PolicyProhibits {
			err = cli.Expect{
				StdErr: []string{
					"Policy validation error:",
					fmt.Sprintf("%v/%v", s.TargetType, s.TargetName),
					"cannot be exposed",
				},
			}.Check(stdout, stderr)
			return
		}
		return
	} else {
		if s.ExpectServiceNotFound {
			err = fmt.Errorf("Command was expected to fail with Service Not Found, but it didn't")
			return
		}
		if s.PolicyProhibits {
			err = fmt.Errorf("Policy error was expected, but not encountered")
			return
		}
	}

	// Verifying the skupper-services config map to ensure a target has been defined
	log.Println("Validating 'skupper service bind'")
	var svc *types.ServiceInterface
	if platform.IsKubernetes() {
		var cm *corev1.ConfigMap
		log.Printf("validating service %s exists in %s config map", s.ServiceName, types.ServiceInterfaceConfigMap)
		cm, err = cluster.VanClient.KubeClient.CoreV1().ConfigMaps(cluster.Namespace).Get(context.TODO(), types.ServiceInterfaceConfigMap, v1.GetOptions{})
		if err != nil {
			err = fmt.Errorf("unable to find %s config map - %v", types.ServiceInterfaceConfigMap, err)
			return
		}

		// retrieving data
		svcStr, ok := cm.Data[s.ServiceName]
		if !ok {
			err = fmt.Errorf("service %s is not defined at %s", s.ServiceName, types.ServiceInterfaceConfigMap)
			return
		}

		// Unmarshalling and verifying targets
		svc = &types.ServiceInterface{}
		err = json.Unmarshal([]byte(svcStr), svc)
		if err != nil {
			return
		}
	} else if platform == types.PlatformPodman {
		var cli *clientpodman.PodmanRestClient
		cli, err = clientpodman.NewPodmanClient("", "")
		if err != nil {
			err = fmt.Errorf("unable to create podman client")
			return
		}
		svcIfaceHandler := podman.NewServiceInterfaceHandlerPodman(cli)
		svc, err = svcIfaceHandler.Get(s.ServiceName)
		if err != nil {
			return
		}
	}

	// No targets found
	svcStr, _ := json.Marshal(svc)
	if len(svc.Targets) == 0 {
		err = fmt.Errorf("bind command failed as service interface has no targets - found: %s", string(svcStr))
		return
	}

	// Validating target name exists
	found := false
	for _, target := range svc.Targets {
		if platform.IsKubernetes() {
			if target.Name == s.TargetName {
				found = true
				break
			}
		} else if platform == types.PlatformPodman {
			if target.Service == s.ServiceName {
				found = true
				break
			}
		}
	}
	if !found {
		err = fmt.Errorf("no target has been found for given target name")
		return
	}

	return
}
