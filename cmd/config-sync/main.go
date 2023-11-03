package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	log.Printf("CONFIG_SYNC: Version: %s", version.Version)

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := SetupSignalHandler()

	namespace, err := kube.CurrentNamespace()
	if err != nil {
		log.Fatal("Error determining namespace: ", err.Error())
	}

	cli, err := client.NewClient(namespace, "", "")
	if err != nil {
		log.Fatal("Error getting van client: ", err.Error())
	}

	log.Println("CONFIG_SYNC: Waiting for Skupper router to be ready")
	_, err = kube.WaitForPodsSelectorStatus(namespace, cli.KubeClient, "skupper.io/component=router", corev1.PodRunning, time.Second*180, time.Second*5)
	if err != nil {
		log.Fatal("Error waiting for router pods to be ready ", err.Error())
	}

	event.StartDefaultEventStore(stopCh)
	if claims.StartClaimVerifier(cli.KubeClient, cli.Namespace, cli, cli) {
		log.Println("CONFIG_SYNC: Claim verifier started")
	} else {
		log.Println("CONFIG_SYNC: Claim verifier not enabled")
	}

	log.Printf("CONFIG_SYNC: Creating ConfigMap informer...")
	informer := corev1informer.NewFilteredConfigMapInformer(
		cli.KubeClient,
		cli.Namespace,
		time.Second*30,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=skupper-internal"
		}))
	go informer.Run(stopCh)
	log.Printf("CONFIG_SYNC: Waiting for informer to sync...")
	if ok := cache.WaitForCacheSync(stopCh, informer.HasSynced); !ok {
		log.Fatal("Failed to wait for caches to sync")
	}

	configSync := newConfigSync(informer, cli)
	log.Println("CONFIG_SYNC: Starting sync controller loop...")
	configSync.start(stopCh)

	go startCollector(cli)

	<-stopCh
	log.Println("CONFIG_SYNC: Shutting down...")
	configSync.stop()
}
