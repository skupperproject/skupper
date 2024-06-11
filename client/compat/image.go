package compat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/generated/libpod/client/images_compat"
)

const (
	imagePullRecommendation = `
If the image is being pulled from an authenticated registry,
make sure to log in first, using:

    %s login <registry-url>

In case you are using a custom authentication file, you should
set the REGISTRY_AUTH_FILE environment variable.`
)

type Error struct {
	Recommendation string
	Err            error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s\n\nRecommendation: %s\n", e.Err, e.Recommendation)
}

func (c *CompatClient) ImageList() ([]*container.Image, error) {
	cli := images_compat.New(c.RestClient, formats)
	params := images_compat.NewImageListParams()
	params.All = boolTrue()
	res, err := cli.ImageList(params)
	if err != nil {
		return nil, fmt.Errorf("error listing images: %v", ToAPIError(err))
	}
	var imgs []*container.Image
	for _, img := range res.Payload {
		for _, imgName := range img.RepoTags {
			imageId := strings.Replace(*img.ID, "sha256:", "", 1)
			newImg := &container.Image{
				Id:         imageId,
				Repository: imgName,
				Created:    time.Unix(*img.Created, 0).UTC().Format(time.RFC3339),
			}
			if len(img.RepoDigests) > 0 {
				newImg.Digest = img.RepoDigests[len(img.RepoDigests)-1]
			}
			imgs = append(imgs, newImg)

		}
	}
	return imgs, nil
}

func (c *CompatClient) ImageInspect(id string) (*container.Image, error) {
	cli := images_compat.New(c.RestClient, formats)
	params := images_compat.NewImageInspectParams()
	params.Name = id
	res, err := cli.ImageInspect(params)
	if err != nil {
		return nil, fmt.Errorf("error inspecting image %s: %v", id, ToAPIError(err))
	}
	img := &container.Image{
		Id:         strings.Replace(res.Payload.ID, "sha256:", "", 1),
		Repository: res.Payload.RepoTags[0],
		Created:    res.Payload.Created,
	}
	if len(res.Payload.RepoDigests) > 0 {
		img.Digest = res.Payload.RepoDigests[len(res.Payload.RepoDigests)-1]
	}
	if !strings.HasPrefix(img.Id, id) {
		for _, name := range res.Payload.RepoTags {
			if strings.Contains(name, id) {
				img.Repository = name
				break
			}
		}
	}
	return img, nil
}

func (c *CompatClient) ImagePull(ctx context.Context, id string) error {
	params := images_compat.NewImageCreateParams()
	imgTag := toImageTag(id)
	params.FromImage = stringP(imgTag.Image)
	params.Tag = stringP(imgTag.Tag)
	params.XRegistryAuth = c.getXRegistryAuth(id)
	params.Context = ctx

	// Need to do that as the default response reader is being closed too soon
	reader := &responseReaderJSONErrorBody{}
	op := &runtime.ClientOperation{
		ID:                 "ImageCreate",
		Method:             "POST",
		PathPattern:        "/images/create",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/x-tar"},
		Schemes:            []string{"http", "https"},
		Params:             params,
		Reader:             reader,
		Context:            ctx,
		Client:             params.HTTPClient,
	}
	res, err := c.RestClient.Submit(op)
	if err != nil {
		return &Error{
			Err:            fmt.Errorf("error pulling image %s: %v", id, err),
			Recommendation: fmt.Sprintf(imagePullRecommendation, c.engine),
		}
	}
	// eventually err is nil but auth problems are reported as json after a string msg
	if resStr, ok := res.(string); ok {
		resStrClean := strings.TrimLeftFunc(resStr, func(r rune) bool {
			if r == '{' {
				return false
			}
			return true
		})
		var jsonRes map[string]interface{}
		if err = json.Unmarshal([]byte(resStrClean), &jsonRes); err == nil {
			if errMsg, ok := jsonRes["error"]; ok && errMsg != "" {
				return &Error{
					Err:            fmt.Errorf("unable to pull image %s: %v", id, errMsg),
					Recommendation: fmt.Sprintf(imagePullRecommendation, c.engine),
				}
			}
		}
	}
	return nil
}

type imageTag struct {
	Image string
	Tag   string
}

func toImageTag(imageName string) imageTag {
	repositorySplit := strings.Split(imageName, "/")
	hostPath := strings.Join(repositorySplit[:len(repositorySplit)-1], "/")
	repository := repositorySplit[len(repositorySplit)-1]
	sep := ":"
	if strings.Contains(repository, "@sha256:") {
		sep = "@"
	}
	nameTag := strings.Split(repository, sep)
	tag := "latest"
	if len(nameTag) == 2 {
		tag = nameTag[1]
	}
	repositoryName := nameTag[0]
	if len(hostPath) > 0 {
		repositoryName = strings.Join([]string{hostPath, repositoryName}, "/")
	}
	return imageTag{
		Image: repositoryName,
		Tag:   tag,
	}
}

func (c *CompatClient) getXRegistryAuth(image string) *string {
	authFile := os.Getenv("REGISTRY_AUTH_FILE")
	// use the default
	if authFile == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		defaultAuthFile := filepath.Join(homeDir, ".docker", "config.json")
		info, err := os.Stat(defaultAuthFile)
		if err != nil || info.IsDir() {
			return nil
		}
		authFile = defaultAuthFile
	}
	data, err := os.ReadFile(authFile)
	if err != nil {
		fmt.Printf("Unable to read REGISTRY_AUTH_FILE - %s", err)
		fmt.Println()
		return nil
	}
	var jsonData map[string]interface{}
	if err = json.Unmarshal(data, &jsonData); err != nil {
		fmt.Printf("Unable to parse REGISTRY_AUTH_FILE - %s", err)
		fmt.Println()
		return nil
	}
	if auths, ok := jsonData["auths"]; ok {
		authsMap, ok := auths.(map[string]interface{})
		if !ok {
			return nil
		}
		imageServer := strings.Split(image, "/")[0]
		if serverData, ok := authsMap[imageServer]; ok {
			if serverDataMap, ok := serverData.(map[string]interface{}); ok {
				authInfo, ok := serverDataMap["auth"]
				if !ok {
					return nil
				}
				authInfoStr := authInfo.(string)
				credentialsBytes, err := base64.StdEncoding.DecodeString(authInfoStr)
				if err != nil {
					fmt.Printf("Unable to decode base64 auth info for %s - %s", imageServer, err)
					fmt.Println()
					return nil
				}
				credentials := strings.Split(string(credentialsBytes), ":")
				if len(credentials) != 2 {
					return nil
				}
				registryAuthMap := map[string]interface{}{}
				if c.engine == "podman" {
					registryAuthMap[imageServer] = map[string]string{
						"username": credentials[0],
						"password": credentials[1],
					}
				} else {
					registryAuthMap = map[string]interface{}{
						"username":      credentials[0],
						"password":      credentials[1],
						"serveraddress": imageServer,
					}
				}
				registryAuthMapJson, _ := json.Marshal(registryAuthMap)
				registryAuthMapB64 := base64.StdEncoding.EncodeToString(registryAuthMapJson)
				return &registryAuthMapB64
			}
		}
	}
	return nil
}
