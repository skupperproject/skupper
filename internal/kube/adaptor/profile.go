package adaptor

import (
	"os"
	paths "path"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type SslProfileSyncer struct {
	profiles map[string]*SslProfile
	path     string
}

func newSslProfileSyncer(path string) *SslProfileSyncer {
	return &SslProfileSyncer{
		profiles: map[string]*SslProfile{},
		path:     path,
	}
}

func (s SslProfileSyncer) get(profile string) (*SslProfile, bool) {
	secret := profile
	if strings.HasSuffix(profile, "-profile") {
		secret = strings.TrimSuffix(profile, "-profile")
	}
	if current, ok := s.profiles[secret]; ok {
		return current, false
	}
	target := &SslProfile{
		name: secret,
		path: paths.Join(s.path, profile),
	}
	s.profiles[secret] = target
	return target, true
}

func (s SslProfileSyncer) bySecretName(secret string) (*SslProfile, bool) {
	current, ok := s.profiles[secret]
	return current, ok
}

type SslProfile struct {
	name   string
	path   string
	secret *corev1.Secret
}

func (s *SslProfile) sync(secret *corev1.Secret) error {
	if s.secret != nil && reflect.DeepEqual(s.secret.Data, secret.Data) {
		return nil
	}
	if err := writeSecretToPath(secret, s.path); err != nil {
		return err
	}
	s.secret = secret
	return nil
}

func writeSecretToPath(secret *corev1.Secret, path string) error {
	if err := mkdir(path); err != nil {
		return err
	}
	for key, value := range secret.Data {
		if err := os.WriteFile(paths.Join(path, key), value, 0777); err != nil {
			return err
		}
	}
	return nil
}

func mkdir(path string) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		err = os.Mkdir(path, 0777)
		if err != nil {
			return err
		}
	}
	return nil
}
