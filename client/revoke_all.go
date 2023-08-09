package client

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/kube/resolver"
)

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

	siteconfig, err := cli.SiteConfigInspect(ctx, nil)
	if err != nil {
		return err
	}
	rslvr, err := resolver.NewResolver(cli, cli.Namespace, &siteconfig.Spec)
	if err != nil {
		return err
	}
	hosts, err := rslvr.GetAllHosts()
	if err != nil {
		return err
	}
	siteServerSecret.Hosts = append(siteServerSecret.Hosts, hosts...)
	_, err = kube.RegenerateCredentials(siteServerSecret, cli.Namespace, ca, cli.KubeClient)
	if err != nil {
		return err
	}
	return cli.restartRouter(cli.Namespace)
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
	return cli.regenerateSiteSecret(ctx, ca)
}
