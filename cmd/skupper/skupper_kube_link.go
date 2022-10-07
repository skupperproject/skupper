package main

import (
	"context"
	"fmt"
	"os"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/domain/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SkupperKubeLink struct {
	kube        *SkupperKube
	linkHandler *kube.LinkHandlerKube
}

func (s *SkupperKubeLink) NewClient(cmd *cobra.Command, args []string) {
	s.kube.NewClient(cmd, args)
}

func (s *SkupperKubeLink) Platform() types.Platform {
	return s.Platform()
}

func (s *SkupperKubeLink) Create(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	cli := s.kube.Cli
	siteConfig, err := cli.SiteConfigInspect(context.Background(), nil)
	if err != nil {
		fmt.Println("Unable to retrieve site config: ", err.Error())
		os.Exit(1)
	}
	connectorCreateOpts.SkupperNamespace = cli.GetNamespace()
	secret, err := cli.ConnectorCreateSecretFromData(context.Background(), connectorCreateOpts)
	if err != nil {
		return fmt.Errorf("Failed to create link: %w", err)
	} else {
		if secret.ObjectMeta.Labels[types.SkupperTypeQualifier] == types.TypeToken {
			if siteConfig.Spec.RouterMode == string(types.TransportModeEdge) {
				fmt.Printf("Site configured to link to %s:%s (name=%s)\n",
					secret.ObjectMeta.Annotations["edge-host"],
					secret.ObjectMeta.Annotations["edge-port"],
					secret.ObjectMeta.Name)
			} else {
				fmt.Printf("Site configured to link to %s:%s (name=%s)\n",
					secret.ObjectMeta.Annotations["inter-router-host"],
					secret.ObjectMeta.Annotations["inter-router-port"],
					secret.ObjectMeta.Name)
			}
		} else {
			fmt.Printf("Site configured to link to %s (name=%s)\n",
				secret.ObjectMeta.Annotations[types.ClaimUrlAnnotationKey],
				secret.ObjectMeta.Name)
		}
	}
	fmt.Println("Check the status of the link using 'skupper link status'.")
	return nil
}

func (s *SkupperKubeLink) CreateFlags(cmd *cobra.Command) {
	// TODO implement me
	panic("implement me")
}

func (s *SkupperKubeLink) Delete(cmd *cobra.Command, args []string) error {
	cli := s.kube.Cli
	connectorRemoveOpts.SkupperNamespace = cli.GetNamespace()
	connectorRemoveOpts.ForceCurrent = false
	return cli.ConnectorRemove(context.Background(), connectorRemoveOpts)
}

func (s *SkupperKubeLink) DeleteFlags(cmd *cobra.Command) {}

func (s *SkupperKubeLink) List(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	cli := s.kube.Cli
	connectors, err := cli.ConnectorList(context.Background())
	if err == nil {
		if len(connectors) == 0 {
			fmt.Println("There are no connectors defined.")
		} else {
			fmt.Println("Connectors:")
			for _, c := range connectors {
				fmt.Printf("    %s (name=%s)", c.Url, c.Name)
				fmt.Println()
			}
		}
	} else if errors.IsNotFound(err) {
		return SkupperNotInstalledError(cli.GetNamespace())
	} else {
		return fmt.Errorf("Unable to retrieve connections: %w", err)
	}
	return nil
}

func (s *SkupperKubeLink) ListFlags(cmd *cobra.Command) {}

func (s *SkupperKubeLink) Status(cmd *cobra.Command, args []string) error {
	return nil
}

func (s *SkupperKubeLink) StatusFlags(cmd *cobra.Command) {}

func (s *SkupperKubeLink) LinkHandler() domain.LinkHandler {
	if s.linkHandler != nil {
		return s.linkHandler
	}
	site, err := s.kube.Cli.SiteConfigInspect(context.Background(), nil)
	if err != nil {
		return nil
	}
	cli := s.kube.Cli.(*client.VanClient)
	cm, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get(types.TransportConfigMapName, v1.GetOptions{})
	if err != nil {
		return nil
	}
	router, err := qdr.GetRouterConfigFromConfigMap(cm)
	if err != nil {
		return nil
	}
	s.linkHandler = kube.NewLinkHandlerKube(cli.Namespace, site, router, cli.KubeClient, cli.RestConfig)
	return s.linkHandler
}
