package fs

import (
	"errors"
	"io/fs"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
)

type ConnectorHandler struct {
	BaseCustomResourceHandler
	pathProvider PathProvider
}

func NewConnectorHandler(namespace string) *ConnectorHandler {
	return &ConnectorHandler{
		pathProvider: PathProvider{
			Namespace: namespace,
		},
	}
}

func (s *ConnectorHandler) Add(resource v1alpha1.Connector) error {

	fileName := resource.Name + ".yaml"
	content, err := s.EncodeToYaml(resource)
	if err != nil {
		return err
	}

	err = s.WriteFile(s.pathProvider.GetNamespace(), fileName, content, "connectors")
	if err != nil {
		return err
	}

	return nil
}

func (s *ConnectorHandler) Get(name string) (*v1alpha1.Connector, error) {
	var context v1alpha1.Connector
	fileName := name + ".yaml"

	err, file := s.ReadFile(s.pathProvider.GetNamespace(), fileName, "connectors")
	if err != nil {
		return nil, err
	}

	if err = s.EncodeYaml(file, &context); err != nil {
		return nil, err
	}

	return &context, nil
}

func (s *ConnectorHandler) GetRuntime(fileName string) (*v1alpha1.Connector, []fs.DirEntry, error) {
	var context v1alpha1.Connector

	if fileName == "" {
		err, files := s.ReadDir(s.pathProvider.GetRuntimeNamespace(), "connectors")
		if err != nil {
			return nil, nil, err
		}
		return nil, files, nil
	} else {
		err, file := s.ReadFile(s.pathProvider.GetRuntimeNamespace(), fileName, "connectors")
		if err != nil {
			return nil, nil, err
		}

		if err = s.EncodeYaml(file, &context); err != nil {
			return nil, nil, err
		}

		return &context, nil, nil
	}
}

func (s *ConnectorHandler) Delete(name string) error {
	fileName := name + ".yaml"

	if err := s.DeleteFile(s.pathProvider.GetNamespace(), fileName, "connectors"); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}

	if err := s.DeleteFile(s.pathProvider.GetRuntimeNamespace(), fileName, "connectors"); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *ConnectorHandler) Update(resource v1alpha1.Site) error { return nil }
