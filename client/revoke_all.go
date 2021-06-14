package client

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
)

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
	siteServerSecret := types.Credential{
		Name:    types.SiteServerSecret,
		Subject: types.TransportServiceName,
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
		err = cli.appendLoadBalancerHostOrIp(types.TransportServiceName, cli.Namespace, &siteServerSecret)
		if err != nil {
			return err
		}
	}
	_, err = kube.RegenerateCredentials(siteServerSecret, cli.Namespace, ca, cli.KubeClient)
	if err != nil {
		return err
	}
	return cli.restartRouter(cli.Namespace)
}
