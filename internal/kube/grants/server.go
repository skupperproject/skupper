package grants

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/skupperproject/skupper/internal/utils/tlscfg"
)

type Server struct {
	tlsEnabled bool
	lock       sync.RWMutex
	cert       *tls.Certificate
	server     *http.Server
	listener   net.Listener
	logger     *slog.Logger
}

func newServer(addr string, tlsEnabled bool, handler http.Handler) *Server {
	return &Server{
		server: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  60 * time.Second,
			WriteTimeout: 60 * time.Second,
			TLSConfig:    tlscfg.Modern(),
		},
		tlsEnabled: tlsEnabled,
		logger:     slog.New(slog.Default().Handler()).With(slog.String("component", "kube.grants.server")),
	}
}

func (s *Server) start() {
	go s.listenAndServe()
}

func (s *Server) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.cert, nil
}

func (s *Server) setCertificate(cert *tls.Certificate) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.cert = cert
}

func (s *Server) setCertificateFromSecret(secret *corev1.Secret) error {
	cert, err := tls.X509KeyPair(secret.Data["tls.crt"], secret.Data["tls.key"])
	if err != nil {
		return err
	}
	s.setCertificate(&cert)
	return nil
}

func (s *Server) listen() error {
	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return err
	}
	s.logger.Info("Grant server listening", slog.Any("address", listener.Addr()))
	s.listener = listener
	return nil
}

func (s *Server) serve() error {
	if s.listener == nil {
		return fmt.Errorf("Cannot serve before listen() is called")
	}
	if s.tlsEnabled {
		s.server.TLSConfig.GetCertificate = s.getCertificate
		return s.server.ServeTLS(s.listener, "", "")
	} else {
		return s.server.Serve(s.listener)
	}
}

func (s *Server) listenAndServe() error {
	if err := s.listen(); err != nil {
		s.logger.Error("Grant server failed to listen", slog.String("address", s.server.Addr), slog.Any("error", err))
		return err
	}
	defer s.listener.Close()
	return s.serve()
}

func (s *Server) stop() error {
	err := s.server.Close()
	s.listener.Close()
	s.listener = nil
	return err
}

func (s *Server) port() int {
	if s.listener == nil {
		return 0
	}
	return s.listener.Addr().(*net.TCPAddr).Port
}
