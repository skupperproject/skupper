package securedaccess

import (
	"strings"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
)

type pair struct {
	First  string
	Second string
}

func (p pair) get() (string, string) {
	return p.First, p.Second
}

func (p pair) qualify(namespace string) pair {
	if namespace == "" {
		return p
	}
	return pair{namespace + "/" + p.First, p.Second}
}

func (p pair) complete() bool {
	return p.First != "" && p.Second != ""
}

func splits(s string, separator string) []pair {
	parts := strings.Split(s, separator)
	l := len(parts)
	if l < 2 {
		return []pair{{"", s}}
	}
	if l == 2 {
		return []pair{{parts[0], parts[1]}}
	}
	var results []pair
	for i := 0; i < l-1; i++ {
		results = append(results, pair{strings.Join(parts[0:i+1], separator), strings.Join(parts[i+1:l], separator)})
	}
	return results
}

func possibleKeyPortNamePairs(qualifiedKey string) []pair {
	namespace, name := splits(qualifiedKey, "/")[0].get()
	possibilities := splits(name, "-")
	var results []pair
	for _, possibility := range possibilities {
		if possibility.complete() {
			results = append(results, possibility.qualify(namespace))
		}
	}
	return results
}

func hasPort(sa *skupperv1alpha1.SecuredAccess, portName string) bool {
	for _, port := range sa.Spec.Ports {
		if port.Name == portName {
			return true
		}
	}
	return false
}
