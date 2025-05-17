package controller

type ActivationCallback interface {
	Start(stopCh <-chan struct{})
	Stop()
	Id() string
}
