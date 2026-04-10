package fs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/pkg/nonkube/api"

	"sigs.k8s.io/yaml"
)

type GetOptions struct {
	RuntimeFirst bool
	LogWarning   bool
	Attributes   map[string]string
	InputOnly    bool
	RuntimeOnly  bool
}

type CustomResourceHandler[T any] interface {
	Add(T) error
	Get(name string, opts GetOptions) (T, error)
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

	completeFilePath := filepath.Join(path, fmt.Sprintf("%s-%s", kind, name))

	// Create the directories recursively
	err := os.MkdirAll(path, 0775)
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

	writtenPath := completeFilePath
	if api.IsRunningInContainer() {
		writtenPath = strings.Replace(completeFilePath, "/output", api.GetHostDataHome(), 1)
	}
	fmt.Println("File written to", writtenPath)

	return nil
}

func (b *BaseCustomResourceHandler) ReadFile(path string, name string, kind string) (error, []byte) {

	completeFilePath := filepath.Join(path, fmt.Sprintf("%s-%s", kind, name))
	if strings.HasPrefix(name, kind+"-") {
		completeFilePath = filepath.Join(path, name)
	}

	file, err := os.ReadFile(completeFilePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %s", err), nil
	}

	return nil, file
}

func (b *BaseCustomResourceHandler) DeleteFile(path string, name string, kind string) error {
	var completeFilePath string

	completeFilePath = filepath.Join(path, fmt.Sprintf("%s-%s", kind, name))
	if kind == "" && name == "" {
		completeFilePath = path
	} else if strings.HasPrefix(name, kind+"-") {
		completeFilePath = filepath.Join(path, name)
	}

	_, err := os.Stat(completeFilePath)
	if os.IsNotExist(err) {
		return err
	}

	if err := os.RemoveAll(completeFilePath); err != nil {
		return fmt.Errorf("failed to delete file: %s", err)
	}

	return nil
}

func (b *BaseCustomResourceHandler) ReadDir(path string, kind string) (error, []fs.DirEntry) {

	filter := func(files []fs.DirEntry) (ret []fs.DirEntry) {
		prefix := fmt.Sprintf("%s-", kind)
		for _, f := range files {
			if strings.HasPrefix(f.Name(), prefix) {
				ret = append(ret, f)
			}
		}
		return ret
	}
	files, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read directory: %s", err), nil
	}

	return nil, filter(files)
}
