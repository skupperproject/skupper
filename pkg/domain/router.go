package domain

import (
	"github.com/skupperproject/skupper/pkg/qdr"
)

type RouterEntityManager interface {
	CreateSslProfile(sslProfile qdr.SslProfile) error
	DeleteSslProfile(name string) error
	CreateConnector(connector qdr.Connector) error
	DeleteConnector(name string) error
}
