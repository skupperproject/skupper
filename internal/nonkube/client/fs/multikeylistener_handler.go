package fs

import (
	"os"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type MultiKeyListenerHandler struct {
	BaseCustomResourceHandler
	pathProvider PathProvider
}

func NewMultiKeyListenerHandler(namespace string) *MultiKeyListenerHandler {
	return &MultiKeyListenerHandler{
		pathProvider: PathProvider{
			Namespace: namespace,
		},
	}
}

func (s *MultiKeyListenerHandler) Add(resource v2alpha1.MultiKeyListener) error {

	fileName := resource.Name + ".yaml"
	content, err := s.EncodeToYaml(resource)
	if err != nil {
		return err
	}

	err = s.WriteFile(s.pathProvider.GetNamespace(), fileName, content, common.MultiKeyListeners)
	if err != nil {
		return err
	}

	return nil
}

func (s *MultiKeyListenerHandler) Get(name string, opts GetOptions) (*v2alpha1.MultiKeyListener, error) {
	var context v2alpha1.MultiKeyListener
	fileName := name + ".yaml"

	if opts.RuntimeFirst == true {
		// First read from runtime directory, where output is found after bootstrap
		// has run.  If no runtime multikeylisteners try and display configured ones
		err, file := s.ReadFile(s.pathProvider.GetRuntimeNamespace(), fileName, common.MultiKeyListeners)
		if err != nil {
			if opts.LogWarning {
				os.Stderr.WriteString("Site not initialized yet\n")
			}
			err, file = s.ReadFile(s.pathProvider.GetNamespace(), fileName, common.MultiKeyListeners)
			if err != nil {
				return nil, err
			}
		}

		if err = s.DecodeYaml(file, &context); err != nil {
			return nil, err
		}
	} else {
		// read from input directory to get latest config
		err, file := s.ReadFile(s.pathProvider.GetNamespace(), fileName, common.MultiKeyListeners)
		if err != nil {
			return nil, err
		}
		if err := s.DecodeYaml(file, &context); err != nil {
			return nil, err
		}
	}

	return &context, nil
}

func (s *MultiKeyListenerHandler) Delete(name string) error {
	fileName := name + ".yaml"

	if err := s.DeleteFile(s.pathProvider.GetNamespace(), fileName, common.MultiKeyListeners); err != nil {
		return err
	}

	return nil
}

func (s *MultiKeyListenerHandler) List() ([]*v2alpha1.MultiKeyListener, error) {
	var multiKeyListeners []*v2alpha1.MultiKeyListener

	// First read from runtime directory, where output is found after bootstrap
	// has run.  If no runtime multikeylisteners try and display configured ones
	path := s.pathProvider.GetRuntimeNamespace()
	err, files := s.ReadDir(path, common.MultiKeyListeners)
	if err != nil {
		os.Stderr.WriteString("Site not initialized yet\n")
		path = s.pathProvider.GetNamespace()
		err, files = s.ReadDir(path, common.MultiKeyListeners)
		if err != nil {
			return nil, err
		}
	}

	for _, file := range files {
		err, mkl := s.ReadFile(path, file.Name(), common.MultiKeyListeners)
		if err != nil {
			return nil, err
		}
		var context v2alpha1.MultiKeyListener
		if err = s.DecodeYaml(mkl, &context); err != nil {
			return nil, err
		}
		multiKeyListeners = append(multiKeyListeners, &context)
	}
	return multiKeyListeners, nil
}
