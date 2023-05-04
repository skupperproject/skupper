package client

import (
	"hash/crc32"
	"sort"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
)

func ContainsAllPolicies(elements []rbacv1.PolicyRule, included []rbacv1.PolicyRule) bool {
	if nil == elements || nil == included {
		return false
	}
	getHashedRules := func(rules []rbacv1.PolicyRule) []uint32 {
		var hashedRules []uint32
		for _, inc := range rules {
			var resources []string
			resources = append(resources, inc.Resources...)
			resources = append(resources, inc.Verbs...)
			resources = append(resources, inc.APIGroups...)
			sort.Strings(resources)
			str := strings.Join(resources, "")
			hashedRules = append(hashedRules, crc32.ChecksumIEEE([]byte(str)))
		}
		return hashedRules
	}
	hashedIncluded := getHashedRules(included)
	hashedElements := getHashedRules(elements)

	for _, el := range hashedElements {
		if !Contains(hashedIncluded, el) {
			return false
		}
	}
	return true
}

func Contains(elements []uint32, element uint32) bool {
	for _, el := range elements {
		if el == element {
			return true
		}
	}
	return false
}
