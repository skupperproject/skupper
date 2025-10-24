/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package kube

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	pkgutils "github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CmdLinkGenerate struct {
	Client             skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient         kubernetes.Interface
	CobraCmd           *cobra.Command
	Flags              *common.CommandLinkGenerateFlags
	linkName           string
	Namespace          string
	tlsCredentials     string
	cost               int
	output             string
	activeSite         *v2alpha1.Site
	generateCredential bool
	generatedLinks     []v2alpha1.Link
	timeout            time.Duration
}

func NewCmdLinkGenerate() *CmdLinkGenerate {

	skupperCmd := CmdLinkGenerate{}

	return &skupperCmd
}

func (cmd *CmdLinkGenerate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(utils.GenericError, err)

	cmd.Client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.KubeClient = cli.GetKubeClient()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdLinkGenerate) ValidateInput(args []string) error {

	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	timeoutValidator := validator.NewTimeoutInSecondsValidator()
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)

	// check if CRDs are installed
	_, err := cmd.Client.Certificates(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		validationErrors = append(validationErrors, utils.HandleMissingCrds(err))
		return errors.Join(validationErrors...)
	}

	//Validate if Site CRD is installed and if there is already a site defined in the namespace
	siteList, err := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		validationErrors = append(validationErrors, utils.HandleMissingCrds(err))
		return errors.Join(validationErrors...)
	}
	if siteList != nil && len(siteList.Items) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("there is no skupper site in this namespace"))
	}

	for _, s := range siteList.Items {
		if s.IsReady() {
			cmd.activeSite = &s
		}
	}

	if cmd.activeSite == nil {
		validationErrors = append(validationErrors, fmt.Errorf("there is no active skupper site in this namespace"))
	}

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("arguments are not allowed in this command"))
	}

	if cmd.Flags.TlsCredentials == "" && !cmd.Flags.GenerateCredential {
		validationErrors = append(validationErrors, fmt.Errorf("the TLS secret name was not specified"))
	} else if cmd.Flags.TlsCredentials != "" {
		ok, err := resourceStringValidator.Evaluate(cmd.Flags.TlsCredentials)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("the name of the tls secret is not valid: %s", err))
		}
	}

	selectedCost, err := strconv.Atoi(cmd.Flags.Cost)
	if err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("link cost is not valid: %s", err))
	}
	ok, err := numberValidator.Evaluate(selectedCost)
	if !ok {
		validationErrors = append(validationErrors, fmt.Errorf("link cost is not valid: %s", err))
	}

	if cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		}
	}

	ok, err = timeoutValidator.Evaluate(cmd.Flags.Timeout)
	if !ok {
		validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid: %s", err))
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdLinkGenerate) InputToOptions() {

	cmd.cost, _ = strconv.Atoi(cmd.Flags.Cost)
	cmd.output = cmd.Flags.Output
	if cmd.output == "" {
		cmd.output = "yaml"
	}
	cmd.generateCredential = cmd.Flags.GenerateCredential
	cmd.timeout = cmd.Flags.Timeout

	var generatedLinkName string
	if cmd.activeSite != nil {
		generatedLinkName = "link-" + cmd.activeSite.Name
	}

	cmd.linkName = generatedLinkName

	if cmd.Flags.TlsCredentials == "" {
		cmd.tlsCredentials = generatedLinkName
	} else {
		cmd.tlsCredentials = cmd.Flags.TlsCredentials
	}

}

