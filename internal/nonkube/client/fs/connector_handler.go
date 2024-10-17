package fs

import (
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

func (s *ConnectorHandler) Update(resource v1alpha1.Connector) error { return nil }

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

func (s *ConnectorHandler) Delete(name string) error {
	fileName := name + ".yaml"

	err := s.DeleteFile(s.pathProvider.GetNamespace(), fileName, "connectors")
	if err != nil {
		return err
	}
	return nil
}
