package fs

import (
	"os"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	corev1 "k8s.io/api/core/v1"
)

type ConfigMapHandler struct {
	BaseCustomResourceHandler
	pathProvider PathProvider
}

func NewConfigMapHandler(namespace string) *ConfigMapHandler {
	return &ConfigMapHandler{
		pathProvider: PathProvider{
			Namespace: namespace,
		},
	}
}

func (s *ConfigMapHandler) getPath(runtime bool) string {
	if runtime {
		return s.pathProvider.GetRuntimeNamespace()
	}
	return s.pathProvider.GetNamespace()
}

func (s *ConfigMapHandler) Add(resource corev1.ConfigMap, runtime bool) error {

	fileName := resource.Name + ".yaml"
	content, err := s.EncodeToYaml(resource)
	if err != nil {
		return err
	}

	err = s.WriteFile(s.getPath(runtime), fileName, content, common.ConfigMaps)
	if err != nil {
		return err
	}

	return nil
}

func (s *ConfigMapHandler) Update(resource corev1.ConfigMap, runtime bool) error {
	cm, err := s.Get(resource.Name, GetOptions{
		RuntimeFirst: runtime,
	})
	if err != nil {
		return err
	}
	cm.Data = resource.Data
	return s.Add(resource, runtime)
}

func (s *ConfigMapHandler) Get(name string, opts GetOptions) (*corev1.ConfigMap, error) {
	var context corev1.ConfigMap
	fileName := name + ".yaml"

	if opts.RuntimeFirst == true {
		// First read from runtime directory, where output is found after bootstrap
		// has run.  If no runtime configmaps try and display configured configmaps
		err, file := s.ReadFile(s.pathProvider.GetRuntimeNamespace(), fileName, common.ConfigMaps)
		if err != nil {
			if opts.LogWarning {
				os.Stderr.WriteString("Site not initialized yet\n")
			}
			err, file = s.ReadFile(s.pathProvider.GetNamespace(), fileName, common.ConfigMaps)
			if err != nil {
				return nil, err
			}
		}

		if err = s.DecodeYaml(file, &context); err != nil {
			return nil, err
		}
	} else {
		// read from input directory to get latest config
		err, file := s.ReadFile(s.pathProvider.GetNamespace(), fileName, common.ConfigMaps)
		if err != nil {
			return nil, err
		}
		if err := s.DecodeYaml(file, &context); err != nil {
			return nil, err
		}
	}

	return &context, nil
}

func (s *ConfigMapHandler) Delete(name string, runtime bool) error {
	fileName := name + ".yaml"

	if err := s.DeleteFile(s.getPath(runtime), fileName, common.ConfigMaps); err != nil {
		return err
	}

	return nil
}

func (s *ConfigMapHandler) List() ([]*corev1.ConfigMap, error) {
	var configmaps []*corev1.ConfigMap

	// First read from runtime directory, where output is found after bootstrap
	// has run.  If no runtime configmaps try and display configured configmaps
	path := s.pathProvider.GetRuntimeNamespace()
	err, files := s.ReadDir(path, common.ConfigMaps)
	if err != nil {
		os.Stderr.WriteString("Site not initialized yet\n")
		path = s.pathProvider.GetNamespace()
		err, files = s.ReadDir(path, common.ConfigMaps)
		if err != nil {
			return nil, err
		}
	}

	for _, file := range files {
		err, configmap := s.ReadFile(path, file.Name(), common.ConfigMaps)
		if err != nil {
			return nil, err
		}
		var context corev1.ConfigMap
		if err = s.DecodeYaml(configmap, &context); err != nil {
			return nil, err
		}
		configmaps = append(configmaps, &context)
	}
	return configmaps, nil
}
