package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
)

func (s *SkupperKube) LinkCreate(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	siteConfig, err := cli.SiteConfigInspect(context.Background(), nil)
	if err != nil {
		fmt.Println("Unable to retrieve site config: ", err.Error())
		os.Exit(1)
	}
	connectorCreateOpts.SkupperNamespace = cli.GetNamespace()
	yaml, err := ioutil.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("Could not read connection token: %s", err.Error())
	}
	secret, err := cli.ConnectorCreateSecretFromData(context.Background(), yaml, connectorCreateOpts)
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

func (s *SkupperKube) LinkDelete(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	connectorRemoveOpts.Name = args[0]
	connectorRemoveOpts.SkupperNamespace = cli.GetNamespace()
	connectorRemoveOpts.ForceCurrent = false
	err := cli.ConnectorRemove(context.Background(), connectorRemoveOpts)
	if err == nil {
		fmt.Println("Link '" + args[0] + "' has been removed")
	} else {
		return fmt.Errorf("Failed to remove link: %w", err)
	}
	return nil
}

func (s *SkupperKube) LinkStatus(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)

	if len(args) == 1 && args[0] != "all" {
		for i := 0; ; i++ {
			if i > 0 {
				time.Sleep(time.Second)
			}
			link, err := cli.ConnectorInspect(context.Background(), args[0])
			if errors.IsNotFound(err) {
				fmt.Printf("No such link %q", args[0])
				fmt.Println()
				break
			} else if err != nil {
				fmt.Println(err)
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
			links, err := cli.ConnectorList(context.Background())
			if err != nil {
				fmt.Println(err)
				break
			} else if allConnected(links) || i == waitFor {
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
				break
			}
		}
	}
	return nil
}
