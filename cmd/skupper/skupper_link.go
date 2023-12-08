package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/skupperproject/skupper/pkg/utils/formatter"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
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

func NewCmdLinkCreate(skupperClient SkupperLinkClient, flag string) *cobra.Command {

	if flag == "" { // hack for backwards compatibility
		flag = "name"
	}

	cmd := &cobra.Command{
		Use:    "create <input-token-file>",
		Short:  "Links this skupper site to the site that issued the token",
		Args:   cobra.ExactArgs(1),
		PreRun: skupperClient.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			// loading secret from file
			yaml, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("Could not read connection token: %s", err.Error())
			}
			costFlag := cmd.Flag("cost")
			ys := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme,
				scheme.Scheme)
			var secret = &corev1.Secret{}
			_, _, err = ys.Decode(yaml, nil, secret)
			if err != nil {
				return fmt.Errorf("Could not parse connection token: %w", err)
			}
			connectorCreateOpts.Secret = secret
			if secret.ObjectMeta.Annotations != nil && !costFlag.Changed {
				if costStr, ok := secret.ObjectMeta.Annotations[types.TokenCost]; ok {
					if cost, err := strconv.Atoi(costStr); err == nil {
						connectorCreateOpts.Cost = int32(cost)
					}
				}
			}

			return skupperClient.Create(cmd, args)
		},
	}
	cmd.Flags().StringVarP(&connectorCreateOpts.Name, flag, "", "", "Provide a specific name for the link (used when deleting it)")
	cmd.Flags().Int32VarP(&connectorCreateOpts.Cost, "cost", "", 1, "Specify a cost for this link.")

	return cmd
}

var connectorRemoveOpts types.ConnectorRemoveOptions

func NewCmdLinkDelete(skupperClient SkupperLinkClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "delete <name>",
		Short:  "Remove specified link",
		Args:   cobra.ExactArgs(1),
		PreRun: skupperClient.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			connectorRemoveOpts.Name = args[0]
			err := skupperClient.Delete(cmd, args)
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
var verboseLinkStatus bool

func allConnected(links []types.LinkStatus) bool {
	for _, l := range links {
		if !l.Connected {
			return false
		}
	}
	return true
}

func NewCmdLinkStatus(skupperClient SkupperLinkClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status [<link-name>]",
		Short:  "Check whether a link to another Skupper site is connected",
		Args:   cobra.MaximumNArgs(1),
		PreRun: skupperClient.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			linkHandler := skupperClient.LinkHandler()
			if linkHandler == nil {
				return fmt.Errorf("unable to retrieve links")
			}

			if verboseLinkStatus && (len(args) == 0 || args[0] == "all") {
				fmt.Println("In order to provide detailed information about the link, specify the link name")
				return nil
			}

			if len(args) == 1 && args[0] != "all" {
				for i := 0; ; i++ {
					if i > 0 {
						time.Sleep(time.Second)
					}
					link, err := linkHandler.Status(args[0])
					if errors.IsNotFound(err) {
						fmt.Printf("No such link %q", args[0])
						fmt.Println()
						break
					} else if err != nil {
						fmt.Println(err)
						break
					} else if verboseLinkStatus {
						detailMap, err := linkHandler.Detail(link)
						err = formatter.PrintKeyValueMap(detailMap)
						if err != nil {
							fmt.Println(err)
						}
						fmt.Println()
						break

					} else if link.Connected {
						fmt.Printf("Link %s is connected", link.Name)
						fmt.Println()
						break
					} else if i == waitFor {
						if link.Description != "" {
							fmt.Printf("Link %s not connected (%s)", link.Name, link.Description)
						} else {
							fmt.Printf("Link %s not connected", link.Name)
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
					links, err := linkHandler.StatusAll()
					if err != nil {
						fmt.Println(err)
						break
					} else if allConnected(links) || i == waitFor {
						fmt.Println("\nLinks created from this site:")
						fmt.Println()

						if len(links) == 0 {
							fmt.Println("\t There are no links configured or connected")
						}
						for _, link := range links {
							if link.Connected {
								fmt.Printf("\t Link %s is connected", link.Name)
								fmt.Println()
							} else {
								if link.Description != "" {
									fmt.Printf("\t Link %s not connected (%s)", link.Name, link.Description)
								} else {
									fmt.Printf("\t Link %s not connected", link.Name)
								}
								fmt.Println()
							}
						}

						ctx, cancel := context.WithTimeout(context.Background(), types.DefaultTimeoutDuration)
						defer cancel()

						fmt.Println("\nCurrent links from other sites that are connected:")
						fmt.Println()

						remoteLinks, err := linkHandler.RemoteLinks(ctx)
						if err != nil {
							return err
						}

						if len(remoteLinks) > 0 {
							for _, remoteLink := range remoteLinks {
								//todo: add the link name (connector name) when ready in the configmap
								remoteNamespace := ""
								if len(remoteLink.Namespace) > 0 {
									remoteNamespace = fmt.Sprintf("on namespace %s", remoteLink.Namespace)

								}
								fmt.Printf("\t Incoming link from site %s %s", remoteLink.SiteId, remoteNamespace)
								fmt.Println()
							}
						} else {
							fmt.Println("\t There are no connected links")
						}
					}
					break
				}
			}

			return nil
		},
	}
	cmd.Flags().IntVar(&waitFor, "wait", 0, "The number of seconds to wait for links to become connected")
	cmd.Flags().BoolVar(&verboseLinkStatus, "verbose", false, "Show detailed information about a link")

	return cmd

}
