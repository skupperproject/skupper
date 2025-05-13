package controller

import (
	"os"
	"path"
	"sync/atomic"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
)

func TestNewRouterConfigHandler(t *testing.T) {
	var err error
	var nsCtrl *NamespaceController

	tempDir := t.TempDir()
	if os.Getuid() == 0 {
		api.DefaultRootDataHome = tempDir
	} else {
		t.Setenv("XDG_DATA_HOME", tempDir)
	}

	stopCh := make(chan struct{})
	namespace := "test-router-config-handler"
	routerConfigHandler := NewRouterConfigHandler(stopCh, namespace)
	routerConfigPath := api.GetInternalOutputPath(namespace, api.RouterConfigPath)
	routerConfigFile := path.Join(routerConfigPath, "skrouterd.json")
	assert.Assert(t, os.MkdirAll(routerConfigPath, 0755))
	f, err := os.Create(routerConfigFile)
	assert.Assert(t, err)
	defer f.Close()
	callback := &testCallback{}
	routerConfigHandler.AddCallback(callback)

	t.Run("start-namespace-controller", func(t *testing.T) {
		nsCtrl, err = NewNamespaceController(namespace)
		assert.Assert(t, err)
		nsCtrl.prepare = func() {
			nsCtrl.watcher.Add(routerConfigPath, routerConfigHandler)
		}
		nsCtrl.Start()
	})

	t.Run("create-router-config-file", func(t *testing.T) {
	})

	t.Run("assert-started-once", func(t *testing.T) {
		err = utils.Retry(time.Second, 5, func() (bool, error) {
			t.Log(callback.started.Load())
			return callback.started.Load() == 1, nil
		})
		assert.Assert(t, err)
	})

	t.Run("assert-stopped-once", func(t *testing.T) {
		assert.Assert(t, os.Remove(routerConfigFile))
		err = utils.Retry(time.Second, 5, func() (bool, error) {
			t.Log(callback.stopped.Load())
			return callback.stopped.Load() == 1, nil
		})
		assert.Assert(t, err)
	})

	t.Run("recreate-router-config-file", func(t *testing.T) {
		f, err := os.Create(routerConfigFile)
		assert.Assert(t, err)
		defer f.Close()

		err = utils.Retry(time.Second, 5, func() (bool, error) {
			t.Log(callback.started.Load())
			return callback.started.Load() == 2, nil
		})
		assert.Assert(t, err)
	})

	t.Run("remove-router-config-path", func(t *testing.T) {
		assert.Assert(t, os.RemoveAll(routerConfigPath))
		err = utils.Retry(time.Second, 5, func() (bool, error) {
			t.Log(callback.stopped.Load())
			return callback.stopped.Load() == 2, nil
		})
		assert.Assert(t, err)
	})

}

type testCallback struct {
	started atomic.Int32
	stopped atomic.Int32
}

func (t *testCallback) Start(stopCh <-chan struct{}) {
	go t.wait(stopCh)
	t.started.Add(1)
}

func (t *testCallback) Stop() {
	t.stopped.Add(1)
}

func (t *testCallback) wait(stopCh <-chan struct{}) {
	<-stopCh
	t.Stop()
}

func (t *testCallback) Id() string {
	return "test-callback"
}
