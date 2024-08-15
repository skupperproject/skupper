package utils

import (
	"fmt"
	"math"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/pkg/nonkube/common"
	"gotest.tools/assert"
)

func TestTarball(t *testing.T) {
	const testFileContent = "test data"

	for _, tc := range []struct {
		description string
		files       int
		directories int
		levels      int
	}{
		{
			description: "only-files",
			files:       10,
			directories: 0,
			levels:      1,
		},
		{
			description: "files-and-directories",
			files:       10,
			directories: 3,
			levels:      1,
		},
		{
			description: "files-and-directories-multiple-levels",
			files:       10,
			directories: 3,
			levels:      3,
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			var cleanupList []string
			tree := generateDirectoryTree(tc.directories, tc.levels)
			baseDir, err := os.MkdirTemp("", "testtarball.*")
			assert.Assert(t, err)
			defer func() {
				assert.Assert(t, os.RemoveAll(baseDir))
				for _, fileOrDir := range cleanupList {
					assert.Assert(t, os.RemoveAll(fileOrDir))
				}
			}()
			// Generating files and asserting generation was successful
			err = createFiles(baseDir, tc.files, []byte(testFileContent), tree)
			assert.Assert(t, err, "unable to create files")
			generatedFilesExpected := tc.files * (len(tree) + 1)
			dirReader := new(common.DirectoryReader)
			filesFound, err := dirReader.ReadDir(baseDir, nil)
			assert.Assert(t, err)
			assert.Equal(t, generatedFilesExpected, len(filesFound))
			for _, file := range filesFound {
				data, err := os.ReadFile(file)
				assert.Assert(t, err)
				assert.Equal(t, string(data), testFileContent)
			}
			var savedData, savedDataExtra []byte
			var savedFile = baseDir + ".tar.gz"
			var savedFileExtra = baseDir + "-extra.tar.gz"
			t.Run(tc.description+"-SaveData", func(t *testing.T) {
				// Compressing generated files
				tb := NewTarball()
				assert.Assert(t, tb != nil)
				assert.Assert(t, tb.AddFiles(baseDir))
				savedData, err = tb.SaveData()
				assert.Assert(t, err)
				assert.Assert(t, len(savedData) > 0)
			})
			t.Run(tc.description+"-Save", func(t *testing.T) {
				// Compressing generated files
				tb := NewTarball()
				assert.Assert(t, tb != nil)
				assert.Assert(t, tb.AddFiles(baseDir))
				err = tb.Save(savedFile)
				assert.Assert(t, err)
				cleanupList = append(cleanupList, savedFile)
				savedFileStat, err := os.Stat(savedFile)
				assert.Assert(t, err)
				assert.Assert(t, savedFileStat.Size() == int64(len(savedData)))
			})
			t.Run(tc.description+"-AddFileData-SaveData", func(t *testing.T) {
				// Compressing generated files adding an extra file
				tb := NewTarball()
				assert.Assert(t, tb != nil)
				assert.Assert(t, tb.AddFiles(baseDir))
				assert.Assert(t, tb.AddFileData("sample.file", 0755, []byte(testFileContent)))
				savedDataExtra, err = tb.SaveData()
				assert.Assert(t, err)
				assert.Assert(t, len(savedDataExtra) > 0)
			})
			t.Run(tc.description+"-AddFileData-Save", func(t *testing.T) {
				// Compressing generated files adding an extra file
				tb := NewTarball()
				assert.Assert(t, tb != nil)
				assert.Assert(t, tb.AddFiles(baseDir))
				assert.Assert(t, tb.AddFileData("sample.file", 0755, []byte(testFileContent)))
				err = tb.Save(savedFileExtra)
				assert.Assert(t, err)
				cleanupList = append(cleanupList, savedFileExtra)
				savedFileStat, err := os.Stat(savedFileExtra)
				assert.Assert(t, err)
				assert.Assert(t, savedFileStat.Size() == int64(len(savedDataExtra)))
				assert.Assert(t, len(savedDataExtra) > len(savedData))
			})
			t.Run(tc.description+"-Uncompress", func(t *testing.T) {
				baseDirCopy := baseDir + ".copy"
				tb := NewTarball()
				err = tb.Extract(savedFile, baseDirCopy)
				assert.Assert(t, err)
				cleanupList = append(cleanupList, baseDirCopy)
				filesFoundCopy, err := dirReader.ReadDir(baseDirCopy, nil)
				assert.Assert(t, err)
				assert.Equal(t, generatedFilesExpected, len(filesFoundCopy))
				for _, file := range filesFoundCopy {
					data, err := os.ReadFile(file)
					assert.Assert(t, err)
					assert.Equal(t, string(data), testFileContent)
				}
			})
			t.Run(tc.description+"-UncompressExtra", func(t *testing.T) {
				baseDirExtra := baseDir + ".extra"
				tb := NewTarball()
				err = tb.Extract(savedFileExtra, baseDirExtra)
				assert.Assert(t, err)
				cleanupList = append(cleanupList, baseDirExtra)
				filesFoundExtra, err := dirReader.ReadDir(baseDirExtra, nil)
				assert.Assert(t, err)
				assert.Equal(t, generatedFilesExpected+1, len(filesFoundExtra))
				for _, file := range filesFoundExtra {
					data, err := os.ReadFile(file)
					assert.Assert(t, err)
					assert.Equal(t, string(data), testFileContent)
					if strings.HasSuffix(file, "sample.file") {
						extraFileStat, err := os.Stat(file)
						assert.Assert(t, err)
						assert.Assert(t, extraFileStat.Mode() == os.FileMode(0755))
					}
				}
			})
		})
	}
}

// createFiles iterates through the directory "tree" list,
// creating the given amount of "files" using the provided
// "content" into the "baseDir".
func createFiles(baseDir string, files int, content []byte, tree []string) error {
	tree = append(tree, "")
	for _, dir := range tree {
		err := os.MkdirAll(path.Join(baseDir, dir), 0755)
		if err != nil {
			return err
		}
		for i := 1; i <= files; i++ {
			filename := path.Join(baseDir, dir, fmt.Sprintf("file.%d", i))
			err = os.WriteFile(filename, content, 0644)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func generateDirectoryTree(dirs, levels int) []string {
	var tree []string
	indexSum := func(level int) int {
		sum := 0
		for l := level; l >= 1; l-- {
			sum += int(math.Pow(float64(dirs), float64(l)))
		}
		return sum
	}
	for level := 1; level <= levels; level++ {
		var baseDirs = []string{""}
		if level > 1 {
			initialIdx := indexSum(level - 2)
			finalIdx := indexSum(level - 1)
			baseDirs = tree[initialIdx:finalIdx]
		}
		for _, baseDir := range baseDirs {
			for dir := 1; dir <= dirs; dir++ {
				tree = append(tree, path.Join(baseDir, fmt.Sprintf("dir%d", dir)))
			}
		}
	}
	return tree
}
