package utils

import (
	"os"
	"path"
	"slices"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestDirectoryReader(t *testing.T) {
	for _, test := range []struct {
		description   string
		files         int
		directories   int
		levels        int
		filter        func(string) bool
		expectedFiles []string
	}{
		{
			description:   "empty-directory",
			files:         0,
			directories:   0,
			levels:        0,
			expectedFiles: []string{},
		},
		{
			description: "empty-directory-tree",
			files:       0,
			directories: 3,
			levels:      3,
		},
		{
			description: "full-directory",
			files:       3,
			directories: 0,
			levels:      1,
			expectedFiles: []string{
				"file.1",
				"file.2",
				"file.3",
			},
		},
		{
			description: "full-directory-tree",
			files:       3,
			directories: 3,
			levels:      1,
			expectedFiles: []string{
				"file.1",
				"file.2",
				"file.3",
				"dir1/file.1",
				"dir1/file.2",
				"dir1/file.3",
				"dir2/file.1",
				"dir2/file.2",
				"dir2/file.3",
				"dir3/file.1",
				"dir3/file.2",
				"dir3/file.3",
			},
		},
		{
			description: "full-directory-tree-filtered",
			files:       3,
			directories: 3,
			levels:      1,
			filter: func(name string) bool {
				return strings.HasSuffix(name, ".3")
			},
			expectedFiles: []string{
				"file.3",
				"dir1/file.3",
				"dir2/file.3",
				"dir3/file.3",
			},
		},
	} {
		t.Run(test.description, func(t *testing.T) {
			baseDir, err := os.MkdirTemp("", "testdirreader.*")
			assert.Assert(t, err)
			defer func() {
				assert.Assert(t, os.RemoveAll(baseDir))
			}()
			// Generating files and asserting generation was successful
			tree := generateDirectoryTree(test.directories, test.levels)
			err = createFiles(baseDir, test.files, []byte("sample data"), tree)
			assert.Assert(t, err, "unable to create files")

			r := new(DirectoryReader)
			files, err := r.ReadDir(baseDir, test.filter)
			assert.NilError(t, err, "error reading directory")
			if test.filter == nil {
				assert.Equal(t, len(files), expectedCreatedFiles(test.directories, test.levels, test.files))
			} else {
				if len(test.expectedFiles) > 0 {
					assert.Equal(t, len(files), len(test.expectedFiles))
				}
			}
			for _, expectedFile := range test.expectedFiles {
				assert.Assert(t, slices.Contains(files, path.Join(baseDir, expectedFile)))
			}
		})
	}
}
