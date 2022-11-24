//go:build podman
// +build podman

package podman

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/kube"
	corev1 "k8s.io/api/core/v1"
)

// some podman tests require a cluster as well
var clusterRun = flag.Bool("use-cluster", false, "run tests against a configured cluster")

var (
	cli     *podman.PodmanRestClient
	cliKube *client.VanClient
)

const (
	ENV_PODMAN_ENDPOINT = "SKUPPER_TEST_PODMAN_ENDPOINT"
	NS                  = "podman-system-test"
	NGINX_IMAGE         = "nginxinc/nginx-unprivileged:stable-alpine"
)

// newBasicSite returns a new instance of a basic SitePodman instance
// as a Site instance that has already been removed should not be reused
func newBasicSite() domain.Site {
	return &SitePodman{
		SiteCommon: &domain.SiteCommon{
			Name: "site-podman-test",
		},
		IngressHosts:   []string{"127.0.0.1"},
		PodmanEndpoint: getEndpoint(),
	}
}

func createBasicSite() error {
	siteHandler, err := NewSitePodmanHandler(getEndpoint())
	if err != nil {
		return err
	}
	return siteHandler.Create(newBasicSite())
}

func teardownBasicSite() error {
	siteHandler, err := NewSitePodmanHandler(getEndpoint())
	if err != nil {
		return err
	}
	return siteHandler.Delete()
}

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

	if *clusterRun {
		// Creating a cluster site
		cliKube, err = client.NewClient(NS, "", "")
		if err != nil {
			log.Fatalf("error creating skupper client for kubernetes - %v", err)
		}
		_, err = kube.NewNamespace(NS, cliKube.KubeClient)
		if err != nil {
			log.Fatalf("error creating namespace %s - %v", NS, err)
		}

		err = configureSiteAndCreateRouter(context.Background(), cliKube, "skupper-k8s")
		if err != nil {
			teardown()
		}
	}

	rc := m.Run()

	// Teardown
	teardown()

	os.Exit(rc)
}

func teardown() {
	if *clusterRun {
		err := kube.DeleteNamespace(NS, cliKube.KubeClient)
		if err != nil {
			log.Fatalf("error deleting namespace %s - %v", NS, err)
		}
	}
}

func configureSiteAndCreateRouter(ctx context.Context, cli *client.VanClient, name string) error {
	routerCreateOpts := types.SiteConfigSpec{
		SkupperName:      "skupper",
		RouterMode:       string(types.TransportModeInterior),
		EnableController: true,
	}
	siteConfig, err := cli.SiteConfigCreate(ctx, routerCreateOpts)
	if err != nil {
		return fmt.Errorf("unable to configure %s site - %v", name, err)
	}
	err = cli.RouterCreate(ctx, *siteConfig)
	if err != nil {
		return fmt.Errorf("unable to create %s VAN router - %v", name, err)
	}

	tick := time.Second * 5
	timeout := time.Minute * 2

	// wait for skupper component to be running
	for _, component := range []string{types.TransportComponentName, types.ControllerComponentName} {
		selector := "skupper.io/component=" + component
		if err := kube.WaitForPodsStatus(cli.Namespace, cli.KubeClient, selector, corev1.PodRunning, timeout, tick); err != nil {
			return err
		}
	}

	return nil
}

func runNginxContainer() error {
	if err := cli.ImagePull(NGINX_IMAGE); err != nil {
		return err
	}
	c := container.Container{
		Name:     "nginx-container",
		Image:    NGINX_IMAGE,
		Networks: map[string]container.ContainerNetworkInfo{},
	}
	c.Networks[container.ContainerNetworkName] = container.ContainerNetworkInfo{ID: container.ContainerNetworkName}
	if err := cli.ContainerCreate(&c); err != nil {
		return err
	}
	if err := cli.ContainerStart(c.Name); err != nil {
		return err
	}
	return nil
}
