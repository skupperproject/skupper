package secrets

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/skupperproject/skupper/internal/qdr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

type syncContext struct {
	profileContext

	SecretKey        string
	SecretContentSum [32]byte
}

type Callback func(profileName string)
type Sync struct {
	logger   *slog.Logger
	cache    SecretsCache
	callback Callback

	mu              sync.Mutex
	configuredSsl   map[string]qdr.SslProfile
	configuredProxy map[string]qdr.ProxyProfile
	profileSecrets  map[string]syncContext

	cleanup func()
}

func NewSync(factory SecretsCacheFactory, callback Callback, logger *slog.Logger) *Sync {
	stopCh := make(chan struct{})
	sync := &Sync{
		cleanup:         sync.OnceFunc(func() { close(stopCh) }),
		logger:          logger,
		callback:        callback,
		configuredSsl:   make(map[string]qdr.SslProfile),
		configuredProxy: make(map[string]qdr.ProxyProfile),
		profileSecrets:  make(map[string]syncContext),
	}
	sync.cache = factory(stopCh, sync.handle)
	return sync
}

func (s *Sync) Stop() {
	s.cleanup()
}

func (m *Sync) doCallback(name string) {
	if m.callback == nil {
		return
	}
	m.callback(name)
}

func (s *Sync) Recover() {
	for _, secret := range s.cache.List() {
		err := s.handle(secret.Namespace+"/"+secret.Name, secret)
		if err != nil {
			s.logger.Error("Recovery error", slog.String("secret_name", secret.Name), slog.Any("error", err))
		}
	}
}

func (s *Sync) handleSslProfile(key string, secret *corev1.Secret, pctx profileContext) (bool, error) {
	prev, hadPrev := s.getProfile(pctx.ProfileName)
	configuredProfile, isConfigured := s.getConfiguredSsl(pctx.ProfileName)
	updated := syncContext{
		profileContext: pctx,
		SecretKey:      key,
	}
	if !isConfigured {
		s.setProfileSecret(updated)
		return false, nil
	}
	sumChanged := updateSecretChecksum(secret, &prev.SecretContentSum)
	updated.SecretContentSum = prev.SecretContentSum
	if hadPrev {
		if pctx.Ordinal < prev.Ordinal {
			s.logger.Info("Ignoring Secret update downgrading ordinal",
				slog.String("secret_name", key),
				slog.Uint64("want_ordinal", prev.Ordinal),
				slog.Uint64("have_ordinal", pctx.Ordinal),
			)
			return false, nil
		}
	}
	hasWrite := false
	if !hadPrev || sumChanged {
		if err := writeSslProfile(secret, configuredProfile); err != nil {
			return false, fmt.Errorf("write for sslProfile failed: %s", err)
		}
		s.logger.Info("Wrote SslProfile contents",
			slog.String("profile_name", pctx.ProfileName),
			slog.String("secret_name", key),
			slog.Uint64("prev_ordinal", prev.Ordinal),
			slog.Uint64("has_ordinal", pctx.Ordinal),
			slog.Uint64("wants_ordinal", configuredProfile.Ordinal),
		)
		hasWrite = true
	}
	s.setProfileSecret(updated)
	ordinalAdvanced := prev.Ordinal < pctx.Ordinal
	if !hasWrite && ordinalAdvanced {
		s.logger.Info("SslProfile Secret Ordinal Advanced",
			slog.String("profile_name", pctx.ProfileName),
			slog.String("secret_name", key),
			slog.Uint64("prev_ordinal", prev.Ordinal),
			slog.Uint64("has_ordinal", pctx.Ordinal),
			slog.Uint64("wants_ordinal", configuredProfile.Ordinal),
		)
	}
	viableUpdate := configuredProfile.Ordinal <= pctx.Ordinal && (hasWrite || ordinalAdvanced)
	return viableUpdate, nil
}

func (s *Sync) handleProxyProfile(key string, secret *corev1.Secret) (bool, error) {
	profileName := secret.Name
	prev, hadPrev := s.getProfile(profileName)
	proxyProfile, isConfigured := s.getConfiguredProxy(profileName)
	updated := syncContext{
		profileContext: profileContext{
			ProfileName: profileName,
		},
		SecretKey: key,
	}
	if !isConfigured {
		s.setProfileSecret(updated)
		return false, nil
	}
	sumChanged := updateSecretChecksum(secret, &prev.SecretContentSum)
	updated.SecretContentSum = prev.SecretContentSum
	hasWrite := false
	if !hadPrev || sumChanged {
		if len(secret.Data["username"]) > 0 && len(secret.Data["password"]) > 0 {
			path := strings.TrimPrefix(proxyProfile.Password, "file:")
			if err := writeProxyProfile(secret, path); err != nil {
				return false, fmt.Errorf("write for proxyProfile failed: %s", err)
			}
			hasWrite = true
		}
	}
	s.setProfileSecret(updated)
	return hasWrite, nil
}

func (s *Sync) handle(key string, secret *corev1.Secret) error {
	if secret == nil {
		return nil
	}

	switch secret.Type {
	case "kubernetes.io/tls":
		metadata, found, err := fromSecret(secret)
		if err != nil {
			return fmt.Errorf("failed to decode secret metadata: %s", err)
		}
		if !found {
			return nil
		}
		for _, profileMetadata := range metadata {
			profileName := profileMetadata.ProfileName
			updated, err := s.handleSslProfile(key, secret, profileMetadata)
			if err != nil {
				return fmt.Errorf("error handling secret %q for profile %s: %s", key, profileName, err)
			}
			if updated {
				s.doCallback(profileName)
			}
		}
	case "kubernetes.io/basic-auth":
		updated, err := s.handleProxyProfile(key, secret)
		if err != nil {
			return fmt.Errorf("error handling secret %q for proxy profile: %s", key, err)
		}
		if updated {
			s.doCallback(secret.Name)
		}
	}

	return nil
}

