package site_podman

import (
	"fmt"
	"os"

	"github.com/skupperproject/skupper/api/types"
)

const (
	SharedTlsCertificates = "skupper-router-certs"
)

var (
	Username                = os.Getenv("USER")
	SkupperContainerVolumes = []string{"skupper-local-server", "skupper-internal", "skupper-site-server", SharedTlsCertificates}
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
