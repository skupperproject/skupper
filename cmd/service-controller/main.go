package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/kube"
)

func describe(i interface{}) {
	fmt.Printf("(%v, %T)\n", i, i)
	fmt.Println()
}

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

func getTlsConfig(verify bool, cert, key, ca string) (*tls.Config, error) {
	var config tls.Config
	config.InsecureSkipVerify = true
	if verify {
		certPool := x509.NewCertPool()
		file, err := ioutil.ReadFile(ca)
		if err != nil {
			return nil, err
		}
		certPool.AppendCertsFromPEM(file)
		config.RootCAs = certPool
		config.InsecureSkipVerify = false
	}

	_, errCert := os.Stat(cert)
	_, errKey := os.Stat(key)
	if errCert == nil || errKey == nil {
		tlsCert, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			log.Fatal("Could not load x509 key pair", err.Error())
		}
		config.Certificates = []tls.Certificate{tlsCert}
	}
	config.MinVersion = tls.VersionTLS10

	return &config, nil
}

func main() {
	origin := os.Getenv("SKUPPER_SERVICE_SYNC_ORIGIN")
	namespace := os.Getenv("SKUPPER_NAMESPACE")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := SetupSignalHandler()

	// todo, get context from env?
	cli, err := client.NewClient(namespace, "", "")
	if err != nil {
		log.Fatal("Error getting van client", err.Error())
	}

	tlsConfig, err := getTlsConfig(true, types.ControllerConfigPath+"tls.crt", types.ControllerConfigPath+"tls.key", types.ControllerConfigPath+"ca.crt")
	if err != nil {
		log.Fatal("Error getting tls config", err.Error())
	}

	controller, err := NewController(cli, origin, tlsConfig)
	if err != nil {
		log.Fatal("Error getting new controller", err.Error())
	}

	log.Println("Waiting for Skupper transport to start")
	pods, err := kube.GetDeploymentPods(types.TransportDeploymentName, namespace, cli.KubeClient)
	if err != nil {
		log.Fatal("Error getting transport deployment pods", err.Error())
	}
	for _, pod := range pods {
		_, err := kube.WaitForPodStatus(namespace, cli.KubeClient, pod.Name, corev1.PodRunning, time.Second*180, time.Second*5)
		if err != nil {
			log.Fatal("Error waiting for skupper transport pod running status", err.Error())
		}
	}

	// start the controller workers
	if err = controller.Run(stopCh); err != nil {
		log.Fatal("Error running controller: ", err.Error())
	}
}
