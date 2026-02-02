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
	logger    *slog.Logger
}

func NewController() (*Controller, error) {
	var err error
	c := &Controller{
		logger: slog.New(slog.Default().Handler()).With(slog.String("component", "nonkube.controller")),
	}
	c.nsHandler, err = NewNamespacesHandler()
	return c, err
}

func (c *Controller) Start() (chan struct{}, *sync.WaitGroup) {
	c.logger.Info("Starting controller")
	wg := &sync.WaitGroup{}
	stop := make(chan struct{})
	c.ensureSingleInstance(stop)
	if err := c.nsHandler.Start(stop, wg); err != nil {
		c.logger.Error("error starting controller", slog.Any("error", err))
		os.Exit(1)
	}
	c.logger.Info("Controller started")
	return stop, wg
}

func (c *Controller) ensureSingleInstance(stop chan struct{}) {
	internalLockFile := path.Join(api.GetDefaultOutputNamespacesPath(), lockFileName)
	lock, err := os.OpenFile(internalLockFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		c.logger.Error("Unable to create lock file", slog.Any("error", err))
		os.Exit(1)
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		c.logger.Error("System controller is already running, exiting")
		os.Exit(1)
	}
	go func() {
		<-stop
		if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
			c.logger.Error("Error releasing lock file", slog.Any("error", err))
			os.Exit(1)
		}
	}()
}
