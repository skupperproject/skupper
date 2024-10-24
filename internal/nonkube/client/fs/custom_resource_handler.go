package fs

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"os"
	"path/filepath"
)

type CustomResourceHandler[T any] interface {
	Add(T) error
	Update(T) error
	Get(name string) T
	Delete(name string) error

	//Common methods
	EncodeToYaml(resource interface{}) (string, error)
	WriteFile(path string, name string, content string, kind string) error
}

type BaseCustomResourceHandler struct{}

func (b *BaseCustomResourceHandler) EncodeToYaml(resource interface{}) (string, error) {

	return utils.Encode("yaml", resource)
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
