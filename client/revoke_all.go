package client

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
)

func (cli *VanClient) regenerateSiteSecret(ca *corev1.Secret) error {
	siteServerSecret := types.Credential{
		Name:    types.SiteServerSecret,
		Subject: types.TransportServiceName,
		Hosts:   []string{types.TransportServiceName + "." + cli.Namespace},
	}
	usingRoutes := false
	if cli.RouteClient != nil {
		route, err := kube.GetRoute(types.InterRouterRouteName, cli.Namespace, cli.RouteClient)
		if err == nil {
			usingRoutes = true
			siteServerSecret.Hosts = append(siteServerSecret.Hosts, route.Spec.Host)
			route, err = kube.GetRoute(types.EdgeRouteName, cli.Namespace, cli.RouteClient)
			if err != nil {
				return err
			}
			siteServerSecret.Hosts = append(siteServerSecret.Hosts, route.Spec.Host)
		} else if !errors.IsNotFound(err) {
			return err
		}
	}
	if !usingRoutes {
		err := cli.appendLoadBalancerHostOrIp(types.TransportServiceName, cli.Namespace, &siteServerSecret)
		if err != nil {
			return err
		}
		err = cli.appendIngressHost([]string{"inter-router", "edge"}, cli.Namespace, &siteServerSecret)
		if err != nil {
			return err
		}
	}
	_, err := kube.RegenerateCredentials(siteServerSecret, cli.Namespace, ca, cli.KubeClient)
	if err != nil {
		return err
	}
	return cli.restartRouter(cli.Namespace)
}

func (cli *VanClient) regenerateClaimsSecret(ca *corev1.Secret) error {
	claimsServerSecret := types.Credential{
		Name:    types.ClaimsServerSecret,
		Subject: types.ControllerServiceName,
		Hosts:   []string{types.ControllerServiceName + "." + cli.Namespace},
	}
	usingRoutes := false
	if cli.RouteClient != nil {
		route, err := kube.GetRoute(types.ClaimRedemptionRouteName, cli.Namespace, cli.RouteClient)
		if err == nil {
			usingRoutes = true
			claimsServerSecret.Hosts = append(claimsServerSecret.Hosts, route.Spec.Host)
		} else if !errors.IsNotFound(err) {
			return err
		}
	}
	if !usingRoutes {
		err := cli.appendLoadBalancerHostOrIp(types.ControllerServiceName, cli.Namespace, &claimsServerSecret)
		if err != nil {
			return err
		}
		err = cli.appendIngressHost([]string{"claims"}, cli.Namespace, &claimsServerSecret)
		if err != nil {
			return err
		}
	}
	_, err := kube.RegenerateCredentials(claimsServerSecret, cli.Namespace, ca, cli.KubeClient)
	if err != nil {
		return err
	}
	return cli.restartController(cli.Namespace)
}

func (cli *VanClient) RevokeAccess(ctx context.Context) error {
	records, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).List(metav1.ListOptions{LabelSelector: "skupper.io/type=token-claim-record"})
	if err != nil {
		return err
	}
	for _, record := range records.Items {
		err = cli.KubeClient.CoreV1().Secrets(cli.Namespace).Delete(record.Name, nil)
		if err != nil {
			return err
		}
	}

	ca, err := kube.RegenerateCertAuthority(types.SiteCaSecret, cli.Namespace, cli.KubeClient)
	if err != nil {
		return err
	}
	err = cli.regenerateSiteSecret(ca)
	if err != nil {
		return err
	}
	return cli.regenerateClaimsSecret(ca)
}
