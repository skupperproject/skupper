package main

import (
	"fmt"
	"os"

	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
)

var notImplementedErr = fmt.Errorf("Not implemented")

var SkupperPodmanCommands = []string{
	"switch",
}

type SkupperPodman struct {
}

func (s *SkupperPodman) notImplementedExit() {
	fmt.Println("Not implemented")
	os.Exit(1)
}

func (s *SkupperPodman) NewClient(cmd *cobra.Command, args []string) {
	s.notImplementedExit()
}

func (s *SkupperPodman) Platform() types.Platform {
	return types.PlatformPodman
}

func (s *SkupperPodman) Delete(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) Update(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) Status(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) Expose(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) ExposeArgs(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) ExposeFlags(cmd *cobra.Command) {
}

func (s *SkupperPodman) Unexpose(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) ServiceCreate(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) ServiceDelete(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) ServiceStatus(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) ServiceLabel(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) ServiceBind(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) ServiceBindArgs(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) ServiceBindFlags(cmd *cobra.Command) {
}

func (s *SkupperPodman) ServiceUnbind(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) Version(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) DebugEvents(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) DebugService(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) ListConnectors(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) LinkCreate(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) LinkDelete(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) LinkStatus(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) TokenCreate(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) RevokeAccess(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) NetworkStatus(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) SupportedCommands() []string {
	return SkupperPodmanCommands
}

func (s *SkupperPodman) Options(cmd *cobra.Command) {
}

func (s *SkupperPodman) Init(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}

func (s *SkupperPodman) InitFlags(cmd *cobra.Command) {
}

func (s *SkupperPodman) DebugDump(cmd *cobra.Command, args []string) error {
	s.notImplementedExit()
	return notImplementedErr
}
