package client

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
)

type testCase struct {
	doc            string
	site           types.SiteInfo
	siteNameMap    map[string]string
	isLocalSite    bool
	expectedError  string
	expectedResult []string
}

func MockGetLocalLink(cli *VanClient, namespace string, siteNameMap map[string]string) (map[string]*types.LinkStatus, error) {
	if namespace == "test6" {
		return nil, fmt.Errorf("error getting local link status")
	} else if namespace == "test7" {
		mapLinks := make(map[string]*types.LinkStatus)
		mapLinks["link1-site1"] = &types.LinkStatus{
			Connected: false,
		}
		return mapLinks, nil
	} else {
		mapLinks := make(map[string]*types.LinkStatus)
		mapLinks["link1-site1"] = &types.LinkStatus{
			Connected: true,
		}
		return mapLinks, nil
	}
}
