package utils

import (
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
)

func SiteConfigured(siteList *v1alpha1.SiteList) bool {
	for _, s := range siteList.Items {
		if s.IsActive() {
			return true
		}
	}
	return false
}
