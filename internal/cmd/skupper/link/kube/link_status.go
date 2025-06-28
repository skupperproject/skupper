package kube

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdLinkStatus struct {
	Client    skupperv2alpha1.SkupperV2alpha1Interface
	CobraCmd  *cobra.Command
	Flags     *common.CommandLinkStatusFlags
	Namespace string
	output    string
	linkName  string
	siteName  string
}

func NewCmdLinkStatus() *CmdLinkStatus {

	return &CmdLinkStatus{}
}

func (cmd *CmdLinkStatus) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(utils.GenericError, err)

	cmd.Client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdLinkStatus) ValidateInput(args []string) error {
	var validationErrors []error
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)

	// Check if CRDs are installed
	_, err := cmd.Client.Links(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		validationErrors = append(validationErrors, utils.HandleMissingCrds(err))
		return errors.Join(validationErrors...)
	}

	siteList, err := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		validationErrors = append(validationErrors, utils.HandleMissingCrds(err))
		return errors.Join(validationErrors...)
	}
	if siteList == nil || len(siteList.Items) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("there is no skupper site available"))
	} else if len(siteList.Items) == 1 {
		cmd.siteName = siteList.Items[0].Name
	}
	if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("this command only accepts one argument"))
	}

	if cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		}
	}

	if len(args) >= 1 && args[0] != "" {
		cmd.linkName = args[0]
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdLinkStatus) InputToOptions() {
	cmd.output = cmd.Flags.Output
}

func (cmd *CmdLinkStatus) Run() error {

	if cmd.linkName != "" {
		// display outgoing links
		selectedLink, err := cmd.Client.Links(cmd.Namespace).Get(context.TODO(), cmd.linkName, metav1.GetOptions{})
		if err == nil {
			if cmd.output != "" {
				return printEncodedOutput(cmd.output, selectedLink)
			} else {
				displaySingleLink(selectedLink)
			}
		}
		// display incoming links
		currentSite, err := cmd.Client.Sites(cmd.Namespace).Get(context.TODO(), cmd.siteName, metav1.GetOptions{})
		if err == nil && currentSite != nil {
			if cmd.output != "" {
				printEncodedIncomingLink(currentSite, cmd.siteName, cmd.linkName, cmd.output)
			} else {
				displaySingleIncomingLink(currentSite, cmd.siteName, cmd.linkName)
			}
		}

	} else {
		// display outgoing links
		linkList, err := cmd.Client.Links(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}

		if linkList != nil && len(linkList.Items) == 0 {
			fmt.Println("There are no outgoing link resources in the namespace")
		} else {
			if cmd.output != "" {
				for _, link := range linkList.Items {
					err := printEncodedOutput(cmd.output, &link)

					if err != nil {
						return err
					}
				}
			} else {
				displayLinkList(linkList.Items)
			}
		}

		// display incoming links
		currentSite, err := cmd.Client.Sites(cmd.Namespace).Get(context.TODO(), cmd.siteName, metav1.GetOptions{})
		if err == nil && currentSite != nil {
			if cmd.output != "" {
				printEncodedIncomingLink(currentSite, cmd.siteName, "", cmd.output)
			} else {
				displayIncomingLink(currentSite, cmd.siteName)
			}
		}
	}
	return nil
}
func (cmd *CmdLinkStatus) WaitUntil() error { return nil }

func printEncodedOutput(outputType string, link *v2alpha1.Link) error {
	encodedOutput, err := utils.Encode(outputType, link)
	fmt.Println(encodedOutput)
	return err
}

func displaySingleLink(link *v2alpha1.Link) {
	fmt.Println("Outgoing link from this site:")
	fmt.Printf("%s\t: %s\n", "Name", link.Name)
	fmt.Printf("%s\t: %s\n", "Status", link.Status.StatusType)
	fmt.Printf("%s\t: %d\n", "Cost", link.Spec.Cost)
	fmt.Printf("%s\t: %s\n", "Message", link.Status.Message)
}

func displayLinkList(linkList []v2alpha1.Link) {
	writer := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', tabwriter.AlignRight)
	fmt.Println("Outgoing link from this site:")
	fmt.Fprintln(writer, "NAME\tSTATUS\tCOST\tMESSAGE")

	for _, link := range linkList {
		fmt.Fprintf(writer, "%s\t%s\t%d\t%s", link.Name, link.Status.StatusType, link.Spec.Cost, link.Status.Message)
		fmt.Fprintln(writer)
	}

	writer.Flush()
}

func displayIncomingLink(currentSite *v2alpha1.Site, localSiteName string) {
	incomingLinksFound := false
	writer := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', tabwriter.AlignRight)
	fmt.Fprintln(writer, "\nIncoming Links from remote sites: ")
	for _, links := range currentSite.Status.Network {
		for _, link := range links.Links {
			if link.RemoteSiteName == localSiteName {
				if !incomingLinksFound {
					fmt.Fprintln(writer, "NAME\tSTATUS\tREMOTE SITE")
				}
				status := "Error"
				if link.Operational {
					status = "Ready"
				}
				fmt.Fprintf(writer, "%s\t%s\t%s", link.Name, status, links.Name)
				fmt.Fprintln(writer)
				incomingLinksFound = true
			}
		}
	}

	if !incomingLinksFound {
		fmt.Fprintln(writer, "There are no incoming link resources in the namespace")
	}

	writer.Flush()
}

func displaySingleIncomingLink(currentSite *v2alpha1.Site, localSiteName string, linkName string) {
	fmt.Println("\nIncoming Links from remote sites: ")
	for _, links := range currentSite.Status.Network {
		for _, link := range links.Links {
			if link.RemoteSiteName == localSiteName && link.Name == linkName {
				status := "Error"
				if link.Operational {
					status = "Ready"
				}
				fmt.Printf("%s\t: %s\n", "Name", link.Name)
				fmt.Printf("%s\t: %s\n", "Status", status)
				fmt.Printf("%s\t: %s\n", "Remote", links.Name)
			}
		}
	}
}

func printEncodedIncomingLink(currentSite *v2alpha1.Site, localSiteName string, linkName string, outputType string) {
	for _, links := range currentSite.Status.Network {
		for _, link := range links.Links {
			if link.RemoteSiteName == localSiteName {
				if linkName == "" || link.Name == linkName {
					encodedOutput, _ := utils.Encode(outputType, links)
					fmt.Println(encodedOutput)
				}
			}
		}
	}
}
