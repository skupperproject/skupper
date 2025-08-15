package fs

import (
	"io/fs"
	"os"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
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

func (s *ConnectorHandler) Add(resource v2alpha1.Connector) error {

	fileName := resource.Name + ".yaml"
	content, err := s.EncodeToYaml(resource)
	if err != nil {
		return err
	}

	err = s.WriteFile(s.pathProvider.GetNamespace(), fileName, content, common.Connectors)
	if err != nil {
		return err
	}

	return nil
}

func (s *ConnectorHandler) Get(name string, opt GetOptions) (*v2alpha1.Connector, error) {
	var context v2alpha1.Connector
	fileName := name + ".yaml"

	if opt.RuntimeFirst == true {
		// First read from runtime directory, where output is found after bootstrap
		// has run.  If no runtime connectors try and display configured connectors
		err, file := s.ReadFile(s.pathProvider.GetRuntimeNamespace(), fileName, common.Connectors)
		if err != nil {
			if opt.LogWarning {
				os.Stderr.WriteString("Site not initialized yet\n")
			}
			err, file = s.ReadFile(s.pathProvider.GetNamespace(), fileName, common.Connectors)
			if err != nil {
				return nil, err
			}
		}
		if err := s.DecodeYaml(file, &context); err != nil {
			return nil, err
		}
	} else {
		// read from input directory to get latest config
		err, file := s.ReadFile(s.pathProvider.GetNamespace(), fileName, common.Connectors)
		if err != nil {
			return nil, err
		}
		if err := s.DecodeYaml(file, &context); err != nil {
			return nil, err
		}
	}

	return &context, nil
}

func (s *ConnectorHandler) Delete(name string) error {
	fileName := name + ".yaml"

	if err := s.DeleteFile(s.pathProvider.GetNamespace(), fileName, common.Connectors); err != nil {
		return err
	}

	return nil
}

func (s *ConnectorHandler) List(opts GetOptions) ([]*v2alpha1.Connector, error) {
	var connectors []*v2alpha1.Connector
	var path string
	var files []fs.DirEntry
	var err error

	// First read from runtime directory, where output is found after bootstrap
	// has run.  If no runtime connectors try and display configured connectors
	if opts.RuntimeFirst {
		path = s.pathProvider.GetRuntimeNamespace()
		err, files = s.ReadDir(path, common.Connectors)
		if err != nil {
			os.Stderr.WriteString("Site not initialized yet\n")
			path = s.pathProvider.GetNamespace()
			err, files = s.ReadDir(path, common.Connectors)
			if err != nil {
				return nil, err
			}
		}
	} else {
		// just get configured values
		path = s.pathProvider.GetNamespace()
		err, files = s.ReadDir(path, common.Connectors)
		if err != nil {
			return nil, err
		}
	}

	for _, file := range files {
		err, connector := s.ReadFile(path, file.Name(), common.Connectors)
		if err != nil {
			return nil, err
		}
		var context v2alpha1.Connector
		if err = s.DecodeYaml(connector, &context); err != nil {
			return nil, err
		}
		connectors = append(connectors, &context)
	}
	return connectors, nil
}
