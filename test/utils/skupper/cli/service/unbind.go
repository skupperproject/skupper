package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UnbindTester runs `skupper service unbind` and asserts that
// the corresponding service no longer has the given target.
type UnbindTester struct {
	ServiceName string
	TargetType  string
	TargetName  string
}

func (s *UnbindTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "service", "unbind", s.ServiceName, s.TargetType, s.TargetName)
	return args
}

func (s *UnbindTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute service unbind command
	stdout, stderr, err = cli.RunSkupperCli(s.Command(cluster))
	if err != nil {
		return
	}

	// Verifying the skupper-services config map to ensure the target has been removed
	log.Println("Validating 'skupper service unbind'")
	log.Printf("validating service %s exists in %s config map", s.ServiceName, types.ServiceInterfaceConfigMap)
	cm, err := cluster.VanClient.KubeClient.CoreV1().ConfigMaps(cluster.Namespace).Get(context.TODO(), types.ServiceInterfaceConfigMap, v1.GetOptions{})
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
	var svc types.ServiceInterface
	err = json.Unmarshal([]byte(svcStr), &svc)
	if err != nil {
		return
	}

	// Validating target name no longer exists
	found := false
	for _, target := range svc.Targets {
		if target.Name == s.TargetName {
			found = true
			break
		}
	}
	if found {
		err = fmt.Errorf("target still exists for the given target name")
		return
	}

	return
}
