package client

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
)

func getSiteId(service *corev1.Service) string {
	for _, ref := range service.ObjectMeta.OwnerReferences {
		if ref.Name == "skupper-site" {
			return string(ref.UID)
		}
	}
	return ""
}

func getClaimsPort(service *corev1.Service) int32 {
	for _, port := range service.Spec.Ports {
		if port.Name == types.ClaimRedemptionPortName {
			return port.Port
		}
	}
	return 0
}

func getClaimsNodePort(service *corev1.Service) (int32, error) {
	for _, port := range service.Spec.Ports {
		if port.Name == types.ClaimRedemptionPortName {
			return port.NodePort, nil
		}
	}
	return 0, fmt.Errorf("NodePort for claims not found.")
}

func (cli *VanClient) getControllerIngressHost() (string, error) {
	config, err := cli.SiteConfigInspect(context.TODO(), nil)
	if err != nil {
		return "", err
	}
	if host := config.Spec.GetControllerIngressHost(); host != "" {
		return host, nil
	}
	return "", fmt.Errorf("Controller ingress host not defined, cannot use claims for nodeport without it. A certificate token can be generated directly with --token-type=cert.")
}

func (cli *VanClient) TokenClaimCreateFile(ctx context.Context, name string, password []byte, expiry time.Duration, uses int, secretFile string) error {
	claim, localOnly, err := cli.TokenClaimCreate(ctx, name, password, expiry, uses)
	if err != nil {
		return err
	}
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	out, err := os.Create(secretFile)
	if err != nil {
		return fmt.Errorf("Could not write to file " + secretFile + ": " + err.Error())
	}
	err = s.Encode(claim, out)
	if err != nil {
		return fmt.Errorf("Could not write out generated secret: " + err.Error())
	} else {
		var extra string
		if localOnly {
			extra = "(Note: token will only be valid for local cluster)"
		}
		fmt.Printf("Token written to %s %s", secretFile, extra)
		fmt.Println()
		return nil
	}
}
func getContourProxyClaimsHostSuffix(cli *VanClient) string {
	config, err := cli.SiteConfigInspect(context.TODO(), nil)
	if err != nil {
		fmt.Printf("Failed to look up site config: %s, ", err)
		fmt.Println()
		return ""
	}
	if config != nil && config.Spec.IsIngressContourHttpProxy() {
		return config.Spec.GetControllerIngressHost()
	}
	return ""
}

func (cli *VanClient) TokenClaimTemplateCreate(ctx context.Context, name string, password []byte, recordName string) (*corev1.Secret, *corev1.Service, bool, error) {
	current, err := cli.getRouterConfig()
	if err != nil {
		return nil, nil, false, err
	}
	if current.IsEdge() {
		return nil, nil, false, fmt.Errorf("Edge configuration cannot accept connections")
	}
	service, err := cli.KubeClient.CoreV1().Services(cli.Namespace).Get(types.ControllerServiceName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, false, err
	}
	port := getClaimsPort(service)
	if port == 0 {
		return nil, nil, false, fmt.Errorf("Site cannot accept connections")
	}
	host := fmt.Sprintf("%s.%s", types.ControllerServiceName, cli.Namespace)
	localOnly := true
	ok, err := configureClaimHostFromRoutes(&host, cli)
	if err != nil {
		return nil, nil, false, err
	} else if ok {
		// host configured from route
		port = 443
		localOnly = false
	} else if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
		host = kube.GetLoadBalancerHostOrIp(service)
		localOnly = false
	} else if service.Spec.Type == corev1.ServiceTypeNodePort {
		host, err = cli.getControllerIngressHost()
		if err != nil {
			return nil, nil, false, err
		}
		port, err = getClaimsNodePort(service)
		if err != nil {
			return nil, nil, false, err
		}
		localOnly = false
	} else if suffix := getContourProxyClaimsHostSuffix(cli); suffix != "" {
		host = strings.Join([]string{types.ClaimsIngressPrefix, cli.Namespace, suffix}, ".")
		port = 443
		localOnly = false
	} else {
		ingressRoutes, err := kube.GetIngressRoutes(types.IngressName, cli.Namespace, cli.KubeClient)
		if err != nil {
			return nil, nil, false, err
		}
		if len(ingressRoutes) > 0 {
			for _, route := range ingressRoutes {
				if route.ServicePort == int(types.ClaimRedemptionPort) {
					host = route.Host
					port = 443
					localOnly = false
					break
				}
			}
		}
	}
	protocol := "https"
	url := fmt.Sprintf("%s://%s:%d/%s", protocol, host, port, recordName)
	caSecret, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(types.SiteCaSecret, metav1.GetOptions{})
	if err != nil {
		return nil, nil, false, err
	}
	claim := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				types.SkupperTypeQualifier: types.TypeClaimRequest,
			},
			Annotations: map[string]string{
				types.ClaimUrlAnnotationKey: url,
				types.SiteVersion:           current.GetSiteMetadata().Version,
			},
		},
		Data: map[string][]byte{
			types.ClaimPasswordDataKey: password,
			types.ClaimCaCertDataKey:   caSecret.Data["tls.crt"],
		},
	}
	siteId := getSiteId(service)
	if siteId != "" {
		claim.ObjectMeta.Annotations[types.TokenGeneratedBy] = siteId
	}
	return &claim, service, localOnly, nil
}

func (cli *VanClient) TokenClaimCreate(ctx context.Context, name string, password []byte, expiry time.Duration, uses int) (*corev1.Secret, bool, error) {
	if name == "" {
		id, err := uuid.NewUUID()
		if err != nil {
			return nil, false, err
		}
		name = id.String()
	}
	claim, service, localOnly, err := cli.TokenClaimTemplateCreate(ctx, name, password, name)
	if err != nil {
		return nil, false, err
	}
	siteMetadata, err := cli.GetSiteMetadata()
	if err != nil {
		return nil, false, err
	}
	record := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				types.SkupperTypeQualifier: types.TypeClaimRecord,
			},
			Annotations: map[string]string{
				types.SiteVersion: siteMetadata.Version,
			},
		},
		Data: map[string][]byte{
			types.ClaimPasswordDataKey: password,
		},
	}
	record.ObjectMeta.OwnerReferences = service.ObjectMeta.OwnerReferences
	if expiry > 0 {
		expiration := time.Now().Add(expiry)
		record.ObjectMeta.Annotations[types.ClaimExpiration] = expiration.Format(time.RFC3339)
	}
	if uses > 0 {
		record.ObjectMeta.Annotations[types.ClaimsRemaining] = strconv.Itoa(uses)
	}
	_, err = cli.KubeClient.CoreV1().Secrets(cli.Namespace).Create(&record)
	if err != nil {
		return nil, false, err
	}

	return claim, localOnly, nil
}

func configureClaimHostFromRoutes(host *string, cli *VanClient) (bool, error) {
	if cli.RouteClient == nil {
		return false, nil
	}
	route, err := cli.RouteClient.Routes(cli.Namespace).Get(types.ClaimRedemptionRouteName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	*host = route.Spec.Host
	return true, nil
}
