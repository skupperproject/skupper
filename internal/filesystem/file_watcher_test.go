package filesystem

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/skupperproject/skupper/internal/utils"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/strings/slices"
)

const (
	tempDirPattern        = "test.filewatcher.*"
	counterHandlerTimeout = 101 * time.Millisecond
)

type operation int

const (
	created operation = iota
	updated
	removed
)

func TestFileWatcher(t *testing.T) {
	stop := make(chan struct{})
	var w *FileWatcher
	var err error
	var filterTxt = func(name string) bool {
		return strings.HasSuffix(name, ".txt")
	}
	var pathBefore = &pathTester{handler: newCounterHandler(filterTxt)}
	var pathAfter = &pathTester{handler: newCounterHandler(filterTxt)}
	var pathAfterClone = &pathTester{handler: newCounterHandler(filterTxt)}

	defer func() {
		var err error
		for _, p := range []*pathTester{
			pathBefore,
			pathAfter,
		} {
			err = errors.Join(os.RemoveAll(p.path))
		}
		assert.Assert(t, err)
	}()

	t.Run("create-watcher", func(t *testing.T) {
		w, err = NewWatcher(slog.String("owner", "test.watcher"))
		assert.Assert(t, err)
	})

	// path is added into watcher before watcher is started
	// with files already present in the file-system
	t.Run("add-path-before-starting", func(t *testing.T) {
		assert.Assert(t, pathBefore.mkBasePath())
		pathBefore.generateFileNames(10, ".txt")
		pathBefore.generateFileNames(10, ".invalid")
		assert.Assert(t, pathBefore.create())
		w.Add(pathBefore.path, pathBefore.handler)
	})

	t.Run("start-watcher", func(t *testing.T) {
		w.Start(stop)
	})

	t.Run("validate-path-before-created", func(t *testing.T) {
		validateOperation(t, pathBefore, created, 1)
	})

	// path is added into watcher after watcher is started
	t.Run("add-path-after-started", func(t *testing.T) {
		// path to be created later
		pathAfter.path = filepath.Join(os.TempDir(), uuid.NewString())
		w.Add(pathAfter.path, pathAfter.handler)
		assert.Assert(t, pathAfter.mkBasePath())

		// ensure base path added (otherwise unable to determine the amount of calls to OnCreate)
		validateBasedPathAdded(t, pathAfter, 1)

		pathAfter.generateFileNames(10, ".txt")
		pathAfter.generateFileNames(10, ".invalid")
		assert.Assert(t, pathAfter.create())
		assert.Assert(t, pathAfter.update())
		assert.Assert(t, pathAfter.update())
	})

	t.Run("validate-path-after", func(t *testing.T) {
		validateOperation(t, pathAfter, created, 1)
		validateOperation(t, pathAfter, updated, 2)
	})

	t.Run("delete-files-path-after", func(t *testing.T) {
		assert.Assert(t, pathAfter.remove())
		validateOperation(t, pathAfter, removed, 1)
	})

	t.Run("add-second-handler-to-path-after", func(t *testing.T) {
		pathAfterClone.path = pathAfter.path
		pathAfterClone.files = pathAfter.files
		w.Add(pathAfterClone.path, pathAfterClone.handler)
		assert.Assert(t, pathAfter.create())
		assert.Assert(t, pathAfter.update())
	})

	t.Run("validate-path-after-handlers", func(t *testing.T) {
		validateOperation(t, pathAfterClone, created, 1)
		validateOperation(t, pathAfter, created, 2)
		validateOperation(t, pathAfterClone, updated, 1)
		validateOperation(t, pathAfter, updated, 3)
	})

	t.Run("remove-files-path-after", func(t *testing.T) {
		assert.Assert(t, pathAfter.remove())
		validateOperation(t, pathAfter, removed, 2)
		validateOperation(t, pathAfterClone, removed, 1)
	})

	t.Run("multi-add-delete-path-after", func(t *testing.T) {
		pathAfter.handler.reset()
		pathAfterClone.handler.reset()
		expectedCalled := 100
		for i := 0; i < expectedCalled; i++ {
			assert.Assert(t, pathAfter.create())
			assert.Assert(t, pathAfter.remove())
		}
		validateOperation(t, pathAfter, created, expectedCalled)
		validateOperation(t, pathAfterClone, created, expectedCalled)
		validateOperation(t, pathAfter, removed, expectedCalled)
		validateOperation(t, pathAfterClone, removed, expectedCalled)
	})

	t.Run("remove-path-before-basedir", func(t *testing.T) {
		assert.Assert(t, pathBefore.remove())
		validateOperation(t, pathBefore, removed, 1)
		// removing basedir and adding
		assert.Assert(t, pathBefore.rmBasePath())
		validateOperation(t, pathBefore, removed, 1)
	})

	t.Run("add-path-before-basedir", func(t *testing.T) {
		pathBefore.handler.reset()
		assert.Assert(t, pathBefore.mkBasePath())
		assert.Assert(t, pathBefore.create())
		validateOperation(t, pathBefore, created, 1)
	})

	t.Run("path-before-timeout", func(t *testing.T) {
		pathBefore.handler.setTimeout(true)
		assert.Assert(t, pathBefore.remove())
		validateOperation(t, pathBefore, removed, 1)
	})

	t.Run("stop-watcher", func(t *testing.T) {
		close(stop)
	})
}

