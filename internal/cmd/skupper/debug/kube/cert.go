package kube

import (
	"context"
	"errors"
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	certdisplay "github.com/skupperproject/skupper/internal/cmd/skupper/debug/cert"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/secrets"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CmdDebugCert struct {
	Client     skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   *cobra.Command
	Flags      *common.CommandDebugCertFlags
	Namespace  string
	certName   string
	output     string
}

func NewCmdDebugCert() *CmdDebugCert {
	return &CmdDebugCert{}
}

func (cmd *CmdDebugCert) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.Flags != nil && cmd.Flags.File != "" {
		return
	}
	cli, err := client.NewClient(
		cobraCommand.Flag("namespace").Value.String(),
		cobraCommand.Flag("context").Value.String(),
		cobraCommand.Flag("kubeconfig").Value.String(),
	)
	if err != nil {
		return
	}
	cmd.Client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.KubeClient = cli.GetKubeClient()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdDebugCert) ValidateInput(args []string) error {
	var validationErrors []error
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)
	filePathValidator := validator.NewFilePathStringValidator()

	if cmd.Flags != nil && cmd.Flags.File != "" {
		ok, err := filePathValidator.Evaluate(cmd.Flags.File)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("file path is not valid: %s", err))
		}
		if len(args) > 0 {
			validationErrors = append(validationErrors, fmt.Errorf("file flag cannot be used with a certificate name argument"))
		}
	} else {
		if cmd.Client == nil || cmd.KubeClient == nil {
			validationErrors = append(validationErrors, fmt.Errorf("failed setting up command"))
		}
		if len(args) > 1 {
			validationErrors = append(validationErrors, fmt.Errorf("only one certificate name is allowed"))
		}
		if len(args) == 1 {
			cmd.certName = args[0]
		}
	}

	if cmd.Flags != nil && cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdDebugCert) InputToOptions() {
	if cmd.Flags != nil {
		cmd.output = cmd.Flags.Output
	}
}

func (cmd *CmdDebugCert) Run() error {
	if cmd.Flags != nil && cmd.Flags.File != "" {
		info, err := certdisplay.ParseCertificateFile(cmd.Flags.File, cmd.Flags.File)
		if err != nil {
			return fmt.Errorf("failed to parse certificate from %s: %w", cmd.Flags.File, err)
		}
		return certdisplay.Display([]certdisplay.Info{*info}, cmd.output, true)
	}

	if cmd.certName != "" {
		info, err := cmd.certInfoByName(cmd.certName)
		if err != nil {
			return err
		}
		return certdisplay.Display([]certdisplay.Info{*info}, cmd.output, true)
	}

	infos, err := cmd.collectCertInfos()
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		fmt.Println("No certificates found in the namespace")
		return nil
	}
	return certdisplay.Display(infos, cmd.output, false)
}

func (cmd *CmdDebugCert) certInfoByName(name string) (*certdisplay.Info, error) {
	certificate, err := cmd.Client.Certificates(cmd.Namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		return cmd.certInfoFromCR(certificate)
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	secret, err := cmd.KubeClient.CoreV1().Secrets(cmd.Namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !secrets.IsTlsCredentialSecret(secret) {
		return nil, fmt.Errorf("secret %s is not a TLS credential", name)
	}
	return cmd.certInfoFromSecret(name, secret, "", "", nil)
}

func (cmd *CmdDebugCert) collectCertInfos() ([]certdisplay.Info, error) {
	seen := map[string]bool{}
	var infos []certdisplay.Info

	certificateList, err := cmd.Client.Certificates(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, utils.HandleMissingCrds(err)
	}

	for _, certificate := range certificateList.Items {
		info, err := cmd.certInfoFromCR(&certificate)
		if err != nil {
			return nil, err
		}
		seen[certificate.Name] = true
		infos = append(infos, *info)
	}

	secretList, err := cmd.KubeClient.CoreV1().Secrets(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, secret := range secretList.Items {
		if seen[secret.Name] {
			continue
		}
		if !secrets.IsTlsCredentialSecret(&secret) {
			continue
		}
		if _, ok := secret.Data["tls.crt"]; !ok {
			continue
		}
		info, err := cmd.certInfoFromSecret(secret.Name, &secret, "", "", nil)
		if err != nil {
			return nil, err
		}
		infos = append(infos, *info)
	}

	return infos, nil
}

func (cmd *CmdDebugCert) certInfoFromCR(certificate *v2alpha1.Certificate) (*certdisplay.Info, error) {
	secret, err := cmd.KubeClient.CoreV1().Secrets(cmd.Namespace).Get(context.TODO(), certificate.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret for certificate %s: %w", certificate.Name, err)
	}
	isSigning := certificate.Spec.Signing
	return cmd.certInfoFromSecret(certificate.Name, secret, string(certificate.Status.StatusType), certificate.Status.Expiration, &isSigning)
}

func (cmd *CmdDebugCert) certInfoFromSecret(name string, secret *corev1.Secret, status, crExpiration string, isSigning *bool) (*certdisplay.Info, error) {
	certData, ok := secret.Data["tls.crt"]
	if !ok || len(certData) == 0 {
		return nil, fmt.Errorf("secret %s does not contain tls.crt", name)
	}
	info, err := certdisplay.ParseCertificate(name, certData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate %s: %w", name, err)
	}
	info.Status = status
	info.CrExpiration = crExpiration
	info.IsSigningCert = isSigning
	return info, nil
}

func (cmd *CmdDebugCert) WaitUntil() error { return nil }
