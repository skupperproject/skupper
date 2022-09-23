package site_podman

import "os"

var (
	Username                = os.Getenv("USER")
	SkupperContainerVolumes = []string{"skupper-local-server", "skupper-internal", "skupper-site-server", "skupper-router-certs"}
)
