package main

import (
	"context"
	"fmt"
	"os"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/version"
	"github.com/spf13/cobra"
)

type SkupperKubeDebug struct {
	kube *SkupperKube
}

func (s *SkupperKubeDebug) NewClient(cmd *cobra.Command, args []string) {
	s.kube.NewClient(cmd, args)
}

func (s *SkupperKubeDebug) Platform() types.Platform {
	return s.kube.Platform()
}

func (s *SkupperKubeDebug) Dump(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	file, err := s.kube.Cli.SkupperDump(context.Background(), args[0], version.Version, s.kube.KubeConfigPath, s.kube.KubeContext)
	if err != nil {
		return fmt.Errorf("Unable to save skupper dump details: %w", err)
	} else {
		fmt.Println("Skupper dump details written to compressed archive: ", file)
	}
	return nil
}

func (s *SkupperKubeDebug) Events(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	output, err := s.kube.Cli.SkupperEvents(verbose)
	if err != nil {
		return err
	}
	os.Stdout.Write(output.Bytes())
	return nil
}

func (s *SkupperKubeDebug) Service(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	output, err := s.kube.Cli.SkupperCheckService(args[0], verbose)
	if err != nil {
		return err
	}
	os.Stdout.Write(output.Bytes())
	return nil
}
