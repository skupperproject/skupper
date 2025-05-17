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

	mu             sync.Mutex
	configured     map[string]qdr.SslProfile
	profileSecrets map[string]syncContext

	cleanup func()
}

func NewSync(factory SecretsCacheFactory, callback Callback, logger *slog.Logger) *Sync {
	stopCh := make(chan struct{})
	sync := &Sync{
		cleanup:        sync.OnceFunc(func() { close(stopCh) }),
		logger:         logger,
		callback:       callback,
		configured:     make(map[string]qdr.SslProfile),
		profileSecrets: make(map[string]syncContext),
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

func (s *Sync) handleProfile(key string, secret *corev1.Secret, pctx profileContext) (bool, error) {
	prev, hadPrev := s.getProfile(pctx.ProfileName)
	configuredProfile, isConfigured := s.getConfigured(pctx.ProfileName)
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

func (s *Sync) handle(key string, secret *corev1.Secret) error {
	if secret == nil {
		return nil
	}
	metadata, found, err := fromSecret(secret)
	if err != nil {
		return fmt.Errorf("failed to decode secret metadata: %s", err)
	}
	if !found {
		return nil
	}
	for _, profileMetadata := range metadata {
		profileName := profileMetadata.ProfileName
		updated, err := s.handleProfile(key, secret, profileMetadata)
		if err != nil {
			return fmt.Errorf("error handling secret %q for profile %s: %s", key, profileName, err)
		}
		if updated {
			s.doCallback(profileName)
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
func (s *Sync) getConfigured(profileName string) (qdr.SslProfile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.configured[profileName]
	return result, ok
}
func (s *Sync) setConfigured(profiles map[string]qdr.SslProfile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configured = profiles
}

func (s *Sync) Expect(profiles map[string]qdr.SslProfile) SyncDelta {
	var delta SyncDelta
	s.setConfigured(profiles)
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
				_, err := s.handleProfile(context.SecretKey, secret, context.profileContext)
				if err != nil {
					delta.Errors = append(delta.Errors, err)
				}
			}
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

func writeFile(path string, data []byte, perm os.FileMode) error {
	if path == "" {
		return nil
	}
	return os.WriteFile(path, data, perm)
}
