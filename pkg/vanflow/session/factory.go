package session

import (
	"crypto/rand"
	"encoding/hex"
	"io"
)

// ContainerFactory creates Containers with a given configuration
type ContainerFactory interface {
	Create() Container
}

func NewContainerFactory(address string, cfg ContainerConfig) ContainerFactory {
	return factory{
		Address: address,
		Config:  cfg,
	}
}

func NewMockContainerFactory() ContainerFactory {
	return mockFactory{Router: NewMockRouter()}
}

type mockFactory struct {
	Router *MockRouter
}

func (m mockFactory) Create() Container {
	return NewMockContainer(m.Router)
}

type factory struct {
	Address string
	Config  ContainerConfig
}

func (f factory) Create() Container {
	return NewContainer(f.Address, f.Config)
}

func randomID() string {
	var bytes [16]byte
	io.ReadFull(rand.Reader, bytes[:])
	return hex.EncodeToString(bytes[:])
}
