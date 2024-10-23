package fs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	"sigs.k8s.io/yaml"
)

type CustomResourceHandler[T any] interface {
	Add(T) error
	Update(T) error
	Get(name string) (T, error)
	List() ([]T, error)
	Delete(name string) error

	//Common methods
	EncodeToYaml(resource interface{}) (string, error)
	WriteFile(path string, name string, content string, kind string) error
	DecodeYaml(content []byte, resource interface{}) (interface{}, error)
	ReadFile(path string, name string, kind string) (error, []byte)
	DeleteFile(path string, name string, kind string) error
	ReadDir(path string, kind string) (error, []fs.DirEntry)
}

type BaseCustomResourceHandler struct{}

func (b *BaseCustomResourceHandler) EncodeToYaml(resource interface{}) (string, error) {

	return utils.Encode("yaml", resource)
}

func (b *BaseCustomResourceHandler) DecodeYaml(content []byte, resource interface{}) error {

	if err := yaml.Unmarshal(content, &resource); err != nil {
		return err
	}
	return nil
}

func (b *BaseCustomResourceHandler) WriteFile(path string, name string, content string, kind string) error {

	fullPath := filepath.Join(path, kind)
	completeFilePath := filepath.Join(fullPath, name)

	// Create the directories recursively
	err := os.MkdirAll(fullPath, 0775)
	if err != nil {
		return fmt.Errorf("failed to create directories: %s", err)
	}

	file, err := os.Create(completeFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %s", err)
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return err
	}

	defer file.Sync()

	fmt.Println("File written to", completeFilePath)

	return nil
}

func (b *BaseCustomResourceHandler) ReadFile(path string, name string, kind string) (error, []byte) {

	fullPath := filepath.Join(path, kind)
	completeFilePath := filepath.Join(fullPath, name)

	file, err := os.ReadFile(completeFilePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %s", err), nil
	}

	return nil, file
}

func (b *BaseCustomResourceHandler) DeleteFile(path string, name string, kind string) error {
	var completeFilePath string

	fullPath := filepath.Join(path, kind)
	completeFilePath = filepath.Join(fullPath, name)

	if err := os.RemoveAll(completeFilePath); err != nil {
		return fmt.Errorf("failed to delete file: %s", err)
	}

	return nil
}

func (b *BaseCustomResourceHandler) ReadDir(path string, kind string) (error, []fs.DirEntry) {

	fullPath := filepath.Join(path, kind)

	files, err := os.ReadDir(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %s", err), nil
	}

	return nil, files
}
