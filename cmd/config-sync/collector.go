package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/flow"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
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

func siteCollector(stopCh <-chan struct{}, cli *client.VanClient) *flow.FlowCollector {
	var fc *flow.FlowCollector
	siteData := map[string]string{}
	platform := config.GetPlatform()
	if platform == "" || platform == types.PlatformKubernetes {
		current, err := kube.GetDeployment(types.TransportDeploymentName, cli.Namespace, cli.KubeClient)
		if err != nil {
			log.Fatal("Failed to get transport deployment", err.Error())
		}
		owner := kube.GetDeploymentOwnerReference(current)

		existing, err := kube.NewConfigMap(types.VanStatusConfigMapName, &siteData, nil, nil, &owner, cli.Namespace, cli.KubeClient)
		if err != nil && existing == nil {
			log.Fatal("Failed to create site status config map ", err.Error())
		}

		err = updateLockOwner(types.SiteLeaderLockName, cli.Namespace, &owner, cli)
		if err != nil {
			log.Println("Update lock error", err.Error())
		}

		fc = flow.NewFlowCollector(flow.FlowCollectorSpec{
			Mode:              flow.RecordStatus,
			Namespace:         cli.Namespace,
			Origin:            os.Getenv("SKUPPER_SITE_ID"),
			PromReg:           nil,
			ConnectionFactory: qdr.NewConnectionFactory("amqp://localhost:5672", nil),
			FlowRecordTtl:     time.Minute * 15})

		fc.Start(stopCh)
	}
	return fc
}

func runLeaderElection(lock *resourcelock.ConfigMapLock, ctx context.Context, id string, cli *client.VanClient) {
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
				log.Printf("COLLECTOR: Leader %s starting site collection \n", podname)
				stopCh = make(chan struct{})
				siteCollector(stopCh, cli)
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
