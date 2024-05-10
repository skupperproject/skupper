package claims

import (
	"log"
	"net/http"
	"time"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils/tlscfg"
)

type GrantServer struct {
	grants *Grants
	addr   string
	key    string
	cert   string
}

func (s *GrantServer) GrantChanged(key string, grant *skupperv1alpha1.Grant) error {
	return s.grants.checkGrant(key, grant)
}

func (s *GrantServer) start() {
	go s.listen()
}

func (s *GrantServer) configure(clients kube.Clients, config *GrantConfig, generator GrantResponse) {
	s.addr = config.Addr
	s.key = config.KeyPath
	s.cert = config.CertPath
	s.grants = newGrants(clients, generator, config.BaseUrl)
	s.grants.ca = config.CaCert
}

func (s *GrantServer) listen() {
	server := &http.Server{
		Addr:         s.addr,
		Handler:      s.grants,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		TLSConfig:    tlscfg.Modern(),
	}

	if s.cert != "" && s.key != "" {
		err := server.ListenAndServeTLS(s.cert, s.key)
		if err != nil {
			log.Printf("Grant server failed to start on %s: %s", s.addr, err)
		} else {
			log.Printf("Grant server listening on %s (TLS enabled)", s.addr)
		}
	} else {
		err := server.ListenAndServe()
		if err != nil {
			log.Printf("Grant server failed to start on %s: %s", s.addr, err)
		} else {
			log.Printf("Grant server listening on %s (TLS not enabled)", s.addr)
		}
	}
}
