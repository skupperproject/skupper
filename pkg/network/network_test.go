package network

import (
	"encoding/json"
	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"
	"testing"
)

func createTestSkupperStatus() *SkupperStatus {
	var networkStatus types.NetworkStatusInfo
	jsonTest := "{\"addresses\":[{\"recType\":\"ADDRESS\",\"identity\":\"8184b711-75c7-46f8-a64f-6374434e0d0b\",\"startTime\":1696200453171254,\"endTime\":0,\"name\":\"backend:8080\",\"protocol\":\"tcp\",\"listenerCount\":2,\"connectorCount\":1}],\"siteStatus\":[{\"site\":{\"recType\":\"SITE\",\"identity\":\"9c88f6dc-ae0e-4dad-956b-def9737b095f\",\"startTime\":1696195725000000,\"endTime\":0,\"source\":\"9c88f6dc-ae0e-4dad-956b-def9737b095f\",\"platform\":\"kubernetes\",\"name\":\"public1\",\"nameSpace\":\"public1\",\"siteVersion\":\"5a16a97\",\"policy\":\"disabled\"},\"routerStatus\":[{\"router\":{\"recType\":\"ROUTER\",\"identity\":\"j97sp:0\",\"parent\":\"9c88f6dc-ae0e-4dad-956b-def9737b095f\",\"startTime\":1696195912209992,\"endTime\":0,\"source\":\"j97sp:0\",\"name\":\"0/public1-skupper-router-5945f87d48-j97sp\",\"namespace\":\"public1\",\"imageName\":\"skupper-router\",\"imageVersion\":\"latest\",\"hostname\":\"skupper-router-5945f87d48-j97sp\",\"buildVersion\":\"abf0523351acc25de50c9d15557847d0c679cd83\"},\"links\":[{\"recType\":\"LINK\",\"identity\":\"j97sp:1\",\"parent\":\"j97sp:0\",\"startTime\":1696195946862535,\"endTime\":0,\"source\":\"j97sp:0\",\"mode\":\"interior\",\"name\":\"public2-skupper-router-6d5cb849dd-2hrrk\",\"direction\":\"incoming\"}],\"listeners\":[{\"recType\":\"LISTENER\",\"identity\":\"j97sp:12\",\"parent\":\"j97sp:0\",\"startTime\":1696200453261051,\"endTime\":0,\"source\":\"j97sp:0\",\"name\":\"backend:8080\",\"destHost\":\"0.0.0.0\",\"destPort\":\"1027\",\"protocol\":\"tcp\",\"address\":\"backend:8080\",\"addressId\":\"8184b711-75c7-46f8-a64f-6374434e0d0b\"}],\"connectors\":[{\"recType\":\"CONNECTOR\",\"identity\":\"j97sp:11\",\"parent\":\"j97sp:0\",\"startTime\":1696200453259514,\"endTime\":0,\"source\":\"j97sp:0\",\"destHost\":\"10.244.0.34\",\"destPort\":\"8080\",\"protocol\":\"tcp\",\"address\":\"backend:8080\",\"target\":\"backend-778cb759c9-thbsv\",\"addressId\":\"8184b711-75c7-46f8-a64f-6374434e0d0b\"}]}]},{\"site\":{\"recType\":\"SITE\",\"identity\":\"429c2780-003d-44cc-9a91-4139885c7d20\",\"startTime\":1696195811000000,\"endTime\":0,\"source\":\"429c2780-003d-44cc-9a91-4139885c7d20\",\"platform\":\"kubernetes\",\"name\":\"public2\",\"nameSpace\":\"public2\",\"siteVersion\":\"5a16a97\",\"policy\":\"disabled\"},\"routerStatus\":[{\"router\":{\"recType\":\"ROUTER\",\"identity\":\"2hrrk:0\",\"parent\":\"429c2780-003d-44cc-9a91-4139885c7d20\",\"startTime\":1696195907837396,\"endTime\":0,\"source\":\"2hrrk:0\",\"name\":\"0/public2-skupper-router-6d5cb849dd-2hrrk\",\"namespace\":\"public2\",\"imageName\":\"skupper-router\",\"imageVersion\":\"latest\",\"hostname\":\"skupper-router-6d5cb849dd-2hrrk\",\"buildVersion\":\"abf0523351acc25de50c9d15557847d0c679cd83\"},\"links\":[{\"recType\":\"LINK\",\"identity\":\"2hrrk:16\",\"parent\":\"2hrrk:0\",\"startTime\":1696195946234499,\"endTime\":0,\"source\":\"2hrrk:0\",\"mode\":\"interior\",\"name\":\"public1-skupper-router-5945f87d48-j97sp\",\"linkCost\":1,\"direction\":\"outgoing\"}],\"listeners\":[{\"recType\":\"LISTENER\",\"identity\":\"2hrrk:23\",\"parent\":\"2hrrk:0\",\"startTime\":1696200453171254,\"endTime\":0,\"source\":\"2hrrk:0\",\"name\":\"backend:8080\",\"destHost\":\"0.0.0.0\",\"destPort\":\"1027\",\"protocol\":\"tcp\",\"address\":\"backend:8080\",\"addressId\":\"8184b711-75c7-46f8-a64f-6374434e0d0b\"}],\"connectors\":null}]}]}\n"
	_ = json.Unmarshal([]byte(jsonTest), &networkStatus)

	return &SkupperStatus{NetworkStatus: &networkStatus}

}

