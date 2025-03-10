/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package nonkube

import (
	"errors"
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	nonkubecommon "github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

type CmdLinkGenerate struct {
	CobraCmd     *cobra.Command
	Namespace    string
	siteName     string
	tokenHandler *fs.TokenHandler
	siteState    *api.SiteState
	Flags        *common.CommandLinkGenerateFlags
	linkName     string
	endpointHost string
}

func NewCmdLinkGenerate() *CmdLinkGenerate {
	return &CmdLinkGenerate{}

}

func (cmd *CmdLinkGenerate) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.Namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.tokenHandler = fs.NewTokenHandler(cmd.Namespace)
}

func (cmd *CmdLinkGenerate) ValidateInput(args []string) error {

	var validationErrors []error

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("arguments are not allowed in this command"))
	}

	pathProvider := fs.PathProvider{Namespace: cmd.Namespace}
	siteStateLoader := &nonkubecommon.FileSystemSiteStateLoader{
		Path: pathProvider.GetRuntimeNamespace(),
	}

	siteState, err := siteStateLoader.Load()
	if err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("there is no active site in this namespace"))
	} else {

		hasRouterAccess := false
		if siteState.RouterAccesses != nil && len(siteState.RouterAccesses) > 0 {
			for _, access := range siteState.RouterAccesses {
				if strings.HasPrefix(access.Name, "router-access") {
					hasRouterAccess = true
					break
				}
			}
		}
		if !hasRouterAccess {
			validationErrors = append(validationErrors, fmt.Errorf("this site is not enabled for link access, there are no links created"))
		}
	}

	cmd.siteState = siteState

	return errors.Join(validationErrors...)

}

func (cmd *CmdLinkGenerate) InputToOptions() {
	cmd.linkName = cmd.Flags.Name
	cmd.endpointHost = cmd.Flags.Host
}
func (cmd *CmdLinkGenerate) Run() error {

	hostTokenPath := api.GetHostSiteInternalPath(cmd.siteState.Site, api.RuntimeTokenPath)

	opts := fs.GetOptions{
		LogWarning: false,
		Attributes: map[string]string{
			"linkName":     cmd.linkName,
			"endpointHost": cmd.endpointHost,
			"tokenPath":    hostTokenPath,
		},
	}
	tokenFiles, err := cmd.tokenHandler.List(opts)
	if err != nil {
		return fmt.Errorf("Error searching for tokens: %s", err)
	}

	if tokenFiles == nil || len(tokenFiles) == 0 {
		fmt.Println("no tokens found")
		return nil
	}

	for _, tokenFile := range tokenFiles {
		file, errFile := os.ReadFile(hostTokenPath + "/" + tokenFile)
		if errFile != nil {
			return fmt.Errorf("error reading file %s: %s", hostTokenPath+"/"+tokenFile, errFile)
		}
		fmt.Println(string(file))
	}

	return nil
}
func (cmd *CmdLinkGenerate) WaitUntil() error { return nil }
