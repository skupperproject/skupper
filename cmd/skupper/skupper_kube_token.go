package main

import (
	"context"
	"fmt"
	"os"

	"github.com/skupperproject/skupper/api/types"
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
	if tokenTemplate != "" {
		return s.createFromTemplate(cmd, args)
	}
	cli := s.kube.Cli
	switch tokenType {
	case "cert":
		err := cli.ConnectorTokenCreateFile(context.Background(), clientIdentity, args[0])
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

func (s *SkupperKubeToken) createFromTemplate(cmd *cobra.Command, args []string) error {
	cli := s.kube.Cli
	switch tokenType {
	case "cert":
		filename := args[0]
		secret, localOnly, err := cli.ConnectorTokenCreateFromTemplate(context.Background(), clientIdentity, tokenTemplate)
		if err != nil {
			return fmt.Errorf("Failed to create token: %w", err)
		}
		s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
		out, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("Could not write to file " + filename + ": " + err.Error())
		}
		err = s.Encode(secret, out)
		if err != nil {
			return fmt.Errorf("Could not write out generated secret: " + err.Error())
		} else {
			var extra string
			if localOnly {
				extra = "(Note: token will only be valid for local cluster)"
			}
			fmt.Printf("Connection token written to %s %s", filename, extra)
			fmt.Println()
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

func (s *SkupperKubeToken) CreateFlags(cmd *cobra.Command) {}
