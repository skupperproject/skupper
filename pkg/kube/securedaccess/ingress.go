package securedaccess

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
)

type IngressAccessType struct {
	manager *SecuredAccessManager
	nginx   bool // if true add nginx class and nginx specific annotations
	domain  string
}

func newIngressAccess(manager *SecuredAccessManager, nginx bool, domain string) AccessType {
	return &IngressAccessType{
		manager: manager,
		nginx:   nginx,
		domain:  domain,
	}
}

func (o *IngressAccessType) RealiseAndResolve(access *skupperv1alpha1.SecuredAccess, svc *corev1.Service) ([]skupperv1alpha1.Endpoint, error) {
	desired := toIngress(qualify(access.Namespace, o.domain), access)
	if o.nginx {
		className := "nginx"
		desired.Spec.IngressClassName = &className
		addNginxIngressAnnotations(desired.ObjectMeta.Annotations)
	}
	ingress, qualified, err := o.ensureIngress(access.Namespace, desired)
	if err != nil {
		return nil, err
	}
	if !qualified {
		return nil, nil
	}

	var endpoints []skupperv1alpha1.Endpoint
	for _, rule := range ingress.Spec.Rules {
		endpoints = append(endpoints, skupperv1alpha1.Endpoint{
			Name: prefix(rule.Host),
			Host: rule.Host,
			Port: "443",
		})
	}
	return endpoints, nil
}

func (o *IngressAccessType) ensureIngress(namespace string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, bool, error) {
	key := fmt.Sprintf("%s/%s", namespace, ingress.Name)
	domain := o.domain
	if existing, ok := o.manager.ingresses[key]; ok {
		if domain == "" {
			domain = deduceDomainForIngressHosts(existing)
			if domain == "" {
				log.Printf("No domain can be inferred yet for ingress %s/%s", namespace, ingress.Name)
			} else if qualifyIngressHosts(domain, ingress) {
				log.Printf("Updated hosts for ingress %s/%s by appending domain %s", namespace, ingress.Name, domain)
			}
		}
		if equivalentIngress(existing, ingress) {
			log.Printf("No change to ingress %s/%s is required", namespace, ingress.Name)
			return existing, domain != "", nil
		}
		existing.Spec = ingress.Spec
		updated, err := o.manager.clients.GetKubeClient().NetworkingV1().Ingresses(namespace).Update(context.Background(), existing, metav1.UpdateOptions{})
		if err != nil {
			log.Printf("Error on update for ingress %s/%s: %s", namespace, ingress.Name, err)
			return existing, false, err
		}
		log.Printf("Ingress %s/%s updated successfully", namespace, ingress.Name)
		o.manager.ingresses[key] = updated
		return updated, domain != "", nil
	}
	created, err := o.manager.clients.GetKubeClient().NetworkingV1().Ingresses(namespace).Create(context.Background(), ingress, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Error on create for ingress %s/%s: %s", namespace, ingress.Name, err)
		return nil, false, err
	}
	log.Printf("Ingress %s/%s created successfully", namespace, ingress.Name)
	o.manager.ingresses[key] = created
	return created, domain != "", nil
}

func toIngress(domain string, access *skupperv1alpha1.SecuredAccess) *networkingv1.Ingress {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: access.Name,
			Labels: map[string]string{
				"internal.skupper.io/secured-access": "true",
			},
			Annotations: map[string]string{
				"internal.skupper.io/controlled": "true",
			},
			OwnerReferences: ownerReferences(access),
		},
	}
	pathType := networkingv1.PathTypePrefix
	for _, port := range access.Spec.Ports {
		host := port.Name
		if domain != "" {
			host = host + "." + domain
		}
		ingress.Spec.Rules = append(ingress.Spec.Rules, networkingv1.IngressRule{
			Host: host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: access.Name,
									Port: networkingv1.ServiceBackendPort{
										Number: int32(port.Port),
									},
								},
							},
						},
					},
				},
			},
		})
	}
	return ingress
}

func equivalentIngress(actual *networkingv1.Ingress, desired *networkingv1.Ingress) bool {
	return reflect.DeepEqual(actual.Spec, desired.Spec)
}

func addNginxIngressAnnotations(annotations map[string]string) {
	annotations["nginx.ingress.kubernetes.io/ssl-passthrough"] = "true"
	annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "true"
}

func deduceDomainForIngressHosts(ingress *networkingv1.Ingress) string {
	if len(ingress.Status.LoadBalancer.Ingress) == 0 {
		return ""
	}
	hostOrIp := ingress.Status.LoadBalancer.Ingress[0]
	if hostOrIp.Hostname != "" {
		return hostOrIp.Hostname
	} else if hostOrIp.IP != "" {
		return hostOrIp.IP + ".nip.io"
	} else {
		return ""
	}
}

func qualifyIngressHosts(domain string, ingress *networkingv1.Ingress) bool {
	changed := false
	for i, rule := range ingress.Spec.Rules {
		if !strings.HasSuffix(rule.Host, domain) {
			ingress.Spec.Rules[i].Host = qualify(rule.Host, domain)
			changed = true
		}
	}
	return changed
}

func prefix(hostname string) string {
	return strings.Split(hostname, ".")[0]
}

func qualify(hostname, domain string) string {
	if domain == "" {
		return hostname
	}
	return hostname + "." + domain
}
