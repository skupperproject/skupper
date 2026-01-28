package adaptor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/internal/flow"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	kubeflow "github.com/skupperproject/skupper/internal/kube/flow"
	"github.com/skupperproject/skupper/internal/version"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type StatusSyncClient struct {
	client typedcorev1.ConfigMapInterface
}

func (s *StatusSyncClient) Logger() *slog.Logger {
	logger := slog.New(slog.Default().Handler()).With(
		slog.String("component", "kube.flow.statusSync"),
	)
	return logger
}

func (s *StatusSyncClient) Get(ctx context.Context) (*corev1.ConfigMap, error) {
	return s.client.Get(ctx, types.NetworkStatusConfigMapName, metav1.GetOptions{})
}

func (s *StatusSyncClient) Update(ctx context.Context, latest *corev1.ConfigMap) error {
	_, err := s.client.Update(ctx, latest, metav1.UpdateOptions{})
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
		slog.Error("Failed to get transport deployment", slog.Any("error", err))
		os.Exit(1)
	}

	owner := metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       current.ObjectMeta.Name,
		UID:        current.ObjectMeta.UID,
	}

	existing, err := newConfigMap(types.NetworkStatusConfigMapName, &siteData, nil, nil, &owner, cli.Namespace, cli.Kube)
	if err != nil && existing == nil {
		slog.Error("Failed to create site status config map", slog.Any("error", err))
		os.Exit(1)
	}

	factory := session.NewContainerFactory("amqp://localhost:5672", session.ContainerConfig{ContainerID: "kube-flow-collector"})
	statusSyncClient := &StatusSyncClient{
		client: cli.Kube.CoreV1().ConfigMaps(cli.Namespace),
	}
	statusSync := flow.NewStatusSync(factory, nil, statusSyncClient, types.NetworkStatusConfigMapName)
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
	var (
		mu              sync.Mutex
		leaderCtx       context.Context
		leaderCtxCancel func()
	)
	// attempt to run leader election forever
	strategy := backoff.NewExponentialBackOff(backoff.WithMaxElapsedTime(0))
	backoff.RetryNotify(func() error {
		leaderelection.RunOrDie(context.Background(), leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true,
			LeaseDuration:   15 * time.Second,
			RenewDeadline:   10 * time.Second,
			RetryPeriod:     2 * time.Second,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(ctx context.Context) {
					mu.Lock()
					defer mu.Unlock()
					leaderCtx, leaderCtxCancel = context.WithCancel(ctx)
					slog.Info("COLLECTOR: Became leader. Starting status sync and site controller", slog.Any("elapsedTime", strategy.GetElapsedTime()))
					siteCollector(leaderCtx, cli)
					if err := startFlowController(leaderCtx, cli); err != nil {
						slog.Error("COLLECTOR: Failed to start controller for emitting site events", slog.Any("error", err))
					}
				},
				OnStoppedLeading: func() {
					slog.Info("COLLECTOR: Lost leader lock. Stopping status sync and site controller", slog.Any("elapsedTime", strategy.GetElapsedTime()))
					mu.Lock()
					defer mu.Unlock()
					if leaderCtxCancel == nil {
						return
					}
					leaderCtxCancel()
					leaderCtx, leaderCtxCancel = nil, nil
				},
				OnNewLeader: func(current_id string) {
					if current_id == id {
						// Remain as the leader
						return
					}
					slog.Info("COLLECTOR: New leader for site collection", slog.String("newLeader", current_id))
				},
			},
		})
		return fmt.Errorf("leader election died")
	},
		strategy,
		func(_ error, d time.Duration) {
			slog.Info("COLLECTOR: leader election failed. retrying after delay", slog.Any("delay", d))
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
