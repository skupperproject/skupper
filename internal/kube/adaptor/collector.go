package adaptor

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/config"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	kubeflow "github.com/skupperproject/skupper/pkg/kube/flow"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
	"github.com/skupperproject/skupper/pkg/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

func updateLockOwner(lockname, namespace string, owner *metav1.OwnerReference, cli *internalclient.KubeClient) error {
	current, err := cli.Kube.CoordinationV1().Leases(namespace).Get(context.TODO(), lockname, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if owner != nil {
		current.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			*owner,
		}
	}
	_, err = cli.Kube.CoordinationV1().Leases(namespace).Update(context.TODO(), current, metav1.UpdateOptions{})
	return err
}

func siteCollector(ctx context.Context, cli *internalclient.KubeClient) {
	siteData := map[string]string{}
	platform := config.GetPlatform()
	if platform != types.PlatformKubernetes {
		return
	}
	current, err := cli.Kube.AppsV1().Deployments(cli.Namespace).Get(context.TODO(), deploymentName(), metav1.GetOptions{})
	if err != nil {
		log.Fatal("Failed to get transport deployment", err.Error())
	}

	owner := metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       current.ObjectMeta.Name,
		UID:        current.ObjectMeta.UID,
	}

	existing, err := newConfigMap(types.NetworkStatusConfigMapName, &siteData, nil, nil, &owner, cli.Namespace, cli.Kube)
	if err != nil && existing == nil {
		log.Fatal("Failed to create site status config map ", err.Error())
	}

	err = updateLockOwner(types.SiteLeaderLockName, cli.Namespace, &owner, cli)
	if err != nil {
		log.Println("Update lock error", err.Error())
	}

	factory := session.NewContainerFactory("amqp://localhost:5672", session.ContainerConfig{ContainerID: "kube-flow-collector"})
	statusSync := kubeflow.NewStatusSync(factory, nil, cli.Kube.CoreV1().ConfigMaps(cli.Namespace), types.NetworkStatusConfigMapName)
	go statusSync.Run(ctx)

}

func startFlowController(ctx context.Context, cli *internalclient.KubeClient) error {
	deployment, err := cli.Kube.AppsV1().Deployments(cli.Namespace).Get(context.TODO(), deploymentName(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get transport deployment: %s", err)
	}
	if len(deployment.OwnerReferences) < 1 {
		return fmt.Errorf("transport deployment had no owner required to infer site name and ID")
	}
	siteID := string(deployment.OwnerReferences[0].UID)
	siteName := deployment.OwnerReferences[0].Name

	informer := corev1informer.NewPodInformer(cli.Kube, cli.Namespace, time.Minute*5, cache.Indexers{})
	platform := "kubernetes"
	fc := kubeflow.NewController(kubeflow.ControllerConfig{
		Factory:  session.NewContainerFactory("amqp://localhost:5672", session.ContainerConfig{ContainerID: "kube-flow-controller"}),
		Informer: informer,
		Site: vanflow.SiteRecord{
			BaseRecord: vanflow.NewBase(siteID, deployment.ObjectMeta.CreationTimestamp.Time),
			Name:       &siteName,
			Namespace:  &cli.Namespace,
			Platform:   &platform,
			Version:    &version.Version,
			Provider:   &platform, //todo(ck) Not really correct. involved with nodes access (below)
		},
	})
	go informer.Run(ctx.Done())
	//TODO: should watching nodes be optional or should we attempt to determine if we have permissions first?
	//kubeflow.WatchNodes(controller, cli.Namespace, flowController.UpdateHost)
	go func() {
		fc.Run(ctx)
		if ctx.Err() == nil {
			slog.Error("kube flow controller unexpectedly quit")
		}
	}()
	return nil
}

func runLeaderElection(lock *resourcelock.LeaseLock, id string, cli *internalclient.KubeClient) {
	ctx := context.Background()
	begin := time.Now()
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
				siteCollector(ctx, cli)
				if err := startFlowController(ctx, cli); err != nil {
					log.Printf("COLLECTOR: Failed to start controller for emitting site events: %s", err)
				}
			},
			OnStoppedLeading: func() {
				// we held the lock but lost it. This indicates that something
				// went wrong. Exit and restart.
				log.Fatalf("COLLECTOR: Lost leader lock after %s", time.Since(begin))
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

func StartCollector(cli *internalclient.KubeClient) {
	lockname := types.SiteLeaderLockName
	namespace := cli.Namespace
	podname, _ := os.Hostname()

	leaseLock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      lockname,
			Namespace: namespace,
		},
		Client: cli.Kube.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: podname,
		},
	}

	runLeaderElection(leaseLock, podname, cli)
}

func deploymentName() string {
	deployment := os.Getenv("SKUPPER_ROUTER_DEPLOYMENT")
	if deployment == "" {
		return types.TransportDeploymentName
	}
	return deployment

}

func newConfigMap(name string, data *map[string]string, labels *map[string]string, annotations *map[string]string, owner *metav1.OwnerReference, namespace string, kubeclient kubernetes.Interface) (*corev1.ConfigMap, error) {
	configMaps := kubeclient.CoreV1().ConfigMaps(namespace)
	existing, err := configMaps.Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		//TODO:  already exists
		return existing, nil
	} else if errors.IsNotFound(err) {
		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}

		if data != nil {
			cm.Data = *data
		}
		if labels != nil {
			cm.ObjectMeta.Labels = *labels
		}
		if annotations != nil {
			cm.ObjectMeta.Annotations = *annotations
		}
		if owner != nil {
			cm.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
				*owner,
			}
		}

		created, err := configMaps.Create(context.TODO(), cm, metav1.CreateOptions{})

		if err != nil {
			return nil, fmt.Errorf("Failed to create config map: %w", err)
		} else {
			return created, nil
		}
	} else {
		cm := &corev1.ConfigMap{}
		return cm, fmt.Errorf("Failed to check existing config maps: %w", err)
	}
}
