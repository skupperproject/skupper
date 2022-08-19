package client

import (
	"context"

	"github.com/skupperproject/skupper/api/types"
)

func (cli *VanClient) SiteConfigRemove(ctx context.Context) error {
	return cli.ConfigMapManager(cli.Namespace).DeleteConfigMap(types.SiteConfigMapName)
}
