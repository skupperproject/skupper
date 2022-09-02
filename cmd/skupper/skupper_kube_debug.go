package main

import (
	"context"
	"fmt"
	"os"

	"github.com/skupperproject/skupper/client"
	"github.com/spf13/cobra"
)

func (s *SkupperKube) DebugEvents(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	output, err := s.Cli.SkupperEvents(verbose)
	if err != nil {
		return err
	}
	os.Stdout.Write(output.Bytes())
	return nil
}

func (s *SkupperKube) DebugService(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	output, err := s.Cli.SkupperCheckService(args[0], verbose)
	if err != nil {
		return err
	}
	os.Stdout.Write(output.Bytes())
	return nil
}

func (s *SkupperKube) DebugDump(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	file, err := s.Cli.SkupperDump(context.Background(), args[0], client.Version, s.KubeConfigPath, s.KubeContext)
	if err != nil {
		return fmt.Errorf("Unable to save skupper dump details: %w", err)
	} else {
		fmt.Println("Skupper dump details written to compressed archive: ", file)
	}
	return nil
}
