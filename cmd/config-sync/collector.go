package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/flow"
	"github.com/skupperproject/skupper/pkg/kube"
	kubeflow "github.com/skupperproject/skupper/pkg/kube/flow"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

func updateLockOwner(lockname, namespace string, owner *metav1.OwnerReference, cli *client.VanClient) error {
	current, err := cli.KubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), lockname, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if owner != nil {
		current.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			*owner,
		}
	}
	_, err = cli.KubeClient.CoreV1().ConfigMaps(namespace).Update(context.TODO(), current, metav1.UpdateOptions{})
	return err
}

func siteCollector(stopCh <-chan struct{}, cli *client.VanClient) {
	var fc *flow.FlowCollector
	siteData := map[string]string{}
	platform := config.GetPlatform()
	if platform != types.PlatformKubernetes {
		return
	}
	current, err := kube.GetDeployment(types.TransportDeploymentName, cli.Namespace, cli.KubeClient)
	if err != nil {
		log.Fatal("Failed to get transport deployment", err.Error())
	}
	owner := kube.GetDeploymentOwnerReference(current)

	existing, err := kube.NewConfigMap(types.NetworkStatusConfigMapName, &siteData, nil, nil, &owner, cli.Namespace, cli.KubeClient)
	if err != nil && existing == nil {
		log.Fatal("Failed to create site status config map ", err.Error())
	}

	err = updateLockOwner(types.SiteLeaderLockName, cli.Namespace, &owner, cli)
	if err != nil {
		log.Println("Update lock error", err.Error())
	}

	fc = flow.NewFlowCollector(flow.FlowCollectorSpec{
		Mode:                flow.RecordStatus,
		Namespace:           cli.Namespace,
		PromReg:             nil,
		ConnectionFactory:   qdr.NewConnectionFactory("amqp://localhost:5672", nil),
		FlowRecordTtl:       time.Minute * 15,
		NetworkStatusClient: cli.KubeClient,
	})

	go primeBeacons(fc, cli)
	log.Println("COLLECTOR: Starting flow collector")
	fc.Start(stopCh)
}

// primeBeacons attempts to guess the router and service-controller vanflow IDs
// to pass to the flow collector in order to accelerate startup time.
func primeBeacons(fc *flow.FlowCollector, cli *client.VanClient) {
	podname, _ := os.Hostname()
	var prospectRouterID string
	if len(podname) >= 5 {
		prospectRouterID = fmt.Sprintf("%s:0", podname[len(podname)-5:])
	}
	var siteID string
	cm, err := kube.WaitConfigMapCreated(types.SiteConfigMapName, cli.Namespace, cli.KubeClient, 5*time.Second, 250*time.Millisecond)
	if err != nil {
		log.Printf("COLLECTOR: failed to get skupper-site ConfigMap. Proceeding without Site ID. %s\n", err)
	} else if cm != nil {
		siteID = string(cm.ObjectMeta.UID)
	}
	log.Printf("COLLECTOR: Priming site with expected beacons for '%s' and '%s'\n", prospectRouterID, siteID)
	fc.PrimeSiteBeacons(siteID, prospectRouterID)
}

func startFlowController(stopCh <-chan struct{}, cli *client.VanClient) error {
	siteId := os.Getenv("SKUPPER_SITE_ID")

	deployment, err := kube.GetDeployment(types.TransportDeploymentName, cli.Namespace, cli.KubeClient)
	if err != nil {
		log.Fatal("Failed to get transport deployment", err.Error())
	}
	creationTime := uint64(deployment.ObjectMeta.CreationTimestamp.UnixNano()) / uint64(time.Microsecond)
	flowController := flow.NewFlowController(siteId, version.Version, creationTime, qdr.NewConnectionFactory("amqp://localhost:5672", nil), nil/*TODO: enable policy checks?*/)

	controller := kube.NewController("flow-controller", cli)
	kubeflow.WatchPods(controller, cli.Namespace, flowController.UpdateProcess)
	//TODO: should watching nodes be optional or should we attempt to determine if we have permissions first?
	//kubeflow.WatchNodes(controller, cli.Namespace, flowController.UpdateHost)
	controller.Start(stopCh)
	flowController.Start(stopCh)
	return nil
}

func runLeaderElection(lock *resourcelock.ConfigMapLock, ctx context.Context, id string, cli *client.VanClient) {
	begin := time.Now()
	var stopCh chan struct{}
	podname, _ := os.Hostname()
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(c context.Context) {
				log.Printf("COLLECTOR: Leader %s starting site collection after %s\n", podname, time.Since(begin))
				stopCh = make(chan struct{})
				siteCollector(stopCh, cli)
				if err := startFlowController(stopCh, cli); err != nil {
					log.Printf("COLLECTOR: Failed to start controller for emitting site events: %s", err)
				}
			},
			OnStoppedLeading: func() {
				// No longer the leader, transition to inactive
				close(stopCh)
			},
			OnNewLeader: func(current_id string) {
				if current_id == id {
					// Remain as the leader
					return
				}
				log.Printf("COLLECTOR: New leader for site collection is %s\n", current_id)
			},
		},
	})
}

func startCollector(cli *client.VanClient) {
	lockname := types.SiteLeaderLockName
	namespace := cli.Namespace
	podname, _ := os.Hostname()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmLock := &resourcelock.ConfigMapLock{
		ConfigMapMeta: metav1.ObjectMeta{
			Name:      lockname,
			Namespace: namespace,
		},
		Client: cli.KubeClient.CoreV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: podname,
		},
	}

	runLeaderElection(cmLock, ctx, podname, cli)
}
