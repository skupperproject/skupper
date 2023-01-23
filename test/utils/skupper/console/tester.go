package console

import (
	"context"
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/utils/base"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func IsConsoleEnabled(cluster *base.ClusterContext) bool {
	svc, err := cluster.VanClient.KubeClient.CoreV1().Services(cluster.Namespace).Get(context.TODO(), types.ControllerServiceName, v1.GetOptions{})
	if err != nil {
		return false
	}

	if svc == nil {
		return false
	}

	// Need to find the 8080 port in the skupper service
	for _, port := range svc.Spec.Ports {
		if port.Port == 8080 {
			return true
		}
	}

	return false
}

func GetInternalCredentials(cluster *base.ClusterContext) (error, string, string) {
	secret, err := cluster.VanClient.KubeClient.CoreV1().Secrets(cluster.Namespace).Get(context.TODO(), "skupper-console-users", v1.GetOptions{})
	if err != nil {
		return err, "", ""
	}

	// verify that secret contains data with admin key
	for user, pass := range secret.Data {
		return nil, user, string(pass)
	}

	return fmt.Errorf("no internal credentials found"), "", ""
}
