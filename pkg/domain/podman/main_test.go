//go:build system || podman
// +build system podman

package podman

import (
	"flag"
	"log"
	"os"
	"testing"

	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain"
)

// some podman tests require a cluster as well
var clusterRun = flag.Bool("use-cluster", false, "run tests against a configured cluster")

var (
	cli       *podman.PodmanRestClient
	siteBasic = &SitePodman{
		SiteCommon: &domain.SiteCommon{
			Name: "site-podman-test",
		},
		IngressHosts:   []string{"127.0.0.1"},
		PodmanEndpoint: getEndpoint(),
	}
)

const (
	ENV_PODMAN_ENDPOINT = "SKUPPER_TEST_PODMAN_ENDPOINT"
)

func getEndpoint() string {
	return os.Getenv(ENV_PODMAN_ENDPOINT)
}

func TestMain(m *testing.M) {
	var err error
	cli, err = podman.NewPodmanClient(getEndpoint(), "")
	if err != nil {
		log.Fatalf("podman client could not be created - %v", err)
	}
	flag.Parse()
	os.Exit(m.Run())
}
