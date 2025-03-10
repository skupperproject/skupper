package fs

import (
	"bytes"
	"fmt"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"os"
)

type TokenHandler struct {
	BaseCustomResourceHandler
	pathProvider PathProvider
}

func NewTokenHandler(namespace string) *TokenHandler {
	return &TokenHandler{
		pathProvider: PathProvider{
			Namespace: namespace,
		},
	}
}

func (s *TokenHandler) Add(resource v2alpha1.Link) error {
	return nil
}

func (s *TokenHandler) Get(name string, opts GetOptions) (*v2alpha1.Link, error) {
	return nil, nil
}

func (s *TokenHandler) Delete(name string) error {
	return nil
}

func (s *TokenHandler) List(opts GetOptions) ([]string, error) {
	var linkName string
	var endpointHost string
	var path string

	if opts.Attributes != nil {
		linkName = opts.Attributes["linkName"]
		endpointHost = opts.Attributes["endpointHost"]
		path = opts.Attributes["tokenPath"]
	}

	var fileNames []string

	tokens, _ := os.ReadDir(path)
	if tokens == nil || len(tokens) == 0 {
		return nil, fmt.Errorf("there are no links created")
	}
	for _, token := range tokens {
		if !token.IsDir() {
			tokenFile, errFile := os.ReadFile(path + "/" + token.Name())
			if errFile != nil {
				return nil, fmt.Errorf("error reading file %s: %s", path+"/"+token.Name(), errFile)
			}

			parts := bytes.Split(tokenFile, []byte("---"))

			for _, part := range parts {

				if bytes.Contains(part, []byte("kind: Link")) {

					var link v2alpha1.Link
					err := s.DecodeYaml(part, &link)
					if err != nil {
						return nil, err
					}

					nameMatches := linkName != "" && link.Name == linkName

					endpointMatches := false
					if endpointHost != "" {

						for _, endpoint := range link.Spec.Endpoints {
							if endpoint.Host == endpointHost {
								endpointMatches = true
								break
							}
						}
					}

					if (nameMatches && endpointHost == "") ||
						(endpointMatches && linkName == "") ||
						(nameMatches && endpointMatches) ||
						(linkName == "" && endpointHost == "") {
						fileNames = append(fileNames, token.Name())
					}

				}
			}

		}
	}

	return fileNames, nil
}
