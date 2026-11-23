package securedaccess

import (
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

func hasPort(sa *skupperv2alpha1.SecuredAccess, portName string) bool {
	for _, port := range sa.Spec.Ports {
		if port.Name == portName {
			return true
		}
	}
	return false
}
