package kube

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/utils/validator"

	"github.com/skupperproject/skupper/internal/kube/client"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdSiteDelete struct {
	Client    skupperv2alpha1.SkupperV2alpha1Interface
	CobraCmd  *cobra.Command
	Flags     *common.CommandSiteDeleteFlags
	Namespace string
	siteName  string
	timeout   time.Duration
	wait      bool
	all       bool
}

func NewCmdSiteDelete() *CmdSiteDelete {

	skupperCmd := CmdSiteDelete{}

	return &skupperCmd
}

func (cmd *CmdSiteDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(utils.GenericError, err)

	cmd.Client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdSiteDelete) ValidateInput(args []string) error {
	var validationErrors []error
	timeoutValidator := validator.NewTimeoutInSecondsValidator()

	//Validate if there is already a site defined in the namespace
	siteList, err := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		validationErrors = append(validationErrors, err)
	} else if siteList == nil || (siteList != nil && len(siteList.Items) == 0) {
		validationErrors = append(validationErrors, fmt.Errorf("there is no existing Skupper site resource to delete"))
	} else {

		if len(args) > 1 {
			validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
		} else if len(args) == 1 {

			selectedSite := args[0]
			for _, s := range siteList.Items {
				if s.Name == selectedSite {
					cmd.siteName = s.Name
				}
			}

			if cmd.siteName == "" {
				validationErrors = append(validationErrors, fmt.Errorf("site with name %q is not available", selectedSite))
			}
		} else if len(args) == 0 {
			if len(siteList.Items) > 1 {
				validationErrors = append(validationErrors, fmt.Errorf("site name is required because there are several sites in this namespace"))
			} else if len(siteList.Items) == 1 {
				cmd.siteName = siteList.Items[0].Name
			}
		}
	}

	if cmd.Flags != nil && cmd.Flags.Timeout.String() != "" {
		ok, err := timeoutValidator.Evaluate(cmd.Flags.Timeout)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid: %s", err))
		}
	}

	return errors.Join(validationErrors...)
}
func (cmd *CmdSiteDelete) InputToOptions() {
	cmd.timeout = cmd.Flags.Timeout
	cmd.wait = cmd.Flags.Wait
	cmd.all = cmd.Flags.All
}

func (cmd *CmdSiteDelete) Run() error {

	err := cmd.Client.Sites(cmd.Namespace).Delete(context.TODO(), cmd.siteName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	// if delete all, also remove all the other resources
	if cmd.all {
		connectors, err := cmd.Client.Connectors(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err == nil && connectors != nil && len(connectors.Items) != 0 {
			for _, connector := range connectors.Items {
				err = cmd.Client.Connectors(cmd.Namespace).Delete(context.TODO(), connector.Name, metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}
		}

		listeners, err := cmd.Client.Listeners(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err == nil && listeners != nil && len(listeners.Items) != 0 {
			for _, listener := range listeners.Items {
				err = cmd.Client.Listeners(cmd.Namespace).Delete(context.TODO(), listener.Name, metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}
		}

		//Links are removed after the removal of the site by the controller

		accessTokens, err := cmd.Client.AccessTokens(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err == nil && accessTokens != nil && len(accessTokens.Items) != 0 {
			for _, accessToken := range accessTokens.Items {
				err = cmd.Client.AccessTokens(cmd.Namespace).Delete(context.TODO(), accessToken.Name, metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}
		}

		accessGrants, err := cmd.Client.AccessGrants(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err == nil && accessGrants != nil && len(accessGrants.Items) != 0 {
			for _, accessGrant := range accessGrants.Items {
				err = cmd.Client.AccessGrants(cmd.Namespace).Delete(context.TODO(), accessGrant.Name, metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}
		}

		routerAccesses, err := cmd.Client.RouterAccesses(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err == nil && routerAccesses != nil && len(routerAccesses.Items) != 0 {
			for _, routerAccess := range routerAccesses.Items {
				err = cmd.Client.RouterAccesses(cmd.Namespace).Delete(context.TODO(), routerAccess.Name, metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}
		}

		securedAccesses, err := cmd.Client.SecuredAccesses(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err == nil && securedAccesses != nil && len(securedAccesses.Items) != 0 {
			for _, securedAccess := range securedAccesses.Items {
				err = cmd.Client.SecuredAccesses(cmd.Namespace).Delete(context.TODO(), securedAccess.Name, metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}
		}

		certificates, err := cmd.Client.Certificates(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err == nil && certificates != nil && len(certificates.Items) != 0 {
			for _, certificate := range certificates.Items {
				err = cmd.Client.Certificates(cmd.Namespace).Delete(context.TODO(), certificate.Name, metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}
		}

		attachedConnectors, err := cmd.Client.AttachedConnectors(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err == nil && attachedConnectors != nil && len(attachedConnectors.Items) != 0 {
			for _, attachedConnector := range attachedConnectors.Items {
				err = cmd.Client.AttachedConnectors(cmd.Namespace).Delete(context.TODO(), attachedConnector.Name, metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}
		}

		attachedConnectorBindings, err := cmd.Client.AttachedConnectorBindings(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err == nil && attachedConnectorBindings != nil && len(attachedConnectorBindings.Items) != 0 {
			for _, attachedConnectorBinding := range attachedConnectorBindings.Items {
				err = cmd.Client.AttachedConnectorBindings(cmd.Namespace).Delete(context.TODO(), attachedConnectorBinding.Name, metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
func (cmd *CmdSiteDelete) WaitUntil() error {

	if cmd.wait {
		waitTime := int(cmd.timeout.Seconds())
		err := utils.NewSpinnerWithTimeout("Waiting for deletion to complete...", waitTime, func() error {

			resource, err := cmd.Client.Sites(cmd.Namespace).Get(context.TODO(), cmd.siteName, metav1.GetOptions{})

			if err == nil && resource != nil {
				return fmt.Errorf("error deleting the resource")
			} else {
				return nil
			}

		})

		if err != nil {
			return fmt.Errorf("Site %q not deleted yet, check the status for more information\n", cmd.siteName)
		}

		fmt.Printf("Site %q is deleted\n", cmd.siteName)
	}

	return nil
}
