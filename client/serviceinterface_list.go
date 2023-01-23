package client

import (
	"context"
	jsonencoding "encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

func (cli *VanClient) ServiceInterfaceList(ctx context.Context) ([]*types.ServiceInterface, error) {
	var vsis []*types.ServiceInterface

	current, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get(ctx, types.ServiceInterfaceConfigMap, metav1.GetOptions{})
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
