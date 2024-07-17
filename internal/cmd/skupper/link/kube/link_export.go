/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package kube

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strconv"
)

var (
	linkExportLong = `Generate a new link resource in a yaml file with the data needed from the target site. The resultant
yaml file needs to be applied in the site in which we want to create the link.`
)

type ExportLinkFlags struct {
	tlsSecret          string
	cost               string
	output             string
	generateCredential bool
}

type CmdLinkExport struct {
	Client             skupperv1alpha1.SkupperV1alpha1Interface
	KubeClient         kubernetes.Interface
	CobraCmd           cobra.Command
	flags              ExportLinkFlags
	linkName           string
	Namespace          string
	tlsSecret          string
	cost               int
	output             string
	outputFile         string
	activeSite         *v1alpha1.Site
	generateCredential bool
}

func NewCmdLinkExport() *CmdLinkExport {

	skupperCmd := CmdLinkExport{flags: ExportLinkFlags{}}

	cmd := cobra.Command{
		Use:    "export <name> <output file>",
		Short:  "Generate a new link resource in a yaml file",
		Long:   linkExportLong,
		PreRun: skupperCmd.NewClient,
		Run: func(cmd *cobra.Command, args []string) {
			utils.HandleErrorList(skupperCmd.ValidateInput(args))
			skupperCmd.InputToOptions()
			utils.HandleError(skupperCmd.Run())
		},
	}

	skupperCmd.CobraCmd = cmd
	skupperCmd.AddFlags()

	return &skupperCmd
}

func (cmd *CmdLinkExport) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.Client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.KubeClient = cli.GetKubeClient()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdLinkExport) AddFlags() {
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.tlsSecret, "tls-secret", "", "the name of a Kubernetes secret containing TLS credentials.")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.cost, "cost", "1", "the configured \"expense\" of sending traffic over the link. ")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.output, "output", "o", "yaml", "print resources to the console instead of submitting them to the Skupper controller. Choices: json, yaml")
	cmd.CobraCmd.Flags().BoolVar(&cmd.flags.generateCredential, "generate-credential", true, "print resources to the console instead of submitting them to the Skupper controller. Choices: json, yaml")
}

func (cmd *CmdLinkExport) ValidateInput(args []string) []error {

	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	outputTypeValidator := validator.NewOptionValidator(utils.OutputTypes)

	//Validate if there is already a site defined in the namespace
	siteList, _ := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if siteList != nil && len(siteList.Items) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("there is no skupper site in this namespace"))
	}

	for _, s := range siteList.Items {
		if s.Status.Status.StatusMessage == "OK" && s.Status.Active {
			cmd.activeSite = &s
		}
	}

	if cmd.activeSite == nil {
		validationErrors = append(validationErrors, fmt.Errorf("there is no active skupper site in this namespace"))
	}

	if len(args) < 2 || args[0] == "" || args[1] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("link name and output file must not be empty"))
	} else if len(args) > 2 {
		validationErrors = append(validationErrors, fmt.Errorf("only two arguments are allowed for this command."))
	} else {
		cmd.linkName = args[0]

		ok, err := resourceStringValidator.Evaluate(cmd.linkName)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("link name is not valid: %s", err))
		}

		cmd.outputFile = args[1]
	}

	if cmd.flags.tlsSecret == "" {
		validationErrors = append(validationErrors, fmt.Errorf("the TLS secret name was not specified"))
	} else {
		ok, err := resourceStringValidator.Evaluate(cmd.flags.tlsSecret)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("the name of the tls secret is not valid: %s", err))
		}
	}

	selectedCost, err := strconv.Atoi(cmd.flags.cost)
	if err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("link cost is not valid: %s", err))
	}
	ok, err := numberValidator.Evaluate(selectedCost)
	if !ok {
		validationErrors = append(validationErrors, fmt.Errorf("link cost is not valid: %s", err))
	}

	if cmd.flags.output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.flags.output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		}
	}

	return validationErrors
}

func (cmd *CmdLinkExport) InputToOptions() {

	cmd.cost, _ = strconv.Atoi(cmd.flags.cost)
	cmd.tlsSecret = cmd.flags.tlsSecret
	cmd.output = cmd.flags.output
	cmd.generateCredential = cmd.flags.generateCredential

}

func (cmd *CmdLinkExport) Run() error {

	if cmd.activeSite == nil {
		return fmt.Errorf("there is no active site to generate the link resource file")
	} else if len(cmd.activeSite.Status.Endpoints) == 0 {
		return fmt.Errorf("the active site has not configured endpoints yet")
	}

	if cmd.output == "" {
		return fmt.Errorf("output format has not been specified")
	}

	resource := v1alpha1.Link{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "Link",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: cmd.linkName,
		},
		Spec: v1alpha1.LinkSpec{
			TlsCredentials: cmd.tlsSecret,
			Cost:           cmd.cost,
			Endpoints:      cmd.activeSite.Status.Endpoints,
		},
	}

	encodedOutput, err := utils.Encode(cmd.output, resource)
	if err != nil {
		return err
	}

	var resourcesToPrint []string

	if cmd.generateCredential {
		certificate := v1alpha1.Certificate{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "skupper.io/v1alpha1",
				Kind:       "Certificate",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: cmd.tlsSecret,
			},
			Spec: v1alpha1.CertificateSpec{
				Ca:     cmd.activeSite.Status.DefaultIssuer,
				Server: true,
			},
		}
		encodedCredential, _ := utils.Encode(cmd.output, certificate)

		resourcesToPrint = []string{encodedCredential, encodedOutput}
	} else {
		resourcesToPrint = []string{encodedOutput}
	}

	utils.CreateFileWithResources(resourcesToPrint, cmd.output, cmd.outputFile)

	fmt.Printf("File %q has been created successfully.\n", cmd.outputFile)
	fmt.Println("Apply this resource in the site in which you want to create the link.")

	return nil
}

func (cmd *CmdLinkExport) WaitUntilReady() error { return nil }
