package gateway

import (
	"context"
	"fmt"
	"os"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeleteTester struct {
	Name string
}

func (d *DeleteTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "gateway", "delete")

	if d.Name != "" {
		args = append(args, "--name", d.Name)
	}
	return args
}

func (d *DeleteTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	ctx := context.Background()
	preGateways, err := cluster.VanClient.GatewayList(ctx)
	if err != nil {
		return
	}
	if len(preGateways) == 0 {
		err = fmt.Errorf("no existing gateways found")
		return
	}

	// Execute the gateway delete command
	stdout, stderr, err = cli.RunSkupperCli(d.Command(cluster))
	if err != nil {
		return
	}

	//
	// Retrieve updated list of gateways
	//
	postGateways, err := cluster.VanClient.GatewayList(ctx)
	if err != nil {
		return
	}

	// If i.Name is empty we need to discover the deleted gateway name
	gatewayName := d.Name
	if gatewayName == "" {
		if len(postGateways) == 0 {
			gatewayName = preGateways[0].Name
		} else if len(postGateways) < len(preGateways) {
			for _, preGw := range preGateways {
				found := false
				for _, postGw := range postGateways {
					if preGw.Name == postGw.Name {
						found = true
					}
				}
				if !found {
					gatewayName = preGw.Name
					break
				}
			}
			if gatewayName == "" {
				err = fmt.Errorf("unable to discover gateway name")
				return
			}
		} else {
			err = fmt.Errorf("gateway has not been removed")
			return
		}
	} else {
		found := false
		for _, existingGw := range postGateways {
			if existingGw.Name == gatewayName {
				found = true
				break
			}
		}
		if found {
			err = fmt.Errorf("gateway %s still exists", gatewayName)
			return
		}
	}

	// Validate router config files and local user service resources removed
	configDir := GetSkupperDataHome() + "/" + gatewayName
	_, err = os.Stat(configDir)
	if err == nil {
		err = fmt.Errorf("configuration directory still exists: %s", configDir)
		return
	}

	serviceFile := GetSystemdUserHome() + "/" + gatewayName + ".service"
	_, err = os.Stat(serviceFile)
	if err == nil {
		err = fmt.Errorf("user service definition still exists: %s", serviceFile)
		return
	}

	//
	// Retrieve ConfigMap with skupper.io/type: gateway-definition (label)
	//
	cmList, err := cluster.VanClient.KubeClient.CoreV1().ConfigMaps(cluster.Namespace).List(v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", types.SkupperTypeQualifier, "gateway-definition"),
	})
	if err != nil {
		return
	}

	for _, cm := range cmList.Items {
		gwName, ok := cm.Annotations["skupper.io/gateway-name"]
		if ok && gwName == gatewayName {
			fmt.Errorf("gateway configmap still exists")
		}
	}

	err = nil
	return
}