func (s *Sync) getProfile(profileName string) (syncContext, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.profileSecrets[profileName]
	return result, ok
}
func (s *Sync) setProfileSecret(pctx syncContext) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profileSecrets[pctx.ProfileName] = pctx
}
func (s *Sync) getConfiguredSsl(profileName string) (qdr.SslProfile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.configuredSsl[profileName]
	return result, ok
}
func (s *Sync) setConfiguredSsl(profiles map[string]qdr.SslProfile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configuredSsl = profiles
}

func (s *Sync) ExpectSslProfiles(profiles map[string]qdr.SslProfile) SyncDelta {
	var delta SyncDelta
	s.setConfiguredSsl(profiles)
	for profileName, qdrProfile := range profiles {
		context, ok := s.getProfile(profileName)
		if !ok {
			delta.Missing = append(delta.Missing, profileName)
			continue
		}
		if context.Ordinal < qdrProfile.Ordinal {
			if delta.PendingOrdinals == nil {
				delta.PendingOrdinals = make(map[string]OrdinalDelta)
			}
			delta.PendingOrdinals[profileName] = OrdinalDelta{
				SecretName: context.SecretKey,
				Expect:     qdrProfile.Ordinal,
				Current:    context.Ordinal,
			}
		} else {
			secret, _ := s.cache.Get(context.SecretKey)
			if secret != nil {
				_, err := s.handleSslProfile(context.SecretKey, secret, context.profileContext)
				if err != nil {
					delta.Errors = append(delta.Errors, err)
				}
			}
		}
	}
	return delta
}

func (s *Sync) getConfiguredProxy(profileName string) (qdr.ProxyProfile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.configuredProxy[profileName]
	return result, ok
}
func (s *Sync) setConfiguredProxy(profiles map[string]qdr.ProxyProfile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configuredProxy = profiles
}

func (s *Sync) ExpectProxyProfiles(key string, profiles map[string]qdr.ProxyProfile) SyncDelta {
	var delta SyncDelta
	delta.ProxyUpdates = make(map[string]qdr.ProxyProfile)
	s.setConfiguredProxy(profiles)
	namespace, _, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		delta.Errors = append(delta.Errors, err)
	}
	for profileName, profile := range profiles {
		secret, _ := s.cache.Get(namespace + "/" + profileName)
		if secret == nil {
			delta.Missing = append(delta.Missing, profileName)
			continue
		} else {
			_, err := s.handleProxyProfile(key, secret)
			if err != nil {
				delta.Errors = append(delta.Errors, err)
			}
			profile.Host = string(secret.Data["host"])
			profile.Port = string(secret.Data["port"])
			profile.Username = string(secret.Data["username"])
			delta.ProxyUpdates[profileName] = profile
		}

	}
	return delta
}

type OrdinalDelta struct {
	SecretName string
	Expect     uint64
	Current    uint64
}
type SyncDelta struct {
	Missing         []string
	PendingOrdinals map[string]OrdinalDelta
	ProxyUpdates    map[string]qdr.ProxyProfile
	Errors          []error
}

func (d SyncDelta) Error() error {
	if d.Empty() {
		return nil
	}
	var parts []string
	for _, err := range d.Errors {
		parts = append(parts, err.Error())
	}
	for _, missing := range d.Missing {
		parts = append(parts, fmt.Sprintf("missing secret for profile %q", missing))
	}
	for pname, diff := range d.PendingOrdinals {
		parts = append(parts, fmt.Sprintf(
			"profile %q configured with ordinal %d, but secret %q has %d",
			pname,
			diff.Expect,
			diff.SecretName,
			diff.Current,
		))
	}
	return fmt.Errorf("secrets not synchronized with router config: %s", strings.Join(parts, ", "))
}

func (d SyncDelta) Empty() bool {
	return len(d.Missing) == 0 && len(d.PendingOrdinals) == 0 && len(d.Errors) == 0
}

func writeSslProfile(secret *corev1.Secret, profile qdr.SslProfile) error {
	if profile.CaCertFile == "" {
		return fmt.Errorf("empty sslProfile %q", secret.Name)
	}
	baseName := path.Dir(profile.CaCertFile)
	if err := os.MkdirAll(baseName, 0755); err != nil {
		return fmt.Errorf("error making sslProfile certificates directory %q: %e", baseName, err)
	}

	if err := writeFile(profile.CaCertFile, secret.Data["ca.crt"], 0644); err != nil {
		return fmt.Errorf("error writing ca.crt: %e", err)
	}
	if err := writeFile(profile.CertFile, secret.Data["tls.crt"], 0644); err != nil {
		return fmt.Errorf("error writing tls.crt: %e", err)
	}
	if err := writeFile(profile.PrivateKeyFile, secret.Data["tls.key"], 0600); err != nil {
		return fmt.Errorf("error writing tls.key: %e", err)
	}
	return nil
}

func writeProxyProfile(secret *corev1.Secret, filePath string) error {
	_, ok := secret.Data["password"]
	if !ok {
		return fmt.Errorf("empty proxyProfile %q", secret.Name)
	}
	baseName := path.Dir(filePath)
	if err := os.MkdirAll(baseName, 0755); err != nil {
		return fmt.Errorf("error making proxyProfile password directory %q: %e", baseName, err)
	}
	if err := writeFile(filePath, []byte(secret.Data["password"]), 0644); err != nil {
		return fmt.Errorf("error writing password.txt: %e", err)
	}
	return nil
}

func writeFile(path string, data []byte, perm os.FileMode) error {
	if path == "" {
		return nil
	}
	return os.WriteFile(path, data, perm)
}
