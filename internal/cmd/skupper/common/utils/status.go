package utils

import (
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

func SiteReady(siteList *v2alpha1.SiteList) (bool, string) {
	for _, s := range siteList.Items {
		if s.IsReady() {
			return true, s.Name
		}
	}
	return false, ""
}

func SiteLinkAccessEndpoints(siteList *v2alpha1.SiteList) (bool, string) {
	for _, s := range siteList.Items {
		if len(s.Status.Endpoints) > 0 {
			return true, s.Name
		}
	}
	return false, ""
}