func (cmd *CmdLinkGenerate) Run() error {

	if cmd.activeSite == nil {
		return fmt.Errorf("There is no active site to generate the link resource file")
	} else if len(cmd.activeSite.Status.Endpoints) == 0 {
		return fmt.Errorf("A link cannot be generated because link access is not enabled. \n Use \"skupper site update --enable-link-access\" to enable it.")
	}

	if cmd.output == "" {
		return fmt.Errorf("Output format is not specified")
	}

	endpointMap := getEndpointsByGroups(cmd.activeSite.Status.Endpoints)

	var resources []v2alpha1.Link
	for key, endpointGroup := range endpointMap {

		resource := v2alpha1.Link{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "skupper.io/v2alpha1",
				Kind:       "Link",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: strings.Join([]string{cmd.linkName, key}, "-"),
			},
			Spec: v2alpha1.LinkSpec{
				TlsCredentials: cmd.tlsCredentials,
				Cost:           cmd.cost,
				Endpoints:      endpointGroup,
			},
		}

		resources = append(resources, resource)

	}

	cmd.generatedLinks = resources

	if cmd.generateCredential {
		//Check if the certificate was previously created
		_, err := cmd.Client.Certificates(cmd.Namespace).Get(context.TODO(), cmd.tlsCredentials, metav1.GetOptions{})
		if err != nil {
			certificate := v2alpha1.Certificate{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "Certificate",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: cmd.tlsCredentials,
				},
				Spec: v2alpha1.CertificateSpec{
					Ca:      cmd.activeSite.Status.DefaultIssuer,
					Client:  true,
					Subject: getSubjectsFromEndpoints(cmd.activeSite.Status.Endpoints),
				},
			}
			defaultIssuer := pkgutils.DefaultStr(cmd.activeSite.Status.DefaultIssuer, "skupper-site-ca")
			defaultIssuerCert, err := cmd.Client.Certificates(cmd.Namespace).Get(context.TODO(), defaultIssuer, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("unable to retrieve default issuer certificate %q: %w", defaultIssuer, err)
			}
			certificateController := defaultIssuerCert.Spec.GetCertificateController()
			certificate.Spec.SetCertificateController(certificateController)
			_, err = cmd.Client.Certificates(cmd.Namespace).Create(context.TODO(), &certificate, metav1.CreateOptions{})
			if err != nil {
				return err
			}
		}

	}

	return nil
}

func (cmd *CmdLinkGenerate) WaitUntil() error {

	var resourcesToPrint []string

	for _, generatedLink := range cmd.generatedLinks {
		encodedOutput, err := utils.Encode(cmd.output, generatedLink)
		if err != nil {
			return err
		}

		resourcesToPrint = append(resourcesToPrint, encodedOutput)
	}

	if cmd.generateCredential {

		ctxWithTimeout, cancel := context.WithTimeout(context.Background(), cmd.timeout)
		defer cancel()

		err := pkgutils.RetryErrorWithContext(ctxWithTimeout, time.Second, func() error {

			generatedSecret, err := cmd.KubeClient.CoreV1().Secrets(cmd.Namespace).Get(context.TODO(), cmd.tlsCredentials, metav1.GetOptions{})
			if err != nil {
				return err
			}

			if generatedSecret != nil {

				secretResource := &corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Secret",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: cmd.tlsCredentials,
					},
					Type: "kubernetes.io/tls",
					Data: map[string][]byte{
						"tls.crt": generatedSecret.Data["tls.crt"],
						"tls.key": generatedSecret.Data["tls.key"],
						"ca.crt":  generatedSecret.Data["ca.crt"],
					},
				}

				encodedSecret, _ := utils.Encode(cmd.output, secretResource)
				resourcesToPrint = append(resourcesToPrint, encodedSecret)
			}
			return nil
		})

		if err != nil {
			return fmt.Errorf("TLS secret %q not ready yet, check the status for more information\n", cmd.tlsCredentials)
		}

	}

	printResources(resourcesToPrint, cmd.output)

	if cmd.generateCredential {
		_, err := cmd.Client.Certificates(cmd.Namespace).Get(context.TODO(), cmd.tlsCredentials, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("there was an error trying to delete the generated certificate: %s", err)
		}
	}

	return nil
}

func getSubjectsFromEndpoints(endpointList []v2alpha1.Endpoint) string {

	var hosts []string
	for _, endpoint := range endpointList {
		hosts = append(hosts, endpoint.Host)
	}

	return strings.Join(hosts, ",")
}

func printResources(resources []string, outputFormat string) {

	for index, resource := range resources {
		fmt.Println(resource)
		if outputFormat == "yaml" && index < len(resources)-1 {
			fmt.Println("---")
		}
	}
}

func getEndpointsByGroups(endpointList []v2alpha1.Endpoint) map[string][]v2alpha1.Endpoint {

	endpointGroup := make(map[string][]v2alpha1.Endpoint)

	for _, endpoint := range endpointList {
		if len(endpointGroup[endpoint.Group]) > 0 {
			endpointGroup[endpoint.Group] = append(endpointGroup[endpoint.Group], endpoint)
		} else {
			endpointGroup[endpoint.Group] = []v2alpha1.Endpoint{endpoint}
		}
	}

	return endpointGroup
}
