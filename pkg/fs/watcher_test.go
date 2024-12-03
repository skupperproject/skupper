package fs

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/skupperproject/skupper/pkg/utils"
	"gotest.tools/v3/assert"
)

func TestFileWatcher(t *testing.T) {
	tests := []struct {
		name      string
		fileCount int
		actions   []idAction
		expected  modificationResult
	}{
		{
			name:      "watch-existing-files",
			fileCount: 10,
			actions: []idAction{
				{
					update: []int{0, 1, 2, 3, 4},
					delete: []int{5, 6, 7, 8, 9},
				},
				{
					create: []int{5, 6, 7, 8, 9},
					update: []int{5, 6, 7, 8, 9},
				},
			},
			expected: modificationResult{
				created: map[int]int{5: 1, 6: 1, 7: 1, 8: 1, 9: 1},
				updated: map[int]int{0: 1, 1: 1, 2: 1, 3: 1, 4: 1, 5: 1, 6: 1, 7: 1, 8: 1, 9: 1},
				deleted: map[int]int{5: 1, 6: 1, 7: 1, 8: 1, 9: 1},
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

			watcher := &ModificationHandler{
				fileMap: fileIdMap,
				results: modificationResult{},
			}

			// defining watchers
			w, err := NewWatcher()
			assert.Assert(t, err)
			w.Add(os.TempDir(), watcher, fileNamesFilters(fileIdMap)...)
			w.Start(stopCh)

			// process actions
			assert.Assert(t, processActions(fileIdMap, os.TempDir(), test.actions))

			// validate results
			watcher.waitDone(test.expected)
			watcher.mutex.Lock()
			defer watcher.mutex.Unlock()
			assert.DeepEqual(t, test.expected.created, watcher.results.created)
			assert.DeepEqual(t, test.expected.updated, watcher.results.updated)
			assert.DeepEqual(t, test.expected.deleted, watcher.results.deleted)
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
	}
	return fileNames, nil
}

func fileNamesFilters(fileIdMap idMap) []*regexp.Regexp {
	res := []*regexp.Regexp{}
	for _, fileName := range fileIdMap {
		res = append(res, regexp.MustCompile(fmt.Sprintf(`%s$`, fileName)))
	}
	return res
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
	fileMap idMap
	results modificationResult
	mutex   sync.Mutex
}

func (m *ModificationHandler) OnCreate(s string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.results.created == nil {
		m.results.created = map[int]int{}
	}
	m.results.created[getFileId(m.fileMap, s)] += 1
}

func (m *ModificationHandler) OnUpdate(s string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.results.updated == nil {
		m.results.updated = map[int]int{}
	}
	m.results.updated[getFileId(m.fileMap, s)] += 1
}

func (m *ModificationHandler) OnRemove(s string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
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
		}
		// updating files
		for _, id := range action.update {
			name := fileIdMap[id]
			f, err = os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			if _, err = f.WriteString("sample-content\n"); err != nil {
				return err
			}
			if err = f.Close(); err != nil {
				return err
			}
		}
		// deleting files
		for _, id := range action.delete {
			name := fileIdMap[id]
			if err = os.RemoveAll(name); err != nil {
				return err
			}
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
