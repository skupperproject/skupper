package main

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/utils"
	"os"
	"strconv"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
)

type SkupperKubeLink struct {
	kube *SkupperKube
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
	silenceCobra(cmd)

	if remoteInfoTimeout.Seconds() <= 0 {
		return fmt.Errorf(`invalid timeout value`)
	}

	if verboseLinkStatus && (len(args) == 0 || args[0] == "all") {
		fmt.Println("In order to provide detailed information about the link, specify the link name")
		return nil
	}

	siteConfig, err := s.kube.Cli.SiteConfigInspect(context.Background(), nil)

	if err != nil {
		fmt.Println("Site configuration is not currently available")
	}

	if len(args) == 1 && args[0] != "all" {
		for i := 0; ; i++ {
			if i > 0 {
				time.Sleep(time.Second)
			}
			link, err := s.kube.Cli.ConnectorInspect(context.Background(), args[0])
			if errors.IsNotFound(err) {
				fmt.Printf("No such link %q", args[0])
				fmt.Println()
				break
			} else if err != nil {
				fmt.Println(err)
				break
			} else if verboseLinkStatus {
				detailMap := createLinkDetailMap(link, siteConfig)
				err = client.PrintKeyValueMap(detailMap)
				if err != nil {
					fmt.Println(err)
				}
				fmt.Println()
				break

			} else if link.Connected {
				fmt.Printf("Link %s is active", link.Name)
				fmt.Println()
				break
			} else if i == waitFor {
				if link.Description != "" {
					fmt.Printf("Link %s not active (%s)", link.Name, link.Description)
				} else {
					fmt.Printf("Link %s not active", link.Name)
				}
				fmt.Println()
				break
			}
		}
	} else {
		for i := 0; ; i++ {
			if i > 0 {
				time.Sleep(time.Second)
			}
			links, err := s.kube.Cli.ConnectorList(context.Background())
			if err != nil {
				fmt.Println(err)
				break
			} else if allConnected(links) || i == waitFor {
				fmt.Println("\nLinks created from this site:")
				fmt.Println("-------------------------------")

				if len(links) == 0 {
					fmt.Println("There are no links configured or active")
				}
				for _, link := range links {
					if link.Connected {
						fmt.Printf("Link %s is active", link.Name)
						fmt.Println()
					} else {
						if link.Description != "" {
							fmt.Printf("Link %s not active (%s)", link.Name, link.Description)
						} else {
							fmt.Printf("Link %s not active", link.Name)
						}
						fmt.Println()
					}
				}

				ctx, cancel := context.WithTimeout(context.Background(), remoteInfoTimeout)
				defer cancel()

				fmt.Println("\nCurrently active links from other sites:")
				fmt.Println("----------------------------------------")

				var remoteLinks []*types.RemoteLinkInfo
				err := utils.RetryErrorWithContext(ctx, time.Second, func() error {
					remoteLinks, err = s.kube.Cli.GetRemoteLinks(ctx, siteConfig)
					if err != nil {
						return err
					}
					return nil
				})

				if err != nil {
					fmt.Println(err)
					break
				} else if len(remoteLinks) > 0 {
					for _, remoteLink := range remoteLinks {
						fmt.Printf("A link from the namespace %s on site %s(%s) is active ", remoteLink.Namespace, remoteLink.SiteName, remoteLink.SiteId)
						fmt.Println()
					}
				} else {
					fmt.Println("There are no active links")
				}
				break
			}
		}
	}
	return nil
}

func (s *SkupperKubeLink) StatusFlags(cmd *cobra.Command) {}

func createLinkDetailMap(link *types.LinkStatus, siteConfig *types.SiteConfig) map[string]string {

	status := "Active"

	if !link.Connected {
		status = "Not active"

		if len(link.Description) > 0 {
			status = fmt.Sprintf("%s (%s)", status, link.Description)
		}
	}

	return map[string]string{
		"Name:":      link.Name,
		"Status:":    status,
		"Namespace:": siteConfig.Spec.SkupperNamespace,
		"Site:":      siteConfig.Spec.SkupperName + "-" + siteConfig.Reference.UID,
		"Cost:":      strconv.Itoa(link.Cost),
		"Created:":   link.Created,
	}
}
