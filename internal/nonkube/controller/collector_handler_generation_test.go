package controller

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

const generationTestNamespace = "collector-generations"

// collectorDataHome is created once for the package. Collector goroutines can
// still be reading api.DefaultRootDataHome after a test returns, so it must
// not be rewritten per test.
var (
	collectorDataHomeOnce sync.Once
	collectorDataHome     string
)

// setupCollectorFS points the nonkube filesystem helpers at a temp directory
// and creates the minimum inputs flow.StartCollector needs (platform file,
// RouterAccess and client certificates). It returns the path of the shared
// network status ConfigMap file written by every collector generation. No
// router is listening, so the status sync itself never gets a session.
func setupCollectorFS(t *testing.T) string {
	t.Helper()
	collectorDataHomeOnce.Do(func() {
		dir, err := os.MkdirTemp("", "collector-generations")
		if err != nil {
			t.Fatal(err)
		}
		collectorDataHome = dir
		// root uses DefaultRootDataHome, other users use XDG_DATA_HOME
		api.DefaultRootDataHome = dir
		os.Setenv("XDG_DATA_HOME", dir)
	})
	nsDir := filepath.Join(api.GetDataHome(), "namespaces", generationTestNamespace)
	if err := os.RemoveAll(nsDir); err != nil {
		t.Fatal(err)
	}
	write := func(p, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(nsDir, "internal", "platform.yaml"), "platform: podman\n")
	write(filepath.Join(nsDir, string(api.RuntimeSiteStatePath), "RouterAccess-skupper-local.yaml"),
		"apiVersion: skupper.io/v2alpha1\nkind: RouterAccess\nmetadata:\n  name: skupper-local\nspec:\n  roles:\n  - name: normal\n    port: 1\n")

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "collector-generations"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	certDir := filepath.Join(nsDir, string(api.CertificatesPath), "skupper-local-client")
	write(filepath.Join(certDir, "ca.crt"), certPEM)
	write(filepath.Join(certDir, "tls.crt"), certPEM)
	write(filepath.Join(certDir, "tls.key"), keyPEM)

	return filepath.Join(nsDir, string(api.RuntimeSiteStatePath), "ConfigMap-skupper-network-status.yaml")
}

// fileExists reports whether p exists.
func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// waitFor polls cond until it returns true or timeout elapses, and reports
// whether cond became true.
func waitFor(cond func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return cond()
}

// lastGeneration returns the done channel of the most recently started
// generation. It is closed once that generation has returned from its start
// method, which for the real collector means its status ConfigMap is gone.
func lastGeneration(h *CollectorLifecycleHandler) <-chan struct{} {
	h.mux.Lock()
	defer h.mux.Unlock()
	return h.lastDone
}

// TestCollectorHandlerRestartKeepsStatus restarts the real collector back to
// back, both before and after the first generation has written its status
// ConfigMap. It waits for the first generation to finish, which removes its
// ConfigMap, and then requires the second generation to create its own and
// keep it until it is stopped. A stale generation's cleanup must never remove
// the current generation's ConfigMap.
func TestCollectorHandlerRestartKeepsStatus(t *testing.T) {
	cmPath := setupCollectorFS(t)
	const (
		rounds  = 20
		timeout = 5 * time.Second
	)
	for i := 0; i < rounds; i++ {
		h := NewCollectorLifecycleHandler(generationTestNamespace)
		stopCh := make(chan struct{})

		h.Start(stopCh)
		gen1Done := lastGeneration(h)
		if i%2 == 0 {
			// let generation 1 get as far as writing the ConfigMap
			if !waitFor(func() bool { return fileExists(cmPath) }, timeout) {
				t.Fatalf("round %d: gen1 never created %s", i, cmPath)
			}
		}
		h.Stop()
		h.Start(stopCh) // generation 2, immediately after Stop
		gen2Done := lastGeneration(h)

		// Generation 1 removes its ConfigMap before it finishes, so after
		// this wait the only file that can exist belongs to generation 2.
		select {
		case <-gen1Done:
		case <-time.After(timeout):
			t.Fatalf("round %d: gen1 did not finish after Stop", i)
		}
		if !waitFor(func() bool { return fileExists(cmPath) }, timeout) {
			t.Fatalf("round %d: gen2 never created %s after gen1 finished", i, cmPath)
		}

		h.Stop()
		select {
		case <-gen2Done:
		case <-time.After(timeout):
			t.Fatalf("round %d: gen2 did not finish after Stop", i)
		}
		if fileExists(cmPath) {
			t.Fatalf("round %d: ConfigMap left behind after gen2 finished", i)
		}
		close(stopCh)
	}
}
