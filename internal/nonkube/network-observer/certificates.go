package networkobserver

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/skupperproject/skupper/internal/certs"
	corev1 "k8s.io/api/core/v1"
)

func GenerateNginxCert(caDir, certDir string) error {
	caSecret, err := loadSecretFromDir(caDir)
	if err != nil {
		return fmt.Errorf("failed to load skupper-local-ca: %w", err)
	}

	secret, err := certs.GenerateSecret("skupper-network-observer", "skupper-network-observer", []string{"localhost"}, 0, caSecret)
	if err != nil {
		return fmt.Errorf("failed to generate nginx certificate: %w", err)
	}

	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("failed to create certificate directory: %w", err)
	}

	if err := os.WriteFile(filepath.Join(certDir, "tls.crt"), secret.Data["tls.crt"], 0644); err != nil {
		return fmt.Errorf("failed to write tls.crt: %w", err)
	}
	if err := os.WriteFile(filepath.Join(certDir, "tls.key"), secret.Data["tls.key"], 0600); err != nil {
		return fmt.Errorf("failed to write tls.key: %w", err)
	}

	return nil
}

func loadSecretFromDir(dir string) (*corev1.Secret, error) {
	crt, err := os.ReadFile(filepath.Join(dir, "tls.crt"))
	if err != nil {
		return nil, err
	}
	key, err := os.ReadFile(filepath.Join(dir, "tls.key"))
	if err != nil {
		return nil, err
	}
	return &corev1.Secret{
		Data: map[string][]byte{
			"tls.crt": crt,
			"tls.key": key,
		},
	}, nil
}
