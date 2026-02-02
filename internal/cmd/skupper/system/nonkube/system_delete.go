package nonkube

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/utils/validator"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
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
	logger               *slog.Logger
}

func NewCmdSystemDelete() *CmdSystemDelete {

	skupperCmd := CmdSystemDelete{
		logger: slog.New(slog.Default().Handler()).With("component", "nonkube.systemDelete"),
	}

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
	namespaceStringValidator := validator.NamespaceStringValidator()

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("This command does not accept arguments"))
	}

	if cmd.Flags == nil || cmd.Flags.Filename == "" {
		validationErrors = append(validationErrors, fmt.Errorf("You need to provide a file to delete custom resources or use standard input.\n Example: cat site.yaml | skupper system delete -f -"))
	}

	if cmd.Namespace != "" {
		ok, err := namespaceStringValidator.Evaluate(cmd.Namespace)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("namespace is not valid: %s", err))
		}
	}

	if cmd.Flags != nil && cmd.Flags.Filename != "" && cmd.Flags.Filename != "-" {

		if !strings.HasSuffix(cmd.Flags.Filename, ".yaml") && !strings.HasSuffix(cmd.Flags.Filename, ".json") {
			validationErrors = append(validationErrors, fmt.Errorf("The file has an unsupported extension, it should have one of the following: .yaml, .json"))
		}

		info, err := os.Stat(cmd.Flags.Filename)
		if os.IsNotExist(err) {
			validationErrors = append(validationErrors, fmt.Errorf("The file %q does not exist", cmd.Flags.Filename))
		} else if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("Error while accessing the file: %s", err))
		}

		if err == nil && info.IsDir() {
			validationErrors = append(validationErrors, fmt.Errorf("The file %q is a directory", cmd.Flags.Filename))
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdSystemDelete) InputToOptions() {
	cmd.file = cmd.Flags.Filename
	if cmd.Flags.Filename == "-" {
		cmd.file = ""
	}
	if cmd.Namespace == "" {
		cmd.Namespace = "default"
	}
}

func (cmd *CmdSystemDelete) Run() error {
	//read the file or the pipe stream
	inputReader := cmd.CobraCmd.InOrStdin()
	crDeleted := false

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
		return fmt.Errorf("Failed parsing the custom resources: %s", err)
	}

	for _, site := range parsedInput.Site {
		if site.Name != "" {
			err := cmd.siteHandler.Delete(site.Name)
			if err != nil {
				cmd.logger.Error("Error while deleting site", slog.String("site", site.Name), slog.Any("error", err))
			} else {
				crDeleted = true
				fmt.Printf("Site %s deleted\n", site.Name)
			}
		}
	}

	for _, connector := range parsedInput.Connector {
		if connector.Name != "" {
			err := cmd.connectorHandler.Delete(connector.Name)
			if err != nil {
				cmd.logger.Error("Error while deleting connector", slog.String("connector", connector.Name), slog.Any("error", err))
			} else {
				crDeleted = true
				fmt.Printf("Connector %s deleted\n", connector.Name)
			}
		}
	}

	for _, listener := range parsedInput.Listener {
		if listener.Name != "" {
			err := cmd.listenerHandler.Delete(listener.Name)
			if err != nil {
				cmd.logger.Error("Error while deleting listener", slog.String("listener", listener.Name), slog.Any("error", err))
			} else {
				crDeleted = true
				fmt.Printf("Listener %s deleted\n", listener.Name)
			}
		}
	}

	for _, link := range parsedInput.Link {
		if link.Name != "" {
			err := cmd.linkHandler.Delete(link.Name)
			if err != nil {
				cmd.logger.Error("Error while deleting link", slog.String("link", link.Name), slog.Any("error", err))
			} else {
				crDeleted = true
				fmt.Printf("Link %s deleted\n", link.Name)
			}

		}
	}

	for _, routerAccess := range parsedInput.RouterAccess {
		if routerAccess.Name != "" {
			err := cmd.routerAccessHandler.Delete(routerAccess.Name)
			if err != nil {
				cmd.logger.Error("Error while deleting router access", slog.String("router access", routerAccess.Name), slog.Any("error", err))
			} else {
				crDeleted = true
				fmt.Printf("RouterAccess %s deleted\n", routerAccess.Name)
			}

		}
	}

	for _, accessToken := range parsedInput.AccessToken {
		if accessToken.Name != "" {
			err := cmd.accessTokenHandler.Delete(accessToken.Name)
			if err != nil {
				cmd.logger.Error("Error while deleting access token", slog.String("access token", accessToken.Name), slog.Any("error", err))
			} else {
				crDeleted = true
				fmt.Printf("AccessToken %s deleted\n", accessToken.Name)
			}
		}
	}

	for _, secret := range parsedInput.Secret {
		if secret.Name != "" {
			err := cmd.secretHandler.Delete(secret.Name)
			if err != nil {
				cmd.logger.Error("Error while deleting secret", slog.String("secret", secret.Name), slog.Any("error", err))
			} else {
				crDeleted = true
				fmt.Printf("Secret %s deleted\n", secret.Name)
			}
		}
	}

	for _, certificate := range parsedInput.Certificate {
		if certificate.Name != "" {
			err := cmd.certificateHandler.Delete(certificate.Name)
			if err != nil {
				cmd.logger.Error("Error while deleting certificate", slog.String("certificate", certificate.Name), slog.Any("error", err))
			} else {
				crDeleted = true
				fmt.Printf("Certificate %s deleted\n", certificate.Name)
			}
		}
	}

	for _, securedAccess := range parsedInput.SecuredAccess {
		if securedAccess.Name != "" {
			err := cmd.securedAccessHandler.Delete(securedAccess.Name)
			if err != nil {
				cmd.logger.Error("Error while deleting secured access", slog.String("secured access", securedAccess.Name), slog.Any("error", err))
			} else {
				crDeleted = true
				fmt.Printf("SecuredAccess %s deleted\n", securedAccess.Name)
			}
		}
	}

	if crDeleted {
		fmt.Println("Custom resources deleted. If a site is already running, run `skupper system reload` to make effective the changes.")
	}

	return nil
}

func (cmd *CmdSystemDelete) WaitUntil() error { return nil }
