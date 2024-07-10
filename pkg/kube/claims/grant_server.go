package claims

import (
	"crypto/tls"
	"log"
	"net/http"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils/tlscfg"
)

type GrantServer struct {
	grants   *Grants
	addr     string
	keyPath  string
	certPath string
	lock     sync.RWMutex
	cert     *tls.Certificate
}

func (s *GrantServer) GrantChanged(key string, grant *skupperv1alpha1.AccessGrant) error {
	return s.grants.checkGrant(key, grant)
}

func (s *GrantServer) start() {
	go s.listen()
}

func (s *GrantServer) configure(clients internalclient.Clients, config *GrantConfig, generator GrantResponse) {
	s.addr = config.Addr
	s.keyPath = config.KeyPath
	s.certPath = config.CertPath
	s.grants = newGrants(clients, generator, config.scheme(), config.BaseUrl)
}

func (s *GrantServer) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.cert, nil
}

func (s *GrantServer) setCertificate(cert *tls.Certificate) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.cert = cert
}

func (s *GrantServer) tlsCredentialsUpdated(key string, secret *corev1.Secret) error {
	if secret == nil {
		return nil
	}
	cert, err := tls.X509KeyPair(secret.Data["tls.crt"], secret.Data["tls.key"])
	if err != nil {
		log.Printf("Could not get X509 key pair for grant server from %s: %s", key, err)
		return nil
	}
	s.setCertificate(&cert)
	if s.grants.setCA(string(secret.Data["ca.crt"])) {
		s.grants.recheckCa()
	}
	log.Print("Grant server tls credentials updated")
	return nil
}

func (s *GrantServer) listen() {
	server := &http.Server{
		Addr:         s.addr,
		Handler:      s.grants,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		TLSConfig:    tlscfg.Modern(),
	}

	if s.certPath != "" && s.keyPath != "" {
		server.TLSConfig.GetCertificate = s.getCertificate
		err := server.ListenAndServeTLS("", "")
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
