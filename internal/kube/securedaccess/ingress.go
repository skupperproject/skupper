package securedaccess

import (
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	"strings"
)

func ingressClassNameForDesiredIngress(nginx bool, defaultClass string, settings map[string]string) string {
	if v := strings.TrimSpace(settings[SettingIngressClassName]); v != "" {
		return v
	}
	if v := strings.TrimSpace(defaultClass); v != "" {
		return v
	}
	if nginx {
		return "nginx"
	}
	return ""
}

func toIngress(domain string, access *skupperv2alpha1.SecuredAccess) *networkingv1.Ingress {
	ingress := &networkingv1.Ingress{ObjectMeta: exposureMetadata(access, access.Name)}
	pathType := networkingv1.PathTypePrefix
	for _, port := range access.Spec.Ports {
		ingress.Spec.Rules = append(ingress.Spec.Rules, networkingv1.IngressRule{Host: qualify(port.Name, domain), IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{{Path: "/", PathType: &pathType, Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: access.Name, Port: networkingv1.ServiceBackendPort{Number: int32(port.Port)}}}}}}}})
	}
	return ingress
}

func addNginxIngressAnnotations(annotations map[string]string) {
	annotations["nginx.ingress.kubernetes.io/ssl-passthrough"] = "true"
	annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "true"
}

func deduceDomainForIngressHosts(ingress *networkingv1.Ingress) string {
	if len(ingress.Status.LoadBalancer.Ingress) == 0 {
		return ""
	}
	hostOrIP := ingress.Status.LoadBalancer.Ingress[0]
	if hostOrIP.Hostname != "" {
		return hostOrIP.Hostname
	}
	if hostOrIP.IP != "" {
		return hostOrIP.IP + ".nip.io"
	}
	return ""
}

func prefix(hostname string) string { return strings.Split(hostname, ".")[0] }
func qualify(hostname, domain string) string {
	if domain == "" {
		return hostname
	}
	return hostname + "." + domain
}
