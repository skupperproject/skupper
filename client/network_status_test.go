package client

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"
	"testing"
)

type testCase struct {
	doc            string
	site           types.SiteInfo
	siteNameMap    map[string]string
	isLocalSite    bool
	expectedError  string
	expectedResult []string
}

func TestGetFormattedLinks(t *testing.T) {

	testcases := []testCase{
		{
			doc: "map-is-not-initialized",
			site: types.SiteInfo{
				Name:      "",
				Namespace: "test1",
				SiteId:    "",
			},
			isLocalSite:   false,
			expectedError: "the site name map used to format the links has no values or it is not initialized",
		},
		{
			doc: "map-is-empty",
			site: types.SiteInfo{
				Name:      "",
				Namespace: "test2",
				SiteId:    "",
				Links:     []string{"link1"},
			},
			isLocalSite:   false,
			siteNameMap:   map[string]string{},
			expectedError: "the site name map used to format the links has no values or it is not initialized",
		},
		{
			doc: "map-does-not-contain-the-value",
			site: types.SiteInfo{
				Name:      "",
				Namespace: "test3",
				SiteId:    "",
				Links:     []string{"link1"},
			},
			isLocalSite:    false,
			siteNameMap:    asMap([]string{"link2=site2"}),
			expectedResult: []string{"link1-"},
		},
		{
			doc: "returns-results-formatted",
			site: types.SiteInfo{
				Name:      "",
				Namespace: "test4",
				SiteId:    "",
				Links:     []string{"link2"},
			},
			isLocalSite:    false,
			siteNameMap:    asMap([]string{"link2=site2"}),
			expectedResult: []string{"link2-site2"},
		},
		{
			doc: "returns-results-formatted-with-link-id-trimmed",
			site: types.SiteInfo{
				Name:      "",
				Namespace: "test5",
				SiteId:    "",
				Links:     []string{"link123456"},
			},
			isLocalSite:    false,
			siteNameMap:    asMap([]string{"link123456=site1"}),
			expectedResult: []string{"link123-site1"},
		},
		{
			doc: "returns-error-after-checking-link-local-status",
			site: types.SiteInfo{
				Name:      "",
				Namespace: "test6",
				SiteId:    "",
				Links:     []string{"link1"},
			},
			isLocalSite:   true,
			siteNameMap:   asMap([]string{"link1=site1"}),
			expectedError: "error getting local link status",
		},
		{
			doc: "returns-results-formatted-with-local-data-in-red",
			site: types.SiteInfo{
				Name:      "",
				Namespace: "test7",
				SiteId:    "",
				Links:     []string{"link1"},
			},
			isLocalSite:    true,
			siteNameMap:    asMap([]string{"link1=site1"}),
			expectedResult: []string{"\u001B[1;31mlink1-site1 (link not connected)\u001B[0m"},
		},
		{
			doc: "returns-results-formatted-with-local-data",
			site: types.SiteInfo{
				Name:      "",
				Namespace: "test8",
				SiteId:    "",
				Links:     []string{"link1"},
			},
			isLocalSite:    true,
			siteNameMap:    asMap([]string{"link1=site1"}),
			expectedResult: []string{"link1-site1"},
		},
	}

	for _, tc := range testcases {

		cli, _ := newMockClient(tc.doc, "", "")

		links, err := GetFormattedLinks(MockGetLocalLink, cli, tc.site, tc.siteNameMap, tc.isLocalSite)

		if err != nil {
			assert.Assert(t, err.Error() == tc.expectedError, "expected: '%s', but found: '%s'", tc.expectedError, err.Error())
		} else {
			assert.DeepEqual(t, links, tc.expectedResult)
		}

	}
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
