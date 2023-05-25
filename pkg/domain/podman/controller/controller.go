package controller

import (
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"github.com/skupperproject/skupper/api/types"
	clientpodman "github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/pkg/flow"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
)

// ControllerPodman defines the podman site implementation of the controller.
type ControllerPodman struct {
	cli               *clientpodman.PodmanRestClient
	cfg               *podman.Config
	tlsConfig         *tls.Config
	origin            string
	containerInformer *clientpodman.ContainerInformer
	servicesWatcher   *ServiceTargetWatcher
	site              *podman.Site
}

func NewControllerPodman(origin string, tlsConfig *tls.Config) (*ControllerPodman, error) {
	cfg, err := podman.NewPodmanConfigFileHandler().GetConfig()
	if err != nil {
		return nil, fmt.Errorf("error reading podman site config - %s", err)
	}
	podmanCli, err := clientpodman.NewPodmanClient(cfg.Endpoint, "")
	if err != nil {
		return nil, fmt.Errorf("error creating podman client - %s", err)
	}
	c := &ControllerPodman{
		cli:       podmanCli,
		cfg:       cfg,
		origin:    origin,
		tlsConfig: tlsConfig,
	}
	return c, nil
}

func (c *ControllerPodman) Run(stopCh <-chan struct{}) error {
	var err error

	log.Println("Starting the Skupper controller")

	err = utils.Retry(time.Second, 120, func() (bool, error) {
		router, err := c.cli.ContainerInspect(types.TransportDeploymentName)
		if err != nil {
			return false, fmt.Errorf("error retrieving %s container state - %w", types.TransportDeploymentName, err)
		}
		if !router.Running {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		log.Fatalf("unable to determine if %s container is running - %s", types.TransportDeploymentName, err)
	}

	siteHandler, err := podman.NewSitePodmanHandler(c.cfg.Endpoint)
	if err != nil {
		log.Fatalf("unable to communicate with podman - %s", err)
	}

	site, err := siteHandler.Get()
	if err != nil {
		log.Fatalf("unable to retrieve site information - %s", err)
	}
	sitePodman := site.(*podman.Site)
	c.site = sitePodman

	siteCreationTime := uint64(time.Now().UnixNano()) / uint64(time.Microsecond)
	flowController := flow.NewFlowController(c.origin, siteCreationTime, qdr.NewConnectionFactory("amqps://"+types.LocalTransportServiceName+":5671", c.tlsConfig))
	flowController.Start(stopCh)
	log.Println("Started flow-controller")

	//
	// Set the beacons
	//

	// HostRecord for podman host is provided only at startup
	SendPodmanHostRecord(c.cli, c.site, c.origin, flowController, siteCreationTime)

	// ProcessRecord container informer
	c.containerInformer = clientpodman.NewContainerInformer(c.cli)
	c.containerInformer.AddInformer(NewContainerProcessInformer(c.cli, c.origin, c.site, flowController))
	c.containerInformer.Start(stopCh)

	// ProcessRecord watcher for service targets (using IP addresses)
	if err = NewServiceTargetWatcher(sitePodman, flowController).Watch(stopCh); err != nil {
		log.Printf("unable to watch service targets - %s", err)
	}

	log.Println("Started container informer")
	<-stopCh
	log.Println("Shutting down controllers")

	return nil
}
