package client

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
)

func (cli *VanClient) isNodePortService(serviceName string) bool {
	service, err := kube.GetService(serviceName, cli.Namespace, cli.KubeClient)
	if err != nil {
		return false
	}
	return service.Spec.Type == corev1.ServiceTypeNodePort
}

func (cli *VanClient) appendRouterIngressHost(cred *types.Credential) bool {
	config, err := cli.SiteConfigInspect(context.TODO(), nil)
	if err == nil {
		host := config.Spec.GetRouterIngressHost()
		if host != "" {
			cred.Hosts = append(cred.Hosts, host)
			return true
		}
	}
	return false
}

func (cli *VanClient) appendControllerIngressHost(cred *types.Credential) bool {
	config, err := cli.SiteConfigInspect(context.TODO(), nil)
	if err == nil {
		host := config.Spec.GetControllerIngressHost()
		if host != "" {
			cred.Hosts = append(cred.Hosts, host)
			return true
		}
	}
	return false
}

func (cli *VanClient) regenerateSiteSecret(ctx context.Context, ca *corev1.Secret) error {
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
		err := cli.appendLoadBalancerHostOrIp(ctx, types.TransportServiceName, cli.Namespace, &siteServerSecret)
		if err != nil {
			return err
		}
		err = cli.appendIngressHost([]string{"inter-router", "edge"}, cli.Namespace, &siteServerSecret)
		if err != nil {
			return err
		}
		if cli.isNodePortService(types.TransportServiceName) {
			cli.appendRouterIngressHost(&siteServerSecret)
		}
	}
	_, err := kube.RegenerateCredentials(siteServerSecret, cli.Namespace, ca, cli.KubeClient)
	if err != nil {
		return err
	}
	return cli.restartRouter(cli.Namespace)
}

func (cli *VanClient) regenerateClaimsSecret(ctx context.Context, ca *corev1.Secret) error {
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
		err := cli.appendLoadBalancerHostOrIp(ctx, types.ControllerServiceName, cli.Namespace, &claimsServerSecret)
		if err != nil {
			return err
		}
		err = cli.appendIngressHost([]string{"claims"}, cli.Namespace, &claimsServerSecret)
		if err != nil {
			return err
		}
		if cli.isNodePortService(types.ControllerServiceName) {
			cli.appendControllerIngressHost(&claimsServerSecret)
		}
	}
	_, err := kube.RegenerateCredentials(claimsServerSecret, cli.Namespace, ca, cli.KubeClient)
	if err != nil {
		return err
	}
	return cli.restartController(cli.Namespace)
}

func (cli *VanClient) RevokeAccess(ctx context.Context) error {
	records, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).List(ctx, metav1.ListOptions{LabelSelector: "skupper.io/type=token-claim-record"})
	if err != nil {
		return err
	}
	for _, record := range records.Items {
		err = cli.KubeClient.CoreV1().Secrets(cli.Namespace).Delete(ctx, record.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	ca, err := kube.RegenerateCertAuthority(types.SiteCaSecret, cli.Namespace, cli.KubeClient)
	if err != nil {
		return err
	}
	err = cli.regenerateSiteSecret(ctx, ca)
	if err != nil {
		return err
	}
	return cli.regenerateClaimsSecret(ctx, ca)
}
