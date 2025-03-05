package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log/slog"
	"os"

	iflag "github.com/skupperproject/skupper/internal/flag"
	"github.com/skupperproject/skupper/internal/kube/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const usage string = `Usage of network-observer <subcommand>:
Intended for internal use. API is not stable.

	ensure-secret:
		Provisions kubernetes Secrets related to the execution of the
		network-observer.`
const ensureSecretCmd string = "ensure-secret"
const ensureSecretDesc string = `Creates a kubernetes secret <secret name> with randomly generated contents
based on one of the preconfigured formats when the secret does not already exist.`

func Run(args []string) {
	subcommand := args[0]
	switch subcommand {
	case "ensure-secret":
		if err := runEnsureSecret(args); err != nil {
			slog.Error("ensure secret error", slog.Any("error", err))
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown command %q\n", subcommand)
		fmt.Println(usage)
		os.Exit(1)
	}
}

type secretProvider func(name string) (*corev1.Secret, error)

func runEnsureSecret(args []string) error {
	var (
		namespace       string
		kubeconfig      string
		format          string
		secretName      string
		secretsProvider func(name string) (*corev1.Secret, error)
	)
	flags := flag.NewFlagSet(ensureSecretCmd, flag.ExitOnError)
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage of %s %s [options...] <secret name>\n", os.Args[0], args[0])
		fmt.Fprintln(flags.Output(), ensureSecretDesc)
		flags.PrintDefaults()
	}
	iflag.StringVar(flags, &namespace, "namespace", "NAMESPACE", "", "The Kubernetes namespace scope for the controller")
	iflag.StringVar(flags, &kubeconfig, "kubeconfig", "KUBECONFIG", "", "A path to the kubeconfig file to use")
	iflag.StringVar(flags, &format, "format", "ENSURE_SECRET_FORMAT", "", "Secret format. One of [oauth2-proxy-session-cookie, htpasswd]. Requried.")
	flags.Parse(args[1:])
	posArgs := flags.Args()
	if len(posArgs) != 1 {
		fmt.Fprintf(flags.Output(), "expected argument for secret name\n")
		flags.Usage()
		os.Exit(1)
	}
	secretName = posArgs[0]

	switch format {
	case "htpasswd":
		secretsProvider = generateHtpasswdSecret
	case "oauth2-proxy-session-cookie":
		secretsProvider = generateOauth2ProxySessionSecret
	case "":
		fmt.Fprintf(flags.Output(), "flag --format is requried\n")
		flags.Usage()
		os.Exit(1)
	default:
		fmt.Fprintf(flags.Output(), "format %q not supported\n", format)
		flags.Usage()
		os.Exit(1)
	}

	cli, err := client.NewClient(namespace, "", kubeconfig)
	if err != nil {
		return fmt.Errorf("error creating skupper client: %w", err)
	}
	secretsClient := cli.Kube.CoreV1().Secrets(cli.GetNamespace())

	return ensureSecret(context.Background(), secretsClient, secretName, secretsProvider)
}

func generateOauth2ProxySessionSecret(name string) (*corev1.Secret, error) {
	secretBytes := [32]byte{}
	if _, err := rand.Read(secretBytes[:]); err != nil {
		return nil, fmt.Errorf("error generating random cookie secret: %w", err)
	}
	encoded := base64.URLEncoding.EncodeToString(secretBytes[:])
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"secret": []byte(encoded),
		},
	}, nil
}

func generateHtpasswdSecret(name string) (*corev1.Secret, error) {
	const htpasswdPrefix = "skupper:{PLAIN}"
	secretBytes := [16]byte{}
	if _, err := rand.Read(secretBytes[:]); err != nil {
		return nil, fmt.Errorf("error generating random base for password: %w", err)
	}
	encoded := base64.RawStdEncoding.EncodeToString(secretBytes[:])
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"htpasswd": fmt.Append([]byte(htpasswdPrefix), encoded),
		},
	}, nil
}

func ensureSecret(ctx context.Context, client secretCreator, secretName string, provider secretProvider) error {
	secret, err := provider(secretName)
	if err != nil {
		return err
	}
	slog.Info("Ensuring cookie secret exists", slog.String("secret", secret.Name))
	if _, err := client.Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("error creating secret: %w", err)
		}
		slog.Info("Secret already exists", slog.String("secret", secret.Name))
	} else {
		slog.Info("Created secret", slog.String("secret", secret.Name))
	}
	return nil
}

type secretCreator interface {
	Create(ctx context.Context, secret *corev1.Secret, opts metav1.CreateOptions) (*corev1.Secret, error)
}
