package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/flow"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/tools/cache"

	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/kube/claims"
)

var onlyOneSignalHandler = make(chan struct{})
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

func SetupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler) // panics when called twice

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}

func main() {
	// if -version used, report and exit
	isVersion := flag.Bool("version", false, "Report the version of Config Sync")
	flag.Parse()
	if *isVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	// Startup message
	log.Printf("Config Sync")
	log.Printf("Version: %s", version.Version)

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := SetupSignalHandler()

	namespace, err := kube.CurrentNamespace()
	if err != nil {
		log.Fatal("Error determining namespace: ", err.Error())
	}

	log.Printf("Creating kube client...")
	cli, err := client.NewClient(namespace, "", "")
	if err != nil {
		log.Fatal("Error getting van client: ", err.Error())
	}

	log.Println("Waiting for Skupper router to be ready")
	pods, err := kube.GetPods("skupper.io/component=router", namespace, cli.KubeClient)
	for _, pod := range pods {
		_, err := kube.WaitForPodStatus(namespace, cli.KubeClient, pod.Name, corev1.PodRunning, time.Second*180, time.Second*5)
		if err != nil {
			log.Fatal("Error waiting for router pods to be ready ", err.Error())
		}
	}

	event.StartDefaultEventStore(stopCh)
	if claims.StartClaimVerifier(cli.KubeClient, cli.Namespace, cli, cli) {
		log.Println("Claim verifier started")
	} else {
		log.Println("Claim verifier not enabled")
	}

	log.Printf("Creating informer...")
	informer := corev1informer.NewFilteredConfigMapInformer(
		cli.KubeClient,
		cli.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=skupper-internal"
		}))
	go informer.Run(stopCh)
	log.Printf("Waiting for informer to sync...")
	if ok := cache.WaitForCacheSync(stopCh, informer.HasSynced); !ok {
		log.Fatal("Failed to wait for caches to sync")
	}

	configSync := newConfigSync(informer, cli)
	log.Println("Starting sync loop...")
	configSync.start(stopCh)

	siteData := map[string]string{}
	platform := config.GetPlatform()
	if platform == "" || platform == types.PlatformKubernetes {
		current, err := kube.GetDeployment(types.TransportDeploymentName, cli.Namespace, cli.KubeClient)
		if err != nil {
			log.Fatal("Failed to get transport deployment", err.Error())
		}
		owner := kube.GetDeploymentOwnerReference(current)
		_, err = kube.NewConfigMap(types.SiteStatusConfigMapName, &siteData, nil, nil, &owner, cli.Namespace, cli.KubeClient)
		if err != nil {
			log.Fatal("Failed to create site status config map ", err.Error())
		}
	}
	fc := flow.NewFlowCollector(flow.FlowCollectorSpec{
		Mode:              flow.Lite,
		Namespace:         cli.Namespace,
		Origin:            os.Getenv("SKUPPER_SITE_ID"),
		PromReg:           nil,
		ConnectionFactory: qdr.NewConnectionFactory("amqp://localhost:5672", nil),
		FlowRecordTtl:     time.Minute * 15})
	log.Println("Starting site status collector...")
	fc.Start(stopCh)

	<-stopCh
	log.Println("Shutting down...")
	configSync.stop()
}
