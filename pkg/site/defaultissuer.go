package site

import (
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
)

func DefaultIssuer(site *skupperv1alpha1.Site) string {
	if site != nil {
		if site.Spec.DefaultIssuer != "" {
			return site.Spec.DefaultIssuer
		}
		if site.Status.DefaultIssuer != "" {
			return site.Status.DefaultIssuer
		}
	}
	return "skupper-site-ca"
}
