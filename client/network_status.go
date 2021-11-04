package client

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/server"
)

func (cli *VanClient) NetworkStatus() (*[]types.SiteInfo, error) {

	sites, err := server.GetSiteInfo(cli.Namespace, cli.KubeClient, cli.RestConfig)

	if err != nil {
		return nil, err
	}
	return &sites, nil
}
