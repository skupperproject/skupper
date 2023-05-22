package podman

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/skupperproject/skupper/pkg/utils"

	"github.com/skupperproject/skupper/api/types"
)

const (
	SharedTlsCertificates = "skupper-router-certs"
)

var (
	Username                = readUsername()
	SkupperContainerVolumes = []string{"skupper-services", "skupper-local-server", "skupper-internal", "skupper-site-server", SharedTlsCertificates,
		types.ConsoleServerSecret, types.ConsoleUsersSecret, types.ClaimsServerSecret}
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
