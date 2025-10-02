package fs

import (
	"io/fs"
	"os"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type ListenerHandler struct {
	BaseCustomResourceHandler
	pathProvider PathProvider
}

func NewListenerHandler(namespace string) *ListenerHandler {
	return &ListenerHandler{
		pathProvider: PathProvider{
			Namespace: namespace,
		},
	}
}

func (s *ListenerHandler) Add(resource v2alpha1.Listener) error {

	fileName := resource.Name + ".yaml"
	content, err := s.EncodeToYaml(resource)
	if err != nil {
		return err
	}

	err = s.WriteFile(s.pathProvider.GetNamespace(), fileName, content, common.Listeners)
	if err != nil {
		return err
	}

	return nil
}

func (s *ListenerHandler) Get(name string, opts GetOptions) (*v2alpha1.Listener, error) {
	var context v2alpha1.Listener
	fileName := name + ".yaml"

	if opts.RuntimeFirst == true {
		// First read from runtime directory, where output is found after bootstrap
		// has run.  If no runtime listeners try and display configured listeners
		err, file := s.ReadFile(s.pathProvider.GetRuntimeNamespace(), fileName, common.Listeners)
		if err != nil {
			if opts.LogWarning {
				os.Stderr.WriteString("Site not initialized yet\n")
			}
			err, file = s.ReadFile(s.pathProvider.GetNamespace(), fileName, common.Listeners)
			if err != nil {
				return nil, err
			}
		}

		if err = s.DecodeYaml(file, &context); err != nil {
			return nil, err
		}
	} else {
		// read from input directory to get latest config
		err, file := s.ReadFile(s.pathProvider.GetNamespace(), fileName, common.Listeners)
		if err != nil {
			return nil, err
		}
		if err := s.DecodeYaml(file, &context); err != nil {
			return nil, err
		}
	}

	return &context, nil
}

func (s *ListenerHandler) Delete(name string) error {
	fileName := name + ".yaml"

	if err := s.DeleteFile(s.pathProvider.GetNamespace(), fileName, common.Listeners); err != nil {
		return err
	}

	return nil
}

func (s *ListenerHandler) List(opts GetOptions) ([]*v2alpha1.Listener, error) {
	var listeners []*v2alpha1.Listener
	var path string
	var files []fs.DirEntry
	var err error

	// First read from runtime directory, where output is found after bootstrap
	// has run.  If no runtime listeners try and display configured listeners
	if opts.RuntimeFirst {
		path = s.pathProvider.GetRuntimeNamespace()
		err, files = s.ReadDir(path, common.Listeners)
		if err != nil {
			os.Stderr.WriteString("Site not initialized yet\n")
			path = s.pathProvider.GetNamespace()
			err, files = s.ReadDir(path, common.Listeners)
			if err != nil {
				return nil, err
			}
		}
	} else {
		// just get configured values
		path = s.pathProvider.GetNamespace()
		err, files = s.ReadDir(path, common.Listeners)
		if err != nil {
			return nil, err
		}
	}

	for _, file := range files {
		err, listener := s.ReadFile(path, file.Name(), common.Listeners)
		if err != nil {
			return nil, err
		}
		var context v2alpha1.Listener
		if err = s.DecodeYaml(listener, &context); err != nil {
			return nil, err
		}
		listeners = append(listeners, &context)
	}
	return listeners, nil
}
