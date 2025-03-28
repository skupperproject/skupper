package controller

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"sync"

	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

const (
	socketFileName = "controller.sock"
)

type Controller struct {
	nsHandler *NamespacesHandler
}

func NewController() (*Controller, error) {
	var err error
	c := &Controller{}
	c.nsHandler, err = NewNamespacesHandler()
	return c, err
}

func (c *Controller) Start() (chan struct{}, *sync.WaitGroup) {
	log.Println("Starting controller")
	wg := &sync.WaitGroup{}
	stop := make(chan struct{})
	wg.Add(1)
	c.ensureSingleInstance(stop)
	if err := c.nsHandler.Start(stop, wg); err != nil {
		log.Fatalf("error starting controller: %v", err)
	}
	return stop, wg
}

// ensureSingleInstance listens to a unix socket to prevent concurrent executions across processes or containers
func (c *Controller) ensureSingleInstance(stop chan struct{}) {
	internalSocketFile := path.Join(api.GetDefaultOutputNamespacesPath(), socketFileName)
	if _, err := net.Dial("unix", internalSocketFile); err == nil {
		log.Fatalf("User controller is already running, exiting")
	}
	if err := os.Remove(internalSocketFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		hostSocketFile := ""
		if api.IsRunningInContainer() {
			hostSocketFile = fmt.Sprintf(" (host path: %q)", path.Join(api.GetHostNamespacesPath(), socketFileName))
		}
		log.Fatalf("unable to remove socket %s%s: %s", internalSocketFile, hostSocketFile, err)
	}
	socket, err := net.Listen("unix", internalSocketFile)
	if err != nil {
		log.Fatalf("error listening on socket %q: %v", internalSocketFile, err.Error())
	}
	go func() {
		// unix socket remains open to prevent concurrent controllers from running
		defer socket.Close()
		<-stop
	}()
}
