package controller

import (
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func TestNamespacesHandler(t *testing.T) {
	if os.Getuid() == 0 {
		t.Logf("Test is running as root, /var/lib/skupper will be used instead of $XDG_DATA_HOME")
	}
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	namespacesPath := api.GetDefaultOutputNamespacesPath()
	assert.Assert(t, os.MkdirAll(namespacesPath, 0755))
	assert.Assert(t, os.MkdirAll(path.Join(namespacesPath, "before"), 0755))

	nsHandler, err := NewNamespacesHandler()
	assert.Assert(t, err)
	assert.Assert(t, nsHandler != nil)

	stopCh := make(chan struct{})
	wg := &sync.WaitGroup{}

	assert.Assert(t, nsHandler.Start(stopCh, wg))

	t.Run("assert-running", func(t *testing.T) {
		assert.Assert(t, utils.Retry(time.Millisecond*100, 10, func() (bool, error) {
			return len(nsHandler.namespaces) == 1, nil
		}), "namespace controller did not start for namespace 'before'")
	})

	var nsCtrlAfter *NamespaceController
	t.Run("create-namespace", func(t *testing.T) {
		assert.Assert(t, os.Mkdir(path.Join(namespacesPath, "after"), 0755))
		assert.Assert(t, utils.Retry(time.Millisecond*100, 10, func() (bool, error) {
			return len(nsHandler.namespaces) == 2, nil
		}), "namespace controller did not start for namespace 'after'")
		nsCtrlAfter = nsHandler.namespaces["after"]
		assert.Assert(t, nsCtrlAfter != nil)
	})

	t.Run("delete-namespace", func(t *testing.T) {
		nsCtrlAfterStopped := false
		go func() {
			<-nsCtrlAfter.stopCh
			nsCtrlAfterStopped = true
		}()
		assert.Assert(t, os.Remove(path.Join(namespacesPath, "after")))
		assert.Assert(t, utils.Retry(time.Millisecond*100, 10, func() (bool, error) {
			return nsCtrlAfterStopped && len(nsHandler.namespaces) == 1, nil
		}), "namespace controller did not stop")
	})

	t.Run("stopped", func(t *testing.T) {
		stopped := false
		go func() {
			wg.Wait()
			stopped = true
		}()
		close(stopCh)
		assert.Assert(t, utils.Retry(time.Millisecond*100, 10, func() (bool, error) {
			return stopped, nil
		}), "controller did not stop")
	})
}
