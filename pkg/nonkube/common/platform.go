package common

import (
	"fmt"
	"os"
	"path"

	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"sigs.k8s.io/yaml"
)

type NamespacePlatformLoader struct {
	PathProvider api.InternalPathProvider
	Platform     string `yaml:"platform"`
}

func (s *NamespacePlatformLoader) GetPathProvider() api.InternalPathProvider {
	if s.PathProvider == nil {
		return api.GetInternalOutputPath
	}
	return s.PathProvider
}

func (s *NamespacePlatformLoader) Load(namespace string) (string, error) {
	if namespace == "" {
		namespace = "default"
	}
	pathProvider := s.GetPathProvider()
	internalPath := pathProvider(namespace, api.InternalBasePath)
	platformFile, err := os.ReadFile(path.Join(internalPath, "platform.yaml"))
	if err != nil {
		return "", fmt.Errorf("failed to read platform config file for namespace %s: %w", namespace, err)
	}
	if err = yaml.Unmarshal(platformFile, s); err != nil {
		return "", fmt.Errorf("failed to unmarshal platform config file for namespace %s: %w", namespace, err)
	}
	return s.Platform, nil
}