func validateOperation(t *testing.T, pt *pathTester, op operation, expectCalled int) {
	t.Helper()
	var m func(string) int
	var method string
	var verb string
	switch op {
	case created:
		m = pt.handler.GetCreatedCount
		method = "OnCreate"
		verb = "created"
	case updated:
		m = pt.handler.GetUpdatedCount
		method = "OnUpdate"
		verb = "updated"
	case removed:
		m = pt.handler.GetRemovedCount
		method = "OnRemove"
		verb = "removed"
	}
	assert.Assert(t, utils.RetryError(time.Millisecond*100, 10, func() error {
		totalFiltered := 0
		expectedFiles := slices.Filter(nil, pt.files, pt.handler.filter)
		for _, fileName := range expectedFiles {
			totalFiltered++
			found := m(fileName)
			if expectCalled != found {
				return fmt.Errorf("expected %s to be called %d times for %q, but got: %d", method, expectCalled, fileName, found)
			}
		}
		if len(expectedFiles) != totalFiltered {
			return fmt.Errorf("expected %d files %s, but got %d", len(expectedFiles), verb, totalFiltered)
		}
		return nil
	}))

	// assert not filtered files did not trigger operation
	ignoredFiles := slices.Filter(nil, pt.files, func(s string) bool {
		return !pt.handler.filter(s)
	})
	for _, fileName := range ignoredFiles {
		found := m(fileName)
		assert.Equal(t, 0, found, "expected %s to be ignored, but %s was triggered", fileName, method)
	}
}

func validateBasedPathAdded(t *testing.T, pt *pathTester, expectedCalled int) {
	assert.Assert(t, utils.RetryError(time.Millisecond*200, 10, func() error {
		found := pt.handler.GetBasePathAddedCount(pt.path)
		if found != expectedCalled {
			return fmt.Errorf("expected OnBasePathAdded to be called %d times for %q, but got: %d",
				expectedCalled, pt.path, found)
		}
		return nil
	}))
}

type pathTester struct {
	path    string
	handler *counterHandler
	files   []string
}

func (p *pathTester) generateFileNames(count int, suffix string) {
	for i := 0; i < count; i++ {
		name := filepath.Join(p.path, rand.String(16)+suffix)
		p.files = append(p.files, name)
	}
}

func (p *pathTester) mkBasePath() error {
	var err error
	if p.path == "" {
		p.path, err = os.MkdirTemp("", tempDirPattern)
	} else {
		err = os.Mkdir(p.path, 0777)
	}
	return err
}

func (p *pathTester) rmBasePath() error {
	return os.RemoveAll(p.path)
}

func (p *pathTester) create() error {
	for _, file := range p.files {
		if _, err := os.Stat(file); err == nil {
			return fmt.Errorf("file %q already exists and cannot be created", file)
		}
		f, err := os.Create(file)
		if err != nil {
			return err
		}
		if err = f.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (p *pathTester) update() error {
	for _, file := range p.files {
		stat, err := os.Stat(file)
		if err == nil && stat.Mode().IsRegular() {
			f, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return err
			}
			if _, err = f.WriteString("hello world\n"); err != nil {
				return err
			}
			if err = f.Sync(); err != nil {
				return err
			}
			if err = f.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *pathTester) remove() error {
	for _, filePath := range p.files {
		if err := os.RemoveAll(filePath); err != nil {
			return err
		}
	}
	return nil
}

type counterHandler struct {
	basePathMap map[string]int
	createMap   map[string]int
	updateMap   map[string]int
	removeMap   map[string]int
	filter      func(string) bool
	m           sync.RWMutex
	timeout     bool
}

func newCounterHandler(filter func(string) bool) *counterHandler {
	return &counterHandler{
		basePathMap: make(map[string]int),
		createMap:   make(map[string]int),
		updateMap:   make(map[string]int),
		removeMap:   make(map[string]int),
		filter:      filter,
	}
}

func (t *counterHandler) setTimeout(timeout bool) {
	t.m.Lock()
	defer t.m.Unlock()
	t.timeout = timeout
}

func (t *counterHandler) reset() {
	t.m.Lock()
	defer t.m.Unlock()
	t.basePathMap = map[string]int{}
	t.createMap = map[string]int{}
	t.updateMap = map[string]int{}
	t.removeMap = map[string]int{}
}

func (t *counterHandler) OnBasePathAdded(basePath string) {
	t.m.Lock()
	defer t.m.Unlock()
	t.basePathMap[basePath] += 1
	if t.timeout {
		time.Sleep(counterHandlerTimeout)
	}
}

func (t *counterHandler) GetBasePathAddedCount(basePath string) int {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.basePathMap[basePath]
}

func (t *counterHandler) OnCreate(name string) {
	t.m.Lock()
	defer t.m.Unlock()
	t.createMap[name] += 1
	if t.timeout {
		time.Sleep(counterHandlerTimeout)
	}
}

func (t *counterHandler) GetCreatedCount(basePath string) int {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.createMap[basePath]
}

func (t *counterHandler) OnUpdate(name string) {
	t.m.Lock()
	defer t.m.Unlock()
	t.updateMap[name] += 1
	if t.timeout {
		time.Sleep(counterHandlerTimeout)
	}
}

func (t *counterHandler) GetUpdatedCount(basePath string) int {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.updateMap[basePath]
}

func (t *counterHandler) OnRemove(name string) {
	t.m.Lock()
	defer t.m.Unlock()
	t.removeMap[name] += 1
	if t.timeout {
		time.Sleep(counterHandlerTimeout)
	}
}

func (t *counterHandler) GetRemovedCount(basePath string) int {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.removeMap[basePath]
}

func (t *counterHandler) Filter(name string) bool {
	return t.filter(name)
}
