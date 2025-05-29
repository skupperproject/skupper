package runtime

import (
	"fmt"

	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
)

func GetLocalRouterAddress(namespace string) (string, error) {
	port, err := GetLocalRouterPort(namespace)
	if err != nil {
		return "", err
	}
	address := fmt.Sprintf("amqps://127.0.0.1:%d", port)
	return address, nil
}

func GetLocalRouterPort(namespace string) (int, error) {
	client := fs.NewRouterAccessHandler(namespace)
	ra, err := client.Get("skupper-local")
	if err != nil {
		return 0, fmt.Errorf("unable to determine router port: %w", err)
	}
	if len(ra.Spec.Roles) == 0 {
		return 0, fmt.Errorf("no roles defined on RouterAccess: %s", ra.Name)
	}
	return ra.Spec.Roles[0].Port, nil
}
