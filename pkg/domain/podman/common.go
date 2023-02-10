package podman

import (
	"fmt"
	"github.com/skupperproject/skupper/pkg/utils"
	"os"
	"os/user"
	"strings"

	"github.com/skupperproject/skupper/api/types"
)

const (
	SharedTlsCertificates = "skupper-router-certs"
)

var (
	Username                = readUsername()
	SkupperContainerVolumes = []string{"skupper-services", "skupper-local-server", "skupper-internal", "skupper-site-server", SharedTlsCertificates}
)

func OwnedBySkupper(resource string, labels map[string]string) error {
	notOwnedErr := fmt.Errorf("%s is not owned by Skupper", resource)
	if labels == nil {
		return notOwnedErr
	}
	if app, ok := labels["application"]; !ok || app != types.AppName {
		return notOwnedErr
	}
	return nil
}

func readUsername() string {
	u, err := user.Current()
	if err != nil {
		return utils.DefaultStr(os.Getenv("USER"), os.Getenv("USERNAME"))
	}
	return strings.Join(strings.Fields(u.Username), "")
}
