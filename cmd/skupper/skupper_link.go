package main

import (
	"context"
	"fmt"
	"io/ioutil"

	"time"

	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/pkg/utils/formatter"
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
			yaml, err := ioutil.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("Could not read connection token: %s", err.Error())
			}
			connectorCreateOpts.Yaml = yaml
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
var remoteInfoTimeout time.Duration
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
		Short:  "Check whether a link to another Skupper site is active",
		Args:   cobra.MaximumNArgs(1),
		PreRun: skupperClient.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			linkHandler := skupperClient.LinkHandler()
			if linkHandler == nil {
				return fmt.Errorf("unable to retrieve links")
			}
			if remoteInfoTimeout.Seconds() <= 0 {
				return fmt.Errorf(`invalid timeout value`)
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
					links, err := linkHandler.StatusAll()
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
							remoteLinks, err = linkHandler.RemoteLinks(ctx)
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
								var nsStr string
								if remoteLink.Namespace != "" {
									nsStr = fmt.Sprintf("the namespace %s on site ", remoteLink.Namespace)
								}
								fmt.Printf("A link from %s%s(%s) is active ", nsStr, remoteLink.SiteName, remoteLink.SiteId)
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
		},
	}
	cmd.Flags().IntVar(&waitFor, "wait", 0, "The number of seconds to wait for links to become active")
	cmd.Flags().DurationVar(&remoteInfoTimeout, "timeout", types.DefaultTimeoutDuration, "Configurable timeout for retrieving information about remote links")
	cmd.Flags().BoolVar(&verboseLinkStatus, "verbose", false, "Show detailed information about a link")

	return cmd

}
