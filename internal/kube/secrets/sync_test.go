package secrets_test

import (
	"io"
	"log/slog"
	"os"
	"path"
	"testing"

	"github.com/skupperproject/skupper/internal/kube/secrets"
	"github.com/skupperproject/skupper/internal/qdr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const tlsProfKey = `internal.skupper.io/tls-profile-context`

var (
	expectedFiles       = []string{"ca.crt", "tls.crt", "tls.key"}
	expectedCAOnlyFiles = []string{"ca.crt"}
)

func TestSyncExpect(t *testing.T) {
	tmpdir := t.TempDir()
	tlog := slog.New(slog.NewTextHandler(io.Discard, nil))
	sCache := secretsCacheFactoryFixture(t, "testing")
	sCache.Secrets["testing/test-tls"] = fixtureTlsSecret("test-tls", "testing")
	sCache.Secrets["testing/test-tls"].Annotations[tlsProfKey] = `[{"profileName": "test-tls"}, {"profileName": "test-tls-profile", "ordinal": 8}]`

	secretSync := secrets.NewSync(sCache.Factory, nil, tlog)
	secretSync.Recover()
	delta := secretSync.Expect(map[string]qdr.SslProfile{
		"test-tls":         fixtureSslProfile("test-tls", tmpdir, 0, 0, false),
		"test-tls-profile": fixtureSslProfile("test-tls-profile", tmpdir, 8, 0, true),
	})
	if !delta.Empty() {
		t.Errorf("expected all profiles to be resolved by secret: %s", delta.Error())
	}
	assertFiles(t, path.Join(tmpdir, "test-tls"), expectedFiles)
	assertFiles(t, path.Join(tmpdir, "test-tls-profile"), expectedCAOnlyFiles)
}

func TestSyncHandler(t *testing.T) {
	tmpdir := t.TempDir()
	tlog := slog.New(slog.NewTextHandler(io.Discard, nil))
	sCache := secretsCacheFactoryFixture(t, "testing")
	secretSync := secrets.NewSync(sCache.Factory, nil, tlog)
	secretSync.Recover()

	configuredProfiles := map[string]qdr.SslProfile{
		"test-tls": fixtureSslProfile("test-tls", tmpdir, 12, 11, false),
	}
	delta := secretSync.Expect(configuredProfiles)
	if len(delta.Missing) != 1 {
		t.Errorf("expected missing profile test-tls: %s", delta.Error())
	}
	if delta.Error() == nil {
		t.Errorf("expected error: %v", delta)
	}

	sCache.Secrets["testing/test-tls"] = fixtureTlsSecret("test-tls", "testing")
	sCache.Secrets["testing/test-tls"].Annotations[tlsProfKey] = `[{"profileName": "test-tls", "ordinal": 1}]`
	if err := sCache.HandlerFn("testing/test-tls", sCache.Secrets["testing/test-tls"]); err != nil {
		t.Errorf("unexpected error handling new secret: %s", err)
	}

	delta = secretSync.Expect(configuredProfiles)
	diff := delta.PendingOrdinals["test-tls"]
	expectDiff := secrets.OrdinalDelta{Expect: 12, Current: 1, SecretName: "testing/test-tls"}
	if diff != expectDiff {
		t.Errorf("expected pending ordinal %v got %v", expectDiff, diff)
	}
	if delta.Error() == nil {
		t.Errorf("expected error: %v", delta)
	}

	sCache.Secrets["testing/test-tls"].Annotations[tlsProfKey] = `[{"profileName": "test-tls", "ordinal": 12}]`
	if err := sCache.HandlerFn("testing/test-tls", sCache.Secrets["testing/test-tls"]); err != nil {
		t.Errorf("unexpected error handling updated secret: %s", err)
	}
	delta = secretSync.Expect(configuredProfiles)
	if !delta.Empty() {
		t.Errorf("expected all profiles to be resolved: %s", delta.Error())
	}
	assertFiles(t, path.Join(tmpdir, "test-tls"), expectedFiles)
}

func TestSyncCallback(t *testing.T) {
	tmpdir := t.TempDir()
	tlog := slog.New(slog.NewTextHandler(io.Discard, nil))
	sCache := secretsCacheFactoryFixture(t, "testing")
	var callbackValues []string
	callback := func(profileName string) {
		callbackValues = append(callbackValues, profileName)
	}
	secretSync := secrets.NewSync(sCache.Factory, callback, tlog)
	secretSync.Recover()

	configuredProfiles := map[string]qdr.SslProfile{
		"test-tls": fixtureSslProfile("test-tls", tmpdir, 12, 11, false),
	}
	delta := secretSync.Expect(configuredProfiles)
	if delta.Empty() {
		t.Errorf("expected missing secret")
	}
	if len(callbackValues) > 0 {
		t.Errorf("unexpected callbacks: %s", callbackValues)
	}
	assertFiles(t, tmpdir, []string{})

	sCache.Secrets["testing/test-tls"] = fixtureTlsSecret("test-tls", "testing")
	sCache.Secrets["testing/test-tls"].Annotations[tlsProfKey] = `[{"profileName": "test-tls", "ordinal": 12}]`
	if err := sCache.HandlerFn("testing/test-tls", sCache.Secrets["testing/test-tls"]); err != nil {
		t.Errorf("unexpected error handling new secret: %s", err)
	}
	assertFiles(t, path.Join(tmpdir, "test-tls"), expectedFiles)

	if len(callbackValues) != 1 || callbackValues[0] != "test-tls" {
		t.Errorf("expected one callback for test-tls profile: %s", callbackValues)
	}
	delta = secretSync.Expect(configuredProfiles)
	if !delta.Empty() {
		t.Errorf("expected all profiles to be resolved: %s", delta.Error())
	}

}

func assertFiles(t *testing.T, dir string, fileNames []string) {
	t.Helper()
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("unexpected error reading directory: %s", err)
	}
	expected := map[string]struct{}{}
	for _, n := range fileNames {
		expected[n] = struct{}{}
	}
	for _, file := range files {
		fileName := file.Name()
		if _, ok := expected[fileName]; ok {
			delete(expected, fileName)
		} else {
			t.Errorf("Unexpected file %q found", fileName)
		}
	}
	for fileName := range expected {
		t.Errorf("Expected file %q not found", fileName)
	}
}

func fixtureTlsSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
		Data: map[string][]byte{
			"ca.crt":  []byte("ca.crt - " + name),
			"tls.crt": []byte("tls.crt - " + name),
			"tls.key": []byte("tls.key - " + name),
		},
	}
}

func fixtureSslProfile(name, baseDir string, ord, oldestOrd uint64, caonly bool) qdr.SslProfile {
	profile := qdr.SslProfile{
		Name:               name,
		CaCertFile:         path.Join(baseDir, name, "ca.crt"),
		Ordinal:            ord,
		OldestValidOrdinal: oldestOrd,
	}
	if !caonly {
		profile.CertFile = path.Join(baseDir, name, "tls.crt")
		profile.PrivateKeyFile = path.Join(baseDir, name, "tls.key")
	}
	return profile
}

func secretsCacheFactoryFixture(t *testing.T, ns string) *stubSecretsCache {
	t.Helper()
	return &stubSecretsCache{
		Secrets:   make(map[string]*corev1.Secret),
		Namespace: ns,
	}
}

type stubSecretsCache struct {
	Secrets   map[string]*corev1.Secret
	Namespace string
	HandlerFn func(string, *corev1.Secret) error
}

func (s *stubSecretsCache) Factory(stopCh <-chan struct{}, handler func(string, *corev1.Secret) error) secrets.SecretsCache {
	s.HandlerFn = handler
	return s
}

func (s stubSecretsCache) Get(key string) (*corev1.Secret, error) {
	return s.Secrets[key], nil
}
func (s stubSecretsCache) List() []*corev1.Secret {
	out := make([]*corev1.Secret, 0, len(s.Secrets))
	for _, secret := range s.Secrets {
		out = append(out, secret)
	}
	return out
}
