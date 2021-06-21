package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/spf13/cobra"

	"github.com/skupperproject/skupper/api/types"
)

func NewCmdLink() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link create <input-token-file> [--name <name>] or link delete ...",
		Short: "Manage skupper links definitions",
	}
	return cmd
}

var connectorCreateOpts types.ConnectorCreateOptions

func NewCmdLinkCreate(newClient cobraFunc, flag string) *cobra.Command {

	if flag == "" { //hack for backwards compatibility
		flag = "name"
	}

	cmd := &cobra.Command{
		Use:    "create <input-token-file>",
		Short:  "Links this skupper installation to that which issued the specified token",
		Args:   cobra.ExactArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
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
		},
	}
	cmd.Flags().StringVarP(&connectorCreateOpts.Name, flag, "", "", "Provide a specific name for the link (used when deleting it)")
	cmd.Flags().Int32VarP(&connectorCreateOpts.Cost, "cost", "", 1, "Specify a cost for this link.")

	return cmd
}

var connectorRemoveOpts types.ConnectorRemoveOptions

func NewCmdLinkDelete(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "delete <name>",
		Short:  "Remove specified link",
		Args:   cobra.ExactArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
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
		},
	}

	return cmd
}

var waitFor int

func allConnected(links []types.LinkStatus) bool {
	for _, l := range links {
		if !l.Connected {
			return false
		}
	}
	return true
}

func NewCmdLinkStatus(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status [<link-name>]",
		Short:  "Check whether a link to another Skupper site is active",
		Args:   cobra.MaximumNArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
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
		},
	}
	cmd.Flags().IntVar(&waitFor, "wait", 0, "The number of seconds to wait for links to become active")

	return cmd

}
