package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube/tokens"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

type SkupperKubeToken struct {
	kube *SkupperKube
}

func (s *SkupperKubeToken) NewClient(cmd *cobra.Command, args []string) {
	s.kube.NewClient(cmd, args)
}

func (s *SkupperKubeToken) Platform() types.Platform {
	return s.kube.Platform()
}

func (s *SkupperKubeToken) Create(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	filename := args[0]
	out, err := getOutput(filename)
	if err != nil {
		return fmt.Errorf("Could not create file %s: %w", filename, err)
	}
	if tokenTemplate != "" {
		return s.createFromTemplate(filename, out)
	}
	cli := s.kube.Cli
	generator, err := tokens.NewTokenGenerator(s.kube.Cli.GetNamespace(), s.kube.Cli)
	if err != nil {
		return err
	}
	switch tokenType {
	case "cert":
		token := generator.NewCertToken(clientIdentity, clientIdentity)
		err = token.Write(out)
		if err != nil {
			return fmt.Errorf("Failed to create token: %w", err)
		}
		return nil
	case "claim":
		name := clientIdentity
		if name == "skupper" {
			name = ""
		}
		if password == "" {
			password = utils.RandomId(24)
		}
		err := cli.TokenClaimCreateFile(context.Background(), name, []byte(password), expiry, uses, args[0])
		if err != nil {
			return fmt.Errorf("Failed to create token: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("invalid token type. Specify cert or claim")
	}
}

func (s *SkupperKubeToken) createFromTemplate(filename string, out io.Writer) error {
	cli := s.kube.Cli
	switch tokenType {
	case "cert":
		secret, localOnly, err := cli.ConnectorTokenCreateFromTemplate(context.Background(), clientIdentity, tokenTemplate)
		if err != nil {
			return fmt.Errorf("Failed to create token: %w", err)
		}
		s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
		err = s.Encode(secret, out)
		if err != nil {
			return fmt.Errorf("Could not write out generated secret: " + err.Error())
		} else {
			if out != os.Stdout {
				fmt.Printf("Connection token written to %s", filename)
				if localOnly {
					fmt.Printf(" (Note: token will only be valid for local cluster)")
				}
				fmt.Println()
			}
			return nil
		}
		return nil
	case "claim":
		return fmt.Errorf("--template option cannot be used for a claim")
	default:
		return fmt.Errorf("invalid token type.")
	}
	return nil
}

func getOutput(filename string) (io.Writer, error) {
	if filename == "-" {
		return os.Stdout, nil
	}
	return os.Create(filename)
}

func (s *SkupperKubeToken) CreateFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&tokenType, "token-type", "t", "claim", "Type of token to create. Valid options are 'claim' or 'cert'")
	cmd.Flags().StringVarP(&password, "password", "p", "", "A password for the claim (only valid if --token-type=claim). If not specified one will be generated.")
	cmd.Flags().DurationVarP(&expiry, "expiry", "", 15*time.Minute, "Expiration time for claim (only valid if --token-type=claim)")
	cmd.Flags().IntVarP(&uses, "uses", "", 1, "Number of uses for which claim will be valid (only valid if --token-type=claim)")
	cmd.Flags().StringVarP(&tokenTemplate, "template", "", "", "The name of a secret used as a template for the token")
	f := cmd.Flag("template")
	f.Hidden = true
}
