package client

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
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

func (cli *VanClient) TokenClaimCreate(ctx context.Context, name string, password []byte, expiry time.Duration, uses int, secretFile string) error {
	current, err := cli.getRouterConfig()
	if err != nil {
		return err
	}
	if current.IsEdge() {
		return fmt.Errorf("Edge configuration cannot accept connections")
	}
	service, err := cli.KubeClient.CoreV1().Services(cli.Namespace).Get(types.ControllerServiceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	port := getClaimsPort(service)
	if port == 0 {
		return fmt.Errorf("Site cannot accept connections")
	}
	host := fmt.Sprintf("%s.%s", types.ControllerServiceName, cli.Namespace)
	localOnly := true
	if cli.RouteClient != nil {
		route, err := cli.RouteClient.Routes(cli.Namespace).Get(types.ClaimRedemptionRouteName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		host = route.Spec.Host
		port = 443
		localOnly = false
	} else if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
		host = kube.GetLoadBalancerHostOrIp(service)
		localOnly = false
	}
	recordName, err := uuid.NewUUID()
	if err != nil {
		return err
	}
	protocol := "https"
	url := fmt.Sprintf("%s://%s:%d/%s", protocol, host, port, recordName.String())
	caSecret, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(types.SiteCaSecret, metav1.GetOptions{})
	if err != nil {
		return err
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
			},
		},
		Data: map[string][]byte{
			types.ClaimPasswordDataKey: password,
			types.ClaimCaCertDataKey:   caSecret.Data["tls.crt"],
		},
	}
	record := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: recordName.String(),
			Labels: map[string]string{
				types.SkupperTypeQualifier: types.TypeClaimRecord,
			},
			Annotations: map[string]string{},
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
		return err
	}

	siteId := getSiteId(service)
	if siteId != "" {
		claim.ObjectMeta.Annotations[types.TokenGeneratedBy] = siteId
	}
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	out, err := os.Create(secretFile)
	if err != nil {
		return fmt.Errorf("Could not write to file " + secretFile + ": " + err.Error())
	}
	err = s.Encode(&claim, out)
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
