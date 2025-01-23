package config

import (
	"os"
	"slices"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/utils"
)

var (
	Platform string
)

// GetPlatform returns the runtime platform defined,
// where the lookup goes through the following sequence:
// - Platform variable,
// - SKUPPER_PLATFORM environment variable
// - Static platform defined by skupper switch
// - Default platform "kubernetes" otherwise.
// In case the defined platform is invalid, "kubernetes"
// will be returned.
func GetPlatform() types.Platform {
	var platform types.Platform
	for i, arg := range os.Args {
		if slices.Contains([]string{"--platform", "-p"}, arg) && i+1 < len(os.Args) {
			platformArg := os.Args[i+1]
			platform = types.Platform(platformArg)
			break
		} else if strings.HasPrefix(arg, "--platform=") || strings.HasPrefix(arg, "-p=") {
			platformArg := strings.Split(arg, "=")[1]
			platform = types.Platform(platformArg)
			break
		}
	}
	if platform == "" {
		platform = types.Platform(utils.DefaultStr(Platform,
			os.Getenv(types.ENV_PLATFORM),
			string(types.PlatformKubernetes)))
	}
	switch platform {
	case types.PlatformPodman:
		return types.PlatformPodman
	case types.PlatformDocker:
		return types.PlatformDocker
	case types.PlatformLinux:
		return types.PlatformLinux
	default:
		return types.PlatformKubernetes
	}
}
