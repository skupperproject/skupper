package resource

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/lru"
)

// SSASuccessCache remembers successful SSA requests.
type SSASuccessCache struct {
	entries *lru.Cache
	manager string
}

func NewSSASuccessCache(capacity int, manager string) *SSASuccessCache {
	return &SSASuccessCache{entries: lru.New(capacity), manager: manager}
}

type ssaRequestKey struct {
	GVK                          schema.GroupVersionKind
	Namespace, Name, Manager, RV string
	UID                          types.UID
	Intent                       [sha256.Size]byte
}

func ssaKey(observed, desired *unstructured.Unstructured, manager string) (ssaRequestKey, error) {
	if desired == nil || manager == "" {
		return ssaRequestKey{}, fmt.Errorf("desired and manager are required")
	}
	if observed.GroupVersionKind() != desired.GroupVersionKind() || observed.GetName() != desired.GetName() || observed.GetNamespace() != desired.GetNamespace() {
		return ssaRequestKey{}, fmt.Errorf("observation identity does not match desired")
	}
	if observed.GetUID() == "" || observed.GetResourceVersion() == "" {
		return ssaRequestKey{}, fmt.Errorf("observation needs UID and resourceVersion")
	}
	// encoding/json sorts map keys; list ordering is intentionally not normalized.
	raw, err := json.Marshal(desired.Object)
	if err != nil {
		return ssaRequestKey{}, err
	}
	return ssaRequestKey{desired.GroupVersionKind(), desired.GetNamespace(), desired.GetName(), manager, observed.GetResourceVersion(), observed.GetUID(), sha256.Sum256(raw)}, nil
}

// Applied reports whether desired was already applied and observed is still
// the server’s response to that apply.
func (c *SSASuccessCache) Applied(observed, desired *unstructured.Unstructured) (bool, error) {
	if desired == nil || c.manager == "" {
		return false, fmt.Errorf("desired and manager are required")
	}
	if observed == nil {
		return false, nil
	}
	k, err := ssaKey(observed, desired, c.manager)
	if err != nil {
		return false, err
	}
	_, hit := c.entries.Get(k)
	return hit, nil
}

// Remember records a successful, non-dry-run apply of desired that returned written.
func (c *SSASuccessCache) Remember(written, desired *unstructured.Unstructured) error {
	if written == nil {
		return fmt.Errorf("successful response required")
	}
	k, err := ssaKey(written, desired, c.manager)
	if err != nil {
		return err
	}
	c.entries.Add(k, struct{}{})
	return nil
}
