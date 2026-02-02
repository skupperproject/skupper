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

type CmdSystemApply struct {
	Client               skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient           kubernetes.Interface
	CobraCmd             *cobra.Command
	Namespace            string
	Flags                *common.CommandSystemApplyFlags
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

func NewCmdSystemApply() *CmdSystemApply {

	skupperCmd := CmdSystemApply{
		logger: slog.New(slog.Default().Handler()).With("component", "nonkube.systemApply"),
	}

	return &skupperCmd
}

func (cmd *CmdSystemApply) NewClient(cobraCommand *cobra.Command, args []string) {
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

func (cmd *CmdSystemApply) ValidateInput(args []string) error {
	var validationErrors []error
	namespaceStringValidator := validator.NamespaceStringValidator()

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("This command does not accept arguments"))
	}

	if cmd.Flags == nil || cmd.Flags.Filename == "" {
		validationErrors = append(validationErrors, fmt.Errorf("You need to provide a file to apply or use standard input.\n Example: cat site.yaml | skupper system apply -f -"))
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

func (cmd *CmdSystemApply) InputToOptions() {
	cmd.file = cmd.Flags.Filename
	if cmd.Flags.Filename == "-" {
		cmd.file = ""
	}
	if cmd.Namespace == "" {
		cmd.Namespace = "default"
	}
}

func (cmd *CmdSystemApply) Run() error {

	//read the file or the pipe stream
	inputReader := cmd.CobraCmd.InOrStdin()
	crApplied := false

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
		err := cmd.siteHandler.Add(site)
		if err != nil {
			cmd.logger.Error("Error while adding site", slog.String("site", site.Name), slog.Any("error", err))
		} else {
			crApplied = true
			fmt.Printf("Site %s added\n", site.Name)
		}
	}

	for _, connector := range parsedInput.Connector {
		err := cmd.connectorHandler.Add(connector)
		if err != nil {
			cmd.logger.Error("Error while adding connector", slog.String("connector", connector.Name), slog.Any("error", err))
		} else {
			crApplied = true
			fmt.Printf("Connector %s added\n", connector.Name)
		}
	}

	for _, listener := range parsedInput.Listener {
		err := cmd.listenerHandler.Add(listener)
		if err != nil {
			cmd.logger.Error("Error while adding listener", slog.String("listener", listener.Name), slog.Any("error", err))
		} else {
			crApplied = true
			fmt.Printf("Listener %s added\n", listener.Name)
		}
	}

	for _, link := range parsedInput.Link {
		err := cmd.linkHandler.Add(link)
		if err != nil {
			cmd.logger.Error("Error while adding link", slog.String("link", link.Name), slog.Any("error", err))
		} else {
			crApplied = true
			fmt.Printf("Link %s added\n", link.Name)
		}
	}

	for _, routerAccess := range parsedInput.RouterAccess {
		err := cmd.routerAccessHandler.Add(routerAccess)
		if err != nil {
			cmd.logger.Error("Error while adding router access", slog.String("router access", routerAccess.Name), slog.Any("error", err))
		} else {
			crApplied = true
			fmt.Printf("RouterAccess %s added\n", routerAccess.Name)
		}
	}

	for _, accessToken := range parsedInput.AccessToken {
		err := cmd.accessTokenHandler.Add(accessToken)
		if err != nil {
			cmd.logger.Error("Error while adding access token", slog.String("access token", accessToken.Name), slog.Any("error", err))
		} else {
			crApplied = true
			fmt.Printf("AccessToken %s added\n", accessToken.Name)
		}
	}

	for _, secret := range parsedInput.Secret {
		err := cmd.secretHandler.Add(secret)
		if err != nil {
			cmd.logger.Error("Error while adding secret", slog.String("secret", secret.Name), slog.Any("error", err))
		} else {
			crApplied = true
			fmt.Printf("Secret %s added\n", secret.Name)
		}
	}

	for _, securedAccess := range parsedInput.SecuredAccess {
		err := cmd.securedAccessHandler.Add(securedAccess)
		if err != nil {
			cmd.logger.Error("Error while adding secured access", slog.String("secured access", securedAccess.Name), slog.Any("error", err))
		} else {
			crApplied = true
			fmt.Printf("SecuredAccess %s added\n", securedAccess.Name)
		}
	}

	for _, certificate := range parsedInput.Certificate {
		err := cmd.certificateHandler.Add(certificate)
		if err != nil {
			cmd.logger.Error("Error while adding certificate", slog.String("certificate", certificate.Name), slog.Any("error", err))
		} else {
			crApplied = true
			fmt.Printf("Certificate %s added\n", certificate.Name)
		}
	}

	if crApplied {
		fmt.Println("Custom resources are applied. If a site is already running, run `skupper system reload` to make effective the changes.")
	}

	return nil
}

func (cmd *CmdSystemApply) WaitUntil() error { return nil }
