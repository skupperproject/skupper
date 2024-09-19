package utils

import (
	"fmt"
	"os"
	"path"
)

type FilenameFilter func(string) bool
type DirectoryReader struct{}

func (f *DirectoryReader) ReadDir(dirname string, filter FilenameFilter) ([]string, error) {
	dir, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	dirInfo, err := dir.Stat()
	if err != nil {
		return nil, err
	}
	if !dirInfo.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dirname)
	}
	files, err := dir.ReadDir(0)
	if err != nil {
		return nil, err
	}
	var fileNames []string
	for _, file := range files {
		if file.IsDir() {
			recursiveFiles, err := f.ReadDir(path.Join(dirname, file.Name()), filter)
			if err != nil {
				return nil, err
			}
			fileNames = append(fileNames, recursiveFiles...)
		} else {
			if filter == nil || filter(file.Name()) {
				fileNames = append(fileNames, path.Join(dirname, file.Name()))
			}
		}
	}
	return fileNames, nil
}
