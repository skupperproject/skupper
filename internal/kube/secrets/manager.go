package secrets

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"github.com/skupperproject/skupper/internal/qdr"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type SecretsCache interface {
	Get(key string) (*corev1.Secret, error)
	List() []*corev1.Secret
}

type SecretsCacheFactory func(stopCh <-chan struct{}, handler func(string, *corev1.Secret) error) SecretsCache
type UpdateRouterConfigFn func(update qdr.ConfigUpdate) error
type PriorValidityProvider interface {
	TLSPriorValidRevisions() uint64
}

type profileWatcherContext struct {
	Ordinal            uint64
	OldestValidOrdinal uint64

	SecretKey        string
	SecretContentSum [32]byte
}

type ProfilesWatcher struct {
	logger     *slog.Logger
	cache      SecretsCache
	client     typedv1.SecretInterface
	update     UpdateRouterConfigFn
	pvProvider PriorValidityProvider
	namespace  string

	state map[string]*profileWatcherContext

	cleanup func()
}

func NewProfilesWatcher(factory SecretsCacheFactory, client kubernetes.Interface, update UpdateRouterConfigFn, pvProvider PriorValidityProvider, namespace string, logger *slog.Logger) *ProfilesWatcher {
	stopCh := make(chan struct{})
	w := &ProfilesWatcher{
		namespace:  namespace,
		logger:     logger,
		client:     client.CoreV1().Secrets(namespace),
		update:     update,
		pvProvider: pvProvider,
		state:      make(map[string]*profileWatcherContext),
		cleanup:    sync.OnceFunc(func() { close(stopCh) }),
	}
	w.cache = factory(stopCh, w.handleSecret)
	return w
}

func (w *ProfilesWatcher) Stop() {
	w.cleanup()
}

func (w *ProfilesWatcher) handleSecret(key string, secret *corev1.Secret) error {
	if secret == nil {
		return nil
	}
	secretName := secret.ObjectMeta.Name
	changed := false
	var secretsContext profileContextSet
	for _, profileName := range secretProfiles(secretName) {
		state, ok := w.state[profileName]
		if !ok {
			continue
		}
		if state.SecretKey == "" {
			state.SecretKey = key
			updateSecretChecksum(secret, &state.SecretContentSum)
		} else if state.SecretKey != key {
			continue
		}
		if updateSecretChecksum(secret, &state.SecretContentSum) {
			state.Ordinal += 1
			changed = true
		}
		pv := w.checkPriorValidity(secret)
		nextOldest := state.Ordinal - pv
		if pv <= state.Ordinal && nextOldest > state.OldestValidOrdinal {
			changed = true
			state.OldestValidOrdinal = nextOldest
		}
		secretsContext = append(secretsContext, profileContext{
			ProfileName: profileName,
			Ordinal:     state.Ordinal,
		})
	}
	updated, err := updateSecret(secret, secretsContext)
	if err != nil {
		return err
	}
	if updated {
		w.logger.Debug("Updating ssl-profile-ordinal secret", slog.String("secret", secretName), slog.Any("context", secretsContext))
		if _, err := w.client.Update(context.TODO(), secret, v1.UpdateOptions{}); err != nil {
			return fmt.Errorf("error updating sslProfile secret anntations: %s", err)
		}
	}
	if !changed {
		return nil
	}
	w.logger.Info("SslProfile Secret Changed",
		slog.String("name", secretName),
		slog.Any("context", secretsContext),
	)
	return w.update(w)
}

func (w *ProfilesWatcher) Apply(config *qdr.RouterConfig) bool {
	changed := false
	for profileName, configured := range config.SslProfiles {
		state, ok := w.state[profileName]
		if !ok {
			continue
		}
		if configured.Ordinal != state.Ordinal {
			changed = true
			configured.Ordinal = state.Ordinal
			config.SslProfiles[profileName] = configured
		}
		if configured.OldestValidOrdinal != state.OldestValidOrdinal {
			changed = true
			configured.OldestValidOrdinal = state.OldestValidOrdinal
			config.SslProfiles[profileName] = configured
		}
	}
	return changed
}

func (w *ProfilesWatcher) UseProfiles(profiles map[string]qdr.SslProfile) {
	found := make(map[string]struct{}, len(w.state))
	for profileName := range w.state {
		found[profileName] = struct{}{}
	}
	for profileName, config := range profiles {
		delete(found, profileName)
		state, ok := w.state[profileName]
		if !ok {
			state = &profileWatcherContext{
				Ordinal:            config.Ordinal,
				OldestValidOrdinal: config.OldestValidOrdinal,
			}
			w.state[profileName] = state
		}
		if state.SecretKey != "" {
			continue
		}
		for _, secretName := range profileSecrets(profileName) {
			key := w.keyfunc(secretName)
			secret, err := w.cache.Get(key)
			if err != nil || secret == nil {
				continue
			}
			w.handleSecret(key, secret)
		}
	}
	for profileName := range found {
		state := w.state[profileName]
		delete(w.state, profileName)
		if state != nil && state.SecretKey != "" {
			secret, err := w.cache.Get(state.SecretKey)
			if err != nil || secret == nil {
				continue
			}
			w.handleSecret(state.SecretKey, secret)

		}
	}
}

func (w *ProfilesWatcher) keyfunc(name string) string {
	return w.namespace + "/" + name
}

func (w *ProfilesWatcher) checkPriorValidity(secret *corev1.Secret) uint64 {
	result := w.pvProvider.TLSPriorValidRevisions()
	if secret.ObjectMeta.Annotations == nil {
		return result
	}
	pvr, ok := secret.ObjectMeta.Annotations[AnnotationKeyTlsPriorValidRevisions]
	if !ok {
		return result
	}
	parsed, err := strconv.ParseUint(pvr, 10, 64)
	if err != nil {
		w.logger.Error(
			"Using site default tls-prior-valid-revisions for secret with invalid annotation value %q: %s",
			secret.Name, err)
		return result
	}
	return parsed
}

func profileSecrets(profileName string) []string {
	out := []string{profileName}
	if strings.HasSuffix(profileName, "-profile") {
		out = append(out, strings.TrimSuffix(profileName, "-profile"))
	}
	return out
}

func secretProfiles(secretName string) []string {
	out := []string{secretName}
	if strings.HasSuffix(secretName, "-profile") {
		out = append(out, strings.TrimSuffix(secretName, "-profile"))
	} else {
		out = append(out, secretName+"-profile")
	}
	return out
}
