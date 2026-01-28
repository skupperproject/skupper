package controller

import (
	"log/slog"
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
	slog.Info("Starting controller")
	wg := &sync.WaitGroup{}
	stop := make(chan struct{})
	c.ensureSingleInstance(stop)
	if err := c.nsHandler.Start(stop, wg); err != nil {
		slog.Error("error starting controller", slog.Any("error", err))
		os.Exit(1)
	}
	slog.Info("Controller started")
	return stop, wg
}

func (c *Controller) ensureSingleInstance(stop chan struct{}) {
	internalLockFile := path.Join(api.GetDefaultOutputNamespacesPath(), lockFileName)
	lock, err := os.OpenFile(internalLockFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		slog.Error("Unable to create lock file", slog.Any("error", err))
		os.Exit(1)
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		slog.Error("System controller is already running, exiting")
		os.Exit(1)
	}
	go func() {
		<-stop
		if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
			slog.Error("Error releasing lock file", slog.Any("error", err))
			os.Exit(1)
		}
	}()
}
