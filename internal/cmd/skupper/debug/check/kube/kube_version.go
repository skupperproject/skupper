package kube

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/check/cli"
	"github.com/skupperproject/skupper/internal/cmd/skupper/debug/check/command"
	"github.com/skupperproject/skupper/internal/kube/client"
	"k8s.io/apimachinery/pkg/version"
)

const (
	minKubeMajor = 1
	minKubeMinor = 24
)

var checkK8sVersion = newKubeCheckCommand(
	"kube-version",
	"the Kubernetes version is supported",
	kubeVersionRun,
	&checkK8sAccess,
)

func NewCmdCheckK8sVersion() command.Check {
	return checkK8sVersion
}

func kubeVersionRun(status cli.Reporter, kubeClient *client.KubeClient) error {
	version, err := kubeClient.Kube.Discovery().ServerVersion()
	if err != nil {
		return status.Error(err, "Failed to retrieve the Kubernetes API server version")
	}

	return status.Error(checkVersion(version), "the Kubernetes version is not supported")
}

func checkVersion(ver *version.Info) error {
	major, err := strconv.Atoi(ver.Major)
	if err != nil {
		return fmt.Errorf("error parsing API server major version %v: %w", ver.Major, err)
	}

	if major > minKubeMajor {
		return nil
	}

	var minor int
	if strings.HasSuffix(ver.Minor, "+") {
		minor, err = strconv.Atoi(ver.Minor[0 : len(ver.Minor)-1])
	} else {
		minor, err = strconv.Atoi(ver.Minor)
	}

	if err != nil {
		return fmt.Errorf("error parsing API server minor version %v: %w", ver.Minor, err)
	}

	if major < minKubeMajor || minor < minKubeMinor {
		return fmt.Errorf("installed Kubernetes version %s.%s is too old; Skupper requires at least %d.%d",
			ver.Major, ver.Minor, minKubeMajor, minKubeMinor)
	}

	return nil
}
