package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"

	"github.com/c-kruse/vanflow/session"
)

func parseMessagingConfig(file string) (session.ContainerFactory, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("could not read messaging-config %s", err)
	}
	var cfg connectJSON
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("could not parse messaging-config %s\n", err)
	}
	connURL := fmt.Sprintf("%s://%s:%s", cfg.Scheme, cfg.Host, cfg.Port)
	var tlsCfg *tls.Config
	if cfg.Tls.CA != "" {
		tlsCfg, err = cfg.Tls.Parse()
		if err != nil {
			return nil, fmt.Errorf("could not parse tls config in messaging-config %s\n", err)
		}
	}
	var sasl session.SASLType
	if tlsCfg != nil {
		sasl = session.SASLTypeExternal
	}

	return session.NewContainerFactory(connURL, session.ContainerConfig{
		TLSConfig: tlsCfg, SASLType: sasl,
	}), nil
}

type tlsConfig struct {
	CA     string `json:"ca,omitempty"`
	Cert   string `json:"cert,omitempty"`
	Key    string `json:"key,omitempty"`
	Verify bool   `json:"verify,omitempty"`
}

type connectJSON struct {
	Scheme string    `json:"scheme,omitempty"`
	Host   string    `json:"host,omitempty"`
	Port   string    `json:"port,omitempty"`
	Tls    tlsConfig `json:"tls,omitempty"`
}

func (c tlsConfig) Parse() (*tls.Config, error) {
	var config tls.Config
	config.InsecureSkipVerify = true
	if c.Verify {
		certPool := x509.NewCertPool()
		file, err := os.ReadFile(c.CA)
		if err != nil {
			return nil, err
		}
		certPool.AppendCertsFromPEM(file)
		config.RootCAs = certPool
		config.InsecureSkipVerify = false
	}

	_, errCert := os.Stat(c.Cert)
	_, errKey := os.Stat(c.Key)
	if errCert == nil || errKey == nil {
		tlsCert, err := tls.LoadX509KeyPair(c.Cert, c.Key)
		if err != nil {
			return nil, fmt.Errorf("could not load x509 key pair: %v", err)
		}
		config.Certificates = []tls.Certificate{tlsCert}
	}
	config.MinVersion = tls.VersionTLS10
	return &config, nil
}
