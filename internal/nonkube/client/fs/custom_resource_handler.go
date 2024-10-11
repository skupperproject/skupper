package fs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	"sigs.k8s.io/yaml"
)

type CustomResourceHandler[T any] interface {
	Add(T) error
	Update(T) error
	Get(name string) T
	Delete(name string) error

	//Common methods
	EncodeToYaml(resource interface{}) (string, error)
	WriteFile(path string, name string, content string, kind string) error
	EncodeYaml(content []byte, resource interface{}) (interface{}, error)
	ReadFile(path string, name string, kind string) (error, []byte)
	DeleteFile(path string, name string, kind string) error
}

type BaseCustomResourceHandler struct{}

func (b *BaseCustomResourceHandler) EncodeToYaml(resource interface{}) (string, error) {

	return utils.Encode("yaml", resource)
}

// TBD better name
func (b *BaseCustomResourceHandler) EncodeYaml(content []byte, resource interface{}) error {

	if err := yaml.Unmarshal(content, &resource); err != nil {
		return err
	}
	return nil
}

func (b *BaseCustomResourceHandler) WriteFile(path string, name string, content string, kind string) error {

	// Resolve the home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	fullPath := filepath.Join(homeDir, path, kind)
	completeFilePath := filepath.Join(fullPath, name)

	// Create the directories recursively
	err = os.MkdirAll(fullPath, 0775)
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

	// Resolve the home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err, nil
	}

	fullPath := filepath.Join(homeDir, path, kind)
	completeFilePath := filepath.Join(fullPath, name)

	file, err := os.ReadFile(completeFilePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %s", err), nil
	}

	return nil, file
}

func (b *BaseCustomResourceHandler) DeleteFile(path string, name string, kind string) error {
	var completeFilePath string

	// Resolve the home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	fullPath := filepath.Join(homeDir, path, kind)
	completeFilePath = filepath.Join(fullPath, name)

	if err := os.RemoveAll(completeFilePath); err != nil {
		return fmt.Errorf("failed to delete file: %s", err)
	}

	return nil
}
