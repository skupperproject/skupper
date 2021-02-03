package utils

import (
	"github.com/skupperproject/skupper/api/types"
	"os"
)

func RouteOrLoadBalancerFromEnv() string {
	useRoutes := os.Getenv("TESTS_USE_OCP_ROUTES")
	if useRoutes == "" {
		return types.IngressLoadBalancerString
	}
	return types.IngressRouteString
}
