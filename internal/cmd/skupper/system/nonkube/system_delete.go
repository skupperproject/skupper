package nonkube

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"os"
)

type CmdSystemDelete struct {
	Client               skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient           kubernetes.Interface
	CobraCmd             *cobra.Command
	Namespace            string
	Flags                *common.CommandSystemDeleteFlags
	ParseInput           func(namespace string, reader *bufio.Reader, result *fs.InputFileResource) error
	siteHandler          *fs.SiteHandler
	connectorHandler     *fs.ConnectorHandler
	listenerHandler      *fs.ListenerHandler
	linkHandler          *fs.LinkHandler
	routerAccessHandler  *fs.RouterAccessHandler
	accessTokenHandler   *fs.AccessTokenHandler
	certificateHandler   *fs.CertificateHandler
	securedAccessHandler *fs.SecuredAccessHandler
	secretHandler        *fs.SecretHandler
	file                 string
}

func NewCmdSystemDelete() *CmdSystemDelete {

	skupperCmd := CmdSystemDelete{}

	return &skupperCmd
}

func (cmd *CmdSystemDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.Namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.connectorHandler = fs.NewConnectorHandler(cmd.Namespace)
	cmd.listenerHandler = fs.NewListenerHandler(cmd.Namespace)
	cmd.linkHandler = fs.NewLinkHandler(cmd.Namespace)
	cmd.routerAccessHandler = fs.NewRouterAccessHandler(cmd.Namespace)
	cmd.accessTokenHandler = fs.NewAccessTokenHandler(cmd.Namespace)
	cmd.siteHandler = fs.NewSiteHandler(cmd.Namespace)
	cmd.secretHandler = fs.NewSecretHandler(cmd.Namespace)
	cmd.certificateHandler = fs.NewCertificateHandler(cmd.Namespace)
	cmd.securedAccessHandler = fs.NewSecuredAccessHandler(cmd.Namespace)
	cmd.ParseInput = fs.ParseInput
}

func (cmd *CmdSystemDelete) ValidateInput(args []string) error {
	var validationErrors []error

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("This command does not accept arguments"))
	}

	if cmd.Flags.Filename != "" {
		info, err := os.Stat(cmd.Flags.Filename)
		if os.IsNotExist(err) {
			validationErrors = append(validationErrors, fmt.Errorf("The file %s does not exist", cmd.Flags.Filename))
		} else if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("Error while accessing the file: %s", err))
		} else if info.IsDir() {
			validationErrors = append(validationErrors, fmt.Errorf("The file %s is a directory", cmd.Flags.Filename))
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdSystemDelete) InputToOptions() {
	cmd.file = cmd.Flags.Filename
}

func (cmd *CmdSystemDelete) Run() error {
	//read the file or the pipe stream
	inputReader := cmd.CobraCmd.InOrStdin()

	if cmd.file != "" {
		file, err := os.Open(cmd.file)
		if err != nil {
			return fmt.Errorf("Error while opening the file: %s", err)
		}
		inputReader = file
	}

	//get custom resources from the input
	parsedInput := &fs.InputFileResource{}
	err := cmd.ParseInput(cmd.Namespace, bufio.NewReader(inputReader), parsedInput)
	if err != nil {
		return fmt.Errorf("failed parsing the custom resources: %s", err)
	}

	//add input to the resources namespace

	for _, site := range parsedInput.Site {
		err := cmd.siteHandler.Delete(site.Name)
		if err != nil {
			return err
		}
		fmt.Printf("Site %s deleted\n", site.Name)
	}

	for _, connector := range parsedInput.Connector {
		err := cmd.connectorHandler.Delete(connector.Name)
		if err != nil {
			return err
		}
		fmt.Printf("Connector %s deleted\n", connector.Name)
	}

	for _, listener := range parsedInput.Listener {
		err := cmd.listenerHandler.Delete(listener.Name)
		if err != nil {
			return err
		}
		fmt.Printf("Listener %s deleted\n", listener.Name)
	}

	for _, link := range parsedInput.Link {
		err := cmd.linkHandler.Delete(link.Name)
		if err != nil {
			return err
		}
		fmt.Printf("Link %s deleted\n", link.Name)
	}

	for _, routerAccess := range parsedInput.RouterAccess {
		err := cmd.routerAccessHandler.Delete(routerAccess.Name)
		if err != nil {
			return err
		}
		fmt.Printf("RouterAccess %s deleted\n", routerAccess.Name)
	}

	for _, accessToken := range parsedInput.AccessToken {
		err := cmd.accessTokenHandler.Delete(accessToken.Name)
		if err != nil {
			return err
		}
		fmt.Printf("AccessToken %s deleted\n", accessToken.Name)
	}

	for _, secret := range parsedInput.Secret {
		err := cmd.secretHandler.Delete(secret.Name)
		if err != nil {
			return err
		}
		fmt.Printf("Secret %s deleted\n", secret.Name)
	}

	for _, certificate := range parsedInput.Certificate {
		err := cmd.certificateHandler.Delete(certificate.Name)
		if err != nil {
			return err
		}
		fmt.Printf("Certificate %s deleted\n", certificate.Name)
	}

	for _, securedAccess := range parsedInput.SecuredAccess {
		err := cmd.securedAccessHandler.Delete(securedAccess.Name)
		if err != nil {
			return err
		}
		fmt.Printf("SecuredAccess %s deleted\n", securedAccess.Name)
	}

	fmt.Println("Custom resources deleted. You can now run `skupper reload` to make effective the changes.")

	return nil
}

func (cmd *CmdSystemDelete) WaitUntil() error { return nil }
