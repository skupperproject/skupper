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
    
	//"github.com/ajssmith/skupper/api/types"
	"github.com/ajssmith/skupper/client"
)

//TODO: Move to types
const (
	ServiceSyncAddress  = "mc/$skupper-service-sync"
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
    cli, err := client.NewClient(namespace, "")
    if err != nil {
        log.Fatal("Error getting van client", err.Error())
    }

	tlsConfig, err := getTlsConfig(true, "/etc/messaging/tls.crt", "/etc/messaging/tls.key", "/etc/messaging/ca.crt")
    if err != nil {
        log.Fatal("Error getting tls config", err.Error())
    }

    controller,err := NewController(cli, origin, tlsConfig)
    if err != nil {
        log.Fatal("Error getting new controller", err.Error())
    }

    // fire up the informers
    go controller.cmInformer.Run(stopCh)
    go controller.depInformer.Run(stopCh)
    go controller.svcInformer.Run(stopCh)

    // start the controller workers
    if err = controller.Run(stopCh); err != nil {
        log.Fatal("Error running controller: %s", err.Error())
    }
}
