package controller

import (
	"log"
	"os"
	"path"
	"sync"
	"syscall"

	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

const (
	lockFileName = "controller.lock"
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
	log.Println("Controller started")
	return stop, wg
}

func (c *Controller) ensureSingleInstance(stop chan struct{}) {
	internalLockFile := path.Join(api.GetDefaultOutputNamespacesPath(), lockFileName)
	lock, err := os.OpenFile(internalLockFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatalf("Unable to create lock file: %v", err)
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		log.Fatalf("User controller is already running, exiting")
	}
	go func() {
		<-stop
		if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
			log.Fatalf("Error releasing lock file: %v", err)
		}
	}()
}