func TestGetServiceSitesMap(t *testing.T) {

	skupperStatus := createTestSkupperStatus()

	results := skupperStatus.GetServiceSitesMap()
	expectedService := "backend:8080"
	expectedSite1 := "9c88f6dc-ae0e-4dad-956b-def9737b095f"
	expectedNamespace1 := "public1"
	expectedSite2 := "429c2780-003d-44cc-9a91-4139885c7d20"
	expectedNamespace2 := "public2"

	assert.Check(t, len(results) == 1)

	for key, value := range results {

		assert.Equal(t, key, expectedService)
		assert.Check(t, len(value) == 2)

		for _, site := range value {
			if site.Site.Namespace == expectedNamespace1 {
				assert.Equal(t, site.Site.Identity, expectedSite1)
			} else if site.Site.Namespace == expectedNamespace2 {
				assert.Equal(t, site.Site.Identity, expectedSite2)
			} else {
				t.Errorf("unexpected site for this test")
			}
		}
	}

}

func TestGetServiceLabelsMap(t *testing.T) {
	skupperStatus := createTestSkupperStatus()

	// Add test data to skupperStatus

	services := []*types.ServiceInterface{
		{
			Address: "example",
			Ports:   []int{8080},
			Labels:  map[string]string{"label": "value"},
		},
	}

	results := skupperStatus.GetServiceLabelsMap(services)

	if len(results) != len(services) {
		t.Errorf("Expected length %d, but got %d", len(services), len(results))
	}

	for key, value := range results {
		assert.Equal(t, key, "example:8080")
		assert.Equal(t, value["label"], "value")
	}

}

func TestGetSiteTargetsMap(t *testing.T) {
	skupperStatus := createTestSkupperStatus()

	expectedSite1 := "9c88f6dc-ae0e-4dad-956b-def9737b095f"
	expectedSite2 := "429c2780-003d-44cc-9a91-4139885c7d20"
	expectedService := "backend:8080"

	result := skupperStatus.GetSiteTargetsMap()

	assert.Check(t, result[expectedSite2] == nil)
	assert.Check(t, result[expectedSite1][expectedService].Address == expectedService)

}

func TestGetRouterSiteMap(t *testing.T) {
	skupperStatus := createTestSkupperStatus()

	expectedRouter1 := "public1-skupper-router-5945f87d48-j97sp"
	expectedSite1 := "9c88f6dc-ae0e-4dad-956b-def9737b095f"
	expectedRouter2 := "public2-skupper-router-6d5cb849dd-2hrrk"
	expectedSite2 := "429c2780-003d-44cc-9a91-4139885c7d20"

	results := skupperStatus.GetRouterSiteMap()

	assert.Equal(t, len(results), 2)
	assert.Check(t, results[expectedRouter1].Site.Identity == expectedSite1)
	assert.Check(t, results[expectedRouter2].Site.Identity == expectedSite2)

}

func TestGetSiteById(t *testing.T) {
	skupperStatus := createTestSkupperStatus()
	siteId := "9c88f6dc-ae0e-4dad-956b-def9737b095f"
	expectedNamespace := "public1"

	result := skupperStatus.GetSiteById(siteId)

	assert.Check(t, result.Site.Identity == siteId)
	assert.Check(t, result.Site.Namespace == expectedNamespace)

}

func TestGetSiteLinkMapPerRouter(t *testing.T) {
	skupperStatus := createTestSkupperStatus()

	link := types.LinkInfo{
		Name: "public2-skupper-router-6d5cb849dd-2hrrk",
	}

	router := &types.RouterStatusInfo{

		Router: types.RouterInfo{
			Name:      "public1-skupper-router-5945f87d48-j97sp",
			Namespace: "public1",
		},

		Links: []types.LinkInfo{link},
	}
	site := &types.SiteInfo{
		Identity:  "9c88f6dc-ae0e-4dad-956b-def9737b095f",
		Namespace: "public1",
	}

	expectedLinks := map[string]types.LinkInfo{
		"429c2780-003d-44cc-9a91-4139885c7d20(public2)": {Name: "public2-skupper-router-6d5cb849dd-2hrrk"},
	}
	result := skupperStatus.GetSiteLinkMapPerRouter(router, site)

	assert.DeepEqual(t, result, expectedLinks)

}

func TestLinkBelongsToSameSite(t *testing.T) {
	skupperStatus := createTestSkupperStatus()

	// Add test data to skupperStatus

	linkName := "public2-skupper-router-6d5cb849dd-2hrrk"
	siteId := "429c2780-003d-44cc-9a91-4139885c7d20"
	routerSiteMap := skupperStatus.GetRouterSiteMap()

	result := skupperStatus.LinkBelongsToSameSite(linkName, siteId, routerSiteMap)

	assert.Check(t, result == true)

}

func TestRemoveLinksFromSameSite(t *testing.T) {
	skupperStatus := createTestSkupperStatus()

	link := types.LinkInfo{
		Name: "public2-skupper-router-6d5cb849dd-2hrrk",
	}
	linkToSameSite := types.LinkInfo{
		Name: "public1-skupper-router-5945f87d48-j97sp",
	}
	router := types.RouterStatusInfo{

		Router: types.RouterInfo{
			Name:      "public1-skupper-router-5945f87d48-j97sp",
			Namespace: "public1",
		},

		Links: []types.LinkInfo{link, linkToSameSite},
	}
	site := types.SiteInfo{
		Identity:  "9c88f6dc-ae0e-4dad-956b-def9737b095f",
		Namespace: "public1",
	}

	result := skupperStatus.RemoveLinksFromSameSite(router, site)

	assert.DeepEqual(t, result, []types.LinkInfo{link})

}
