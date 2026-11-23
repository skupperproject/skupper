package resource

import (
	"fmt"
	"sync"
	"testing"

	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSSASuccessCache(t *testing.T) {
	for _, kind := range []string{"Gateway", "ServiceAccount"} {
		t.Run(kind, func(t *testing.T) {
			desired := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": kind,
				"metadata": map[string]interface{}{"namespace": "test", "name": "example"},
			}}
			if kind == "Gateway" {
				desired.SetAPIVersion("gateway.networking.k8s.io/v1")
			}
			desired.SetAnnotations(map[string]string{"admission": "requested"})
			observed := desired.DeepCopy()
			observed.SetUID("uid-a")
			observed.SetResourceVersion("before")
			written := observed.DeepCopy()
			written.SetResourceVersion("after")
			// Model admission rewriting an authored value: a receipt remembers
			// server acceptance, not literal equality with the request.
			written.SetAnnotations(map[string]string{"admission": "admitted"})
			cache := NewSSASuccessCache(1024, "manager")
			decision := func(c *SSASuccessCache, o, d *unstructured.Unstructured, want bool) {
				t.Helper()
				applied, err := c.Applied(o, d)
				assert.NilError(t, err)
				assert.Equal(t, applied, want)
			}
			decision(cache, nil, desired, false)
			decision(cache, written, desired, false)
			assert.NilError(t, cache.Remember(written, desired))
			decision(cache, written, desired, true)
			decision(cache, observed, desired, false) // Cache the response RV, not the request's RV.
			other := NewSSASuccessCache(1024, "other-manager")
			other.entries = cache.entries // Same receipts, different manager key.
			decision(other, written, desired, false)
			for _, change := range []func(*unstructured.Unstructured){
				func(o *unstructured.Unstructured) { o.SetResourceVersion("status-only-change") },
				func(o *unstructured.Unstructured) { o.SetUID("uid-b") },
			} {
				other := written.DeepCopy()
				change(other)
				decision(cache, other, desired, false)
			}
			changed := desired.DeepCopy()
			changed.SetAnnotations(map[string]string{"intent": "new"})
			decision(cache, written, changed, false)
			// Unknown observation fields are not parsed or compared. This is a
			// synthetic version-skew observation, not newer-server evidence.
			written.Object["futureField"] = "unknown"
			decision(cache, written, desired, true)
			before := written.DeepCopy()
			assert.NilError(t, cache.Remember(written, desired))
			decision(cache, written, desired, true)
			assert.DeepEqual(t, written, before)
			cache = NewSSASuccessCache(1024, "manager")
			decision(cache, written, desired, false)
			assert.NilError(t, NewSSASuccessCache(1024, "manager").Remember(written, desired))
			decision(cache, written, desired, false) // Receipts do not cross instances/clusters.
		})
	}
}

func TestSSASuccessCacheRejectsInvalidIdentity(t *testing.T) {
	desired := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "v1", "kind": "ServiceAccount"}}
	desired.SetName("example")
	desired.SetNamespace("test")
	written := desired.DeepCopy()
	written.SetUID("uid-a")
	written.SetResourceVersion("revision")
	cache := NewSSASuccessCache(1024, "manager")
	for _, change := range []func(*unstructured.Unstructured){
		func(o *unstructured.Unstructured) { o.SetName("other") },
		func(o *unstructured.Unstructured) { o.SetNamespace("other") },
		func(o *unstructured.Unstructured) { o.SetKind("Service") },
		func(o *unstructured.Unstructured) { o.SetAPIVersion("other/v1") },
		func(o *unstructured.Unstructured) { o.SetUID("") },
		func(o *unstructured.Unstructured) { o.SetResourceVersion("") },
	} {
		other := written.DeepCopy()
		change(other)
		_, err := cache.Applied(other, desired)
		assert.Assert(t, err != nil)
		assert.Assert(t, cache.Remember(other, desired) != nil)
	}
	_, err := cache.Applied(nil, nil)
	assert.Assert(t, err != nil)
	_, err = NewSSASuccessCache(1024, "").Applied(written, desired)
	assert.Assert(t, err != nil)
	assert.Assert(t, NewSSASuccessCache(1024, "").Remember(written, desired) != nil)
	assert.Assert(t, cache.Remember(nil, desired) != nil)
	applied, err := cache.Applied(written, desired)
	assert.NilError(t, err)
	assert.Assert(t, !applied, "invalid receipts must not establish success")
}

func TestSSASuccessCacheEvictionAndConcurrency(t *testing.T) {
	for _, capacity := range []int{2, 1024, 1025} { // small, Access default, and above default
		t.Run(fmt.Sprint(capacity), func(t *testing.T) {
			desired := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "v1", "kind": "ServiceAccount"}}
			desired.SetName("example")
			written := desired.DeepCopy()
			written.SetUID("uid-a")
			written.SetResourceVersion("revision")
			cache := NewSSASuccessCache(capacity, "manager")
			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					for range 20 {
						if err := cache.Remember(written, desired); err != nil {
							t.Error(err)
						}
						if applied, err := cache.Applied(written, desired); err != nil || !applied {
							t.Errorf("expected concurrent receipt hit: applied=%v err=%v", applied, err)
						}
					}
				})
			}
			wg.Wait()
			for i := range capacity {
				o, d := written.DeepCopy(), desired.DeepCopy()
				d.SetName(fmt.Sprintf("other-%d", i))
				o.SetName(d.GetName())
				assert.NilError(t, cache.Remember(o, d))
			}
			applied, err := cache.Applied(written, desired)
			assert.NilError(t, err)
			assert.Assert(t, !applied, "capacity+1 entries must evict the oldest receipt")
			desired.SetName("other-0")
			written.SetName(desired.GetName())
			applied, err = cache.Applied(written, desired)
			assert.NilError(t, err)
			assert.Assert(t, applied, "the configured capacity must retain all newer receipts")
		})
	}
}
