package client

import (
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	"github.com/skupperproject/skupper/api/types"
)

func (cli *VanClient) VanServiceInterfaceCreate(ctx context.Context, service *types.ServiceInterface, rs *types.Results) {
	owner, err := getRootObject(cli)
	if err == nil {
		validateServiceInterface(service, rs)
                if rs.ContainsError() {
			rs.AddError("Aborting service interface creation due to error.")
                        return
		}
		updateServiceInterface(service, false, owner, cli, rs)
	} else if errors.IsNotFound(err) {
                rs.AddError("Skupper not initialised in %s", cli.Namespace)
	} else {
                rs.AddError("getRootObject error: %w", err)
                return
	}
}
