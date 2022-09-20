package main

import (
	"context"
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/spf13/cobra"
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

func (s *SkupperKubeToken) CreateFlags(cmd *cobra.Command) {}
