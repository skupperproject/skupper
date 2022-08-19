package client

import (
	"context"
	jsonencoding "encoding/json"

	"github.com/skupperproject/skupper/api/types"
)

func (cli *VanClient) ServiceInterfaceList(ctx context.Context) ([]*types.ServiceInterface, error) {
	var vsis []*types.ServiceInterface

	current, _, err := cli.ConfigMapManager(cli.Namespace).GetConfigMap(types.ServiceInterfaceConfigMap)
	if err == nil {
		for _, v := range current.Data {
			if v != "" {
				si := types.ServiceInterface{}
				err = jsonencoding.Unmarshal([]byte(v), &si)
				if err != nil {
					return vsis, err
				} else {
					vsis = append(vsis, &si)
				}
			}
		}
		return vsis, nil
	} else {
		return vsis, err
	}
}
