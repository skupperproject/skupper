package bootstrap

import (
	"fmt"
	"os/user"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/images"
	internalclient "github.com/skupperproject/skupper/internal/nonkube/client/compat"
)

func CheckRouterImageDrift(namespace string) (bool, error) {
	endpoint := internalclient.GetDefaultContainerEndpoint()
	cli, err := internalclient.NewCompatClient(endpoint, "")
	if err != nil {
		return false, fmt.Errorf("image check: failed to create client: %w", err)
	}

	containerName := namespace + "-skupper-router"
	c, err := cli.ContainerInspect(containerName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}
	if c == nil {
		return false, nil
	}

	return c.Image != images.GetRouterImageName(), nil
}

func CheckControllerImageDrift() (bool, string, error) {
	endpoint := internalclient.GetDefaultContainerEndpoint()
	cli, err := internalclient.NewCompatClient(endpoint, "")
	if err != nil {
		return false, "", fmt.Errorf("controller image check: failed to create client: %w", err)
	}

	currentUser, err := user.Current()
	if err != nil {
		return false, types.SystemReloadTypeManual, fmt.Errorf("controller image check: failed to get current user: %w", err)
	}
	containerName := fmt.Sprintf("%s-skupper-controller", currentUser.Username)
	c, err := cli.ContainerInspect(containerName)
	if err != nil || c == nil {
		return true, types.SystemReloadTypeManual, nil
	}

	reloadType := c.Env[types.ENV_SYSTEM_AUTO_RELOAD]
	if reloadType == "" {
		reloadType = types.SystemReloadTypeManual
	}

	return c.Image != images.GetSystemControllerImageName(), reloadType, nil
}
