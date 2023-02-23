package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/skupperproject/skupper/api/types"
	clientpodman "github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/test/utils/base"
	skuppercli "github.com/skupperproject/skupper/test/utils/skupper/cli"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UnbindTester runs `skupper service unbind` and asserts that
// the corresponding service no longer has the given target.
type UnbindTester struct {
	ServiceName string
	TargetType  string
	TargetName  string
}

func (s *UnbindTester) Command(platform types.Platform, cluster *base.ClusterContext) []string {
	args := skuppercli.SkupperCommonOptions(platform, cluster)
	args = append(args, "service", "unbind", s.ServiceName, s.TargetType, s.TargetName)
	return args
}

func (s *UnbindTester) Run(platform types.Platform, cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute service unbind command
	stdout, stderr, err = skuppercli.RunSkupperCli(s.Command(platform, cluster))
	if err != nil {
		return
	}

	// Verifying the skupper-services config map to ensure the target has been removed
	log.Println("Validating 'skupper service unbind'")
	var svc *types.ServiceInterface
	if platform == types.PlatformPodman {
		svc, err = s.getPodmanService()
	} else {
		svc, err = s.getKubernetesService(cluster)
	}
	if err != nil {
		return
	}

	// Validating target name no longer exists
	found := false
	for _, target := range svc.Targets {
		if platform == types.PlatformPodman {
			if target.Service == s.ServiceName {
				found = true
				break
			}
		} else {
			if target.Name == s.TargetName {
				found = true
				break
			}
		}
	}
	if found {
		err = fmt.Errorf("target still exists for the given target name")
		return
	}

	return
}

func (s *UnbindTester) getKubernetesService(cluster *base.ClusterContext) (*types.ServiceInterface, error) {
	var svc types.ServiceInterface
	var err error
	log.Printf("validating service %s exists in %s config map", s.ServiceName, types.ServiceInterfaceConfigMap)
	cm, err := cluster.VanClient.KubeClient.CoreV1().ConfigMaps(cluster.Namespace).Get(context.TODO(), types.ServiceInterfaceConfigMap, v1.GetOptions{})
	if err != nil {
		err = fmt.Errorf("unable to find %s config map - %v", types.ServiceInterfaceConfigMap, err)
		return nil, err
	}

	// retrieving data
	svcStr, ok := cm.Data[s.ServiceName]
	if !ok {
		return nil, fmt.Errorf("service %s is not defined at %s", s.ServiceName, types.ServiceInterfaceConfigMap)
	}

	// Unmarshalling and verifying targets
	err = json.Unmarshal([]byte(svcStr), &svc)
	if err != nil {
		return nil, err
	}

	return &svc, nil
}

func (s *UnbindTester) getPodmanService() (*types.ServiceInterface, error) {
	cli, err := clientpodman.NewPodmanClient("", "")
	if err != nil {
		return nil, err
	}
	svcIfaceHandler := podman.NewServiceInterfaceHandlerPodman(cli)
	return svcIfaceHandler.Get(s.ServiceName)
}
