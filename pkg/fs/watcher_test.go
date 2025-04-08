package fs

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
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

const tempDirPattern = "test.filewatcher.*"

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
	//var pathAfterClone *pathTester

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
		w, err = NewWatcher()
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

	t.Run("stop-watcher", func(t *testing.T) {
		close(stop)
	})
}

func validateOperation(t *testing.T, pt *pathTester, op operation, expectCalled int) {
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
	assert.Assert(t, utils.RetryError(time.Millisecond*200, 10, func() error {
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
		if err := os.Remove(filePath); err != nil {
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

func (t *counterHandler) OnBasePathAdded(basePath string) {
	t.m.Lock()
	defer t.m.Unlock()
	t.basePathMap[basePath] += 1
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
}

func (t *counterHandler) GetRemovedCount(basePath string) int {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.removeMap[basePath]
}

func (t *counterHandler) Filter(name string) bool {
	return t.filter(name)
}

func TestFileWatcherOld(t *testing.T) {
	tests := []struct {
		name      string
		addFirst  bool
		fileCount int
		actions   []idAction
		expected  modificationResult
	}{
		{
			name:      "watch-existing-files",
			fileCount: 10,
			actions: []idAction{
				{
					create: []int{5, 6, 7, 8, 9},
					delete: []int{5, 6, 7, 8, 9},
				},
				{
					create: []int{5, 6, 7, 8, 9},
					update: []int{5, 6, 7, 8, 9},
				},
				{
					create: []int{0, 1, 2, 3, 4},
					update: []int{0, 1, 2, 3, 4},
				},
				{
					update: []int{5, 6, 7, 8, 9},
					delete: []int{5, 6, 7, 8, 9},
				},
			},
			expected: modificationResult{
				created: map[int]int{0: 1, 1: 1, 2: 1, 3: 1, 4: 1, 5: 2, 6: 2, 7: 2, 8: 2, 9: 2},
				updated: map[int]int{0: 1, 1: 1, 2: 1, 3: 1, 4: 1, 5: 2, 6: 2, 7: 2, 8: 2, 9: 2},
				deleted: map[int]int{5: 2, 6: 2, 7: 2, 8: 2, 9: 2},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stopCh := make(chan struct{})
			defer close(stopCh)

			// create the files first
			fileIdMap, err := createTempFiles(test.fileCount)
			assert.Assert(t, err)
			defer func() {
				assert.Assert(t, cleanupTemp(fileIdMap))
			}()
			t.Logf("Created %d files", len(fileIdMap))
			t.Logf("fileIdMap: %v", fileIdMap)

			watcher := &ModificationHandler{
				fileMap: fileIdMap,
				results: modificationResult{},
			}

			// defining watchers
			w, err := NewWatcher()
			assert.Assert(t, err)
			if test.addFirst {
				w.Add(os.TempDir(), watcher)
			}
			w.Start(stopCh)
			if !test.addFirst {
				w.Add(os.TempDir(), watcher)
			}

			// process actions
			assert.Assert(t, processActions(fileIdMap, os.TempDir(), test.actions))

			// validate results
			watcher.waitDone(test.expected)
			watcher.mutex.Lock()
			defer watcher.mutex.Unlock()
			assert.Equal(t, watcher.basePath, os.TempDir())
			assert.Equal(t, watcher.onAddCalls, 1)
			t.Log(watcher.onCreateCalls, watcher.onUpdateCalls, watcher.onRemoveCalls)
			t.Log(test.expected.created, watcher.results.created)
			t.Log(test.expected.updated, watcher.results.updated)
			t.Log(test.expected.deleted, watcher.results.deleted)
			//assert.DeepEqual(t, test.expected.created, watcher.results.created)
			//assert.DeepEqual(t, test.expected.updated, watcher.results.updated)
			//assert.DeepEqual(t, test.expected.deleted, watcher.results.deleted)
		})
	}
}

func createTempFiles(count int) (idMap, error) {
	fileNames := idMap{}
	for i := 0; i < count; i++ {
		f, err := os.CreateTemp(os.TempDir(), "test-file.*")
		if err != nil {
			return nil, err
		}
		_ = f.Close()
		fileNames[i] = f.Name()
		_ = os.Remove(f.Name())
	}
	return fileNames, nil
}

func getFileId(fileMap idMap, fileName string) int {
	for id, name := range fileMap {
		if name == fileName {
			return id
		}
	}
	return -1
}

type idMap map[int]string

type modificationResult struct {
	created map[int]int
	updated map[int]int
	deleted map[int]int
}
type idAction struct {
	create []int
	update []int
	delete []int
}

type ModificationHandler struct {
	fileMap       idMap
	results       modificationResult
	mutex         sync.Mutex
	basePath      string
	onAddCalls    int
	onCreateCalls int
	onUpdateCalls int
	onRemoveCalls int
}

func (m *ModificationHandler) OnBasePathAdded(basePath string) {
	log.Println("Base Path:", basePath)
	m.basePath = basePath
	m.onAddCalls++
}

func (m *ModificationHandler) Filter(name string) bool {
	for _, fileName := range m.fileMap {
		if fileName == name {
			return true
		}
	}
	return false
}

func (m *ModificationHandler) OnCreate(s string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	log.Println("Create:", s)
	m.onCreateCalls++
	if m.results.created == nil {
		m.results.created = map[int]int{}
	}
	m.results.created[getFileId(m.fileMap, s)] += 1
}

func (m *ModificationHandler) OnUpdate(s string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	log.Println("Update:", s)
	m.onUpdateCalls++
	if m.results.updated == nil {
		m.results.updated = map[int]int{}
	}
	m.results.updated[getFileId(m.fileMap, s)] += 1
}

func (m *ModificationHandler) OnRemove(s string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	log.Println("Remove:", s)
	m.onRemoveCalls++
	if m.results.deleted == nil {
		m.results.deleted = map[int]int{}
	}
	m.results.deleted[getFileId(m.fileMap, s)] += 1
}

func (m *ModificationHandler) waitDone(expected modificationResult) {
	_ = utils.RetryError(time.Millisecond*200, 10, func() error {
		m.mutex.Lock()
		defer m.mutex.Unlock()
		if reflect.DeepEqual(m.results.created, expected.created) &&
			reflect.DeepEqual(m.results.updated, expected.updated) &&
			reflect.DeepEqual(m.results.deleted, expected.deleted) {
			return nil
		}
		return fmt.Errorf("results to not match")
	})
}

func processActions(fileIdMap idMap, baseDir string, actions []idAction) error {
	var err error
	var f *os.File

	for _, action := range actions {
		// creating files
		for _, id := range action.create {
			name := fileIdMap[id]
			f, err = os.Create(name)
			if err != nil {
				return err
			}
			if err = f.Close(); err != nil {
				return err
			}
			fmt.Println("Created file:", name)
		}
		// updating files
		for _, id := range action.update {
			name := fileIdMap[id]
			f, err = os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			if _, err = f.WriteString("sample-content\n"); err != nil {
				return err
			}
			if err := f.Sync(); err != nil {
				return err
			}
			if err = f.Close(); err != nil {
				return err
			}
			fmt.Println("Updated file:", name)
		}
		// deleting files
		for _, id := range action.delete {
			name := fileIdMap[id]
			if err = os.Remove(name); err != nil {
				return err
			}
			fmt.Println("Deleted file:", name)
		}
	}
	return nil
}

func cleanupTemp(fileIdMap idMap) error {
	for _, fileName := range fileIdMap {
		if err := os.RemoveAll(fileName); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
