package controller

import (
	"log"
	"sync"
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
	if err := c.nsHandler.Start(stop); err != nil {
		log.Fatalf("error starting controller: %v", err)
	}
	return stop, wg
}
