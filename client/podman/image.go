package podman

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/go-openapi/runtime"
	"github.com/skupperproject/skupper/client/generated/libpod/client/images"
	"github.com/skupperproject/skupper/pkg/container"
)

const (
	imagePullRecommendation = `
If the image is being pulled from an authenticated registry,
make sure to log in first, using:

    podman login <registry-url>

In case you are using a custom authentication file, you should
set the REGISTRY_AUTH_FILE environment variable.`
)

func (p *PodmanRestClient) ImageList() ([]*container.Image, error) {
	cli := images.New(p.RestClient, formats)
	param := images.NewImageListLibpodParams()
	param.All = boolTrue()
	res, err := cli.ImageListLibpod(param)
	if err != nil {
		return nil, fmt.Errorf("error listing images: %v", err)
	}
	var imgs []*container.Image
	for _, img := range res.Payload {
		for _, imgName := range img.RepoTags {
			// for _, imgName := range img.Names {
			imgs = append(imgs, &container.Image{
				Id:         img.ID,
				Repository: imgName,
				Created:    fmt.Sprint(img.Created),
			})
		}
	}
	return imgs, nil
}

func (p *PodmanRestClient) ImageInspect(id string) (*container.Image, error) {
	cli := images.New(p.RestClient, formats)
	param := images.NewImageInspectLibpodParams()
	param.Name = id
	res, err := cli.ImageInspectLibpod(param)
	if err != nil {
		return nil, fmt.Errorf("error inspecting image %s: %v", id, err)
	}
	img := &container.Image{
		Id:         res.Payload.ID,
		Repository: res.Payload.RepoTags[0],
		Digest:     string(res.Payload.Digest),
		Created:    res.Payload.Created.String(),
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

func (p *PodmanRestClient) ImagePull(ctx context.Context, id string) error {
	params := images.NewImagePullLibpodParams()
	params.Reference = &id
	params.TLSVerify = new(bool)
	params.AllTags = new(bool)
	params.Quiet = new(bool)
	params.Arch = stringP("")
	params.OS = stringP("")
	params.Variant = stringP("")
	params.Policy = stringP("always")
	params.XRegistryAuth = getXRegistryAuth(id)

	// Need to do that as the default response reader is being closed too soon
	reader := &responseReaderJSONErrorBody{}
	op := &runtime.ClientOperation{
		ID:                 "ImagePullLibpod",
		Method:             "POST",
		PathPattern:        "/libpod/images/pull",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json", "application/x-tar"},
		Schemes:            []string{"http", "https"},
		Params:             params,
		Reader:             reader,
		Context:            ctx,
		Client:             params.HTTPClient,
	}
	res, err := p.RestClient.Submit(op)
	if err != nil {
		return &Error{
			Err:            fmt.Errorf("error pulling image %s: %v", id, err),
			Recommendation: imagePullRecommendation,
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
					Recommendation: imagePullRecommendation,
				}
			}
		}
	}
	return nil
}

func getXRegistryAuth(image string) *string {
	authFile := os.Getenv("REGISTRY_AUTH_FILE")
	// use the default
	if authFile == "" {
		return nil
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
				registryAuthMap := map[string]map[string]string{}
				registryAuthMap[imageServer] = map[string]string{
					"username": credentials[0],
					"password": credentials[1],
				}
				registryAuthMapJson, _ := json.Marshal(registryAuthMap)
				registryAuthMapB64 := base64.StdEncoding.EncodeToString(registryAuthMapJson)
				return &registryAuthMapB64
			}
		}
	}
	return nil
}

type Error struct {
	Recommendation string
	Err            error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s\n\nRecommendation: %s\n", e.Err, e.Recommendation)
}
