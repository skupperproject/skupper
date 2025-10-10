package certificates

import (
	"regexp"
	"slices"
	"strings"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	exprAnnotationKeyCertHosts = regexp.MustCompile(`^internal\.skupper\.io/hosts-(?P<refid>.+)$`)
)

const (
	annotationKeyPrefixCertHosts   = "internal.skupper.io/hosts-"
	annotationKeySkupperControlled = "internal.skupper.io/controlled"
)

func certificateHostsAnnotationKey(refid string) string {
	return annotationKeyPrefixCertHosts + refid
}

type ownerMapping struct {
	// IsControlled by the skupper controller
	IsControlled bool
	// PerOwnerHosts is a collection of owner references and associated hosts
	PerOwnerHosts map[string][]string
}

func newOwnerMapping(refs []metav1.OwnerReference, hosts []string) ownerMapping {
	mapping := ownerMapping{
		IsControlled:  true,
		PerOwnerHosts: make(map[string][]string),
	}
	for _, ref := range refs {
		mapping.PerOwnerHosts[string(ref.UID)] = hosts
	}
	return mapping
}

func certificateToOwnerMapping(certificate *skupperv2alpha1.Certificate) ownerMapping {
	result := ownerMapping{
		PerOwnerHosts: make(map[string][]string),
	}
	if certificate == nil || certificate.ObjectMeta.Annotations == nil {
		return result
	}
	if _, ok := certificate.ObjectMeta.Annotations[annotationKeySkupperControlled]; ok {
		result.IsControlled = true
	}
	result.PerOwnerHosts = parseHostsAnnotations(certificate.ObjectMeta.Annotations)
	return result
}

// ApplyMetadata Certificate. Returns true when a change was applied
func (m ownerMapping) ApplyMetadata(certificate *skupperv2alpha1.Certificate) bool {
	if certificate == nil {
		return false
	}
	changed := false
	if m.IsControlled {
		setAnnotation(&certificate.ObjectMeta, annotationKeySkupperControlled, "true")
	} else {
		clearAnnotation(&certificate.ObjectMeta, annotationKeySkupperControlled)
	}
	// clear host annotations not in desired
	for refid := range parseHostsAnnotations(certificate.ObjectMeta.Annotations) {
		if m.PerOwnerHosts != nil {
			if _, ok := m.PerOwnerHosts[refid]; ok {
				continue
			}
		}
		changed = true
		clearAnnotation(&certificate.ObjectMeta, certificateHostsAnnotationKey(refid))
	}
	// update host annotations
	for refid, hosts := range m.PerOwnerHosts {
		slices.Sort(hosts)
		if setAnnotation(&certificate.ObjectMeta, certificateHostsAnnotationKey(refid), strings.Join(hosts, ",")) {
			changed = true
		}
	}
	return changed
}

// CombinedHosts returns the complete set of unique hosts from all owners
func (m ownerMapping) CombinedHosts() []string {
	var allHosts []string
	for _, hosts := range m.PerOwnerHosts {
		allHosts = append(allHosts, hosts...)
	}
	slices.Sort(allHosts)
	return slices.Compact(allHosts)
}

func parseHostsAnnotations(annotations map[string]string) map[string][]string {
	hosts := make(map[string][]string)
	for key, val := range annotations {
		if !exprAnnotationKeyCertHosts.MatchString(key) {
			continue
		}
		keyMatch := exprAnnotationKeyCertHosts.FindStringSubmatch(key)
		refID := keyMatch[exprAnnotationKeyCertHosts.SubexpIndex("refid")]
		hosts[refID] = append(hosts[refID], strings.Split(val, ",")...)
		slices.Sort(hosts[refID])
	}
	return hosts
}

func setAnnotation(meta *metav1.ObjectMeta, key, val string) bool {
	if meta == nil {
		return false
	}
	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}
	prev, ok := meta.Annotations[key]
	if !ok || prev != val {
		meta.Annotations[key] = val
		return true
	}
	return false
}
func clearAnnotation(meta *metav1.ObjectMeta, key string) bool {
	if meta == nil || meta.Annotations == nil {
		return false
	}
	_, found := meta.Annotations[key]
	delete(meta.Annotations, key)
	return found
}
