package client

import (
	"hash/crc32"
	"sort"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
)

func splitWithEscaping(s string, separator, escape byte) []string {
	var token []byte
	var tokens []string
	for i := 0; i < len(s); i++ {
		if s[i] == separator {
			tokens = append(tokens, strings.TrimSpace(string(token)))
			token = token[:0]
		} else if s[i] == escape && i+1 < len(s) {
			i++
			token = append(token, s[i])
		} else {
			token = append(token, s[i])
		}
	}
	tokens = append(tokens, strings.TrimSpace(string(token)))
	return tokens
}

func asMap(entries []string) map[string]string {
	result := map[string]string{}
	for _, entry := range entries {
		parts := strings.Split(entry, "=")
		if len(parts) > 1 {
			result[parts[0]] = parts[1]
		} else {
			result[parts[0]] = ""
		}
	}
	return result
}

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
