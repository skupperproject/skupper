package main

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
)

var SkupperKubeCommands = []string{
	"init", "delete", "update", "connection-token", "token", "link", "connect", "disconnect",
	"check-connection", "status", "list-connectors", "expose", "unexpose", "list-exposed",
	"service", "bind", "unbind", "version", "debug", "completion", "gateway", "revoke-access",
	"network", "switch",
}

type SkupperKube struct {
	Cli            types.VanClientInterface
	KubeContext    string
	Namespace      string
	KubeConfigPath string
	site           *SkupperKubeSite
	service        *SkupperKubeService
	debug          *SkupperKubeDebug
	link           *SkupperKubeLink
	token          *SkupperKubeToken
	network        *SkupperKubeNetwork
	common         SkupperClientCommon
}

func (s *SkupperKube) Site() SkupperSiteClient {
	if s.site == nil {
		s.site = &SkupperKubeSite{kube: s}
	}
	return s.site
}

func (s *SkupperKube) Service() SkupperServiceClient {
	if s.service == nil {
		s.service = &SkupperKubeService{kube: s}
	}
	return s.service
}

func (s *SkupperKube) Debug() SkupperDebugClient {
	if s.debug == nil {
		s.debug = &SkupperKubeDebug{kube: s}
	}
	return s.debug
}

func (s *SkupperKube) Link() SkupperLinkClient {
	if s.link == nil {
		s.link = &SkupperKubeLink{kube: s}
	}
	return s.link
}

func (s *SkupperKube) Token() SkupperTokenClient {
	if s.token == nil {
		s.token = &SkupperKubeToken{kube: s}
	}
	return s.token
}

func (s *SkupperKube) Network() SkupperNetworkClient {
	if s.network == nil {
		s.network = &SkupperKubeNetwork{kube: s}
	}
	return s.network
}

func (s *SkupperKube) SupportedCommands() []string {
	return SkupperKubeCommands
}

func (s *SkupperKube) Platform() types.Platform {
	return types.PlatformKubernetes
}

func (s *SkupperKube) Options(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().StringVarP(&s.KubeConfigPath, "kubeconfig", "", "", "Path to the kubeconfig file to use")
	rootCmd.PersistentFlags().StringVarP(&s.KubeContext, "context", "c", "", "The kubeconfig context to use")
	rootCmd.PersistentFlags().StringVarP(&s.Namespace, "namespace", "n", "", "The Kubernetes namespace to use")
}

type kubeInit struct {
	annotations                  []string
	ingressAnnotations           []string
	routerServiceAnnotations     []string
	controllerServiceAnnotations []string
}

func (s *SkupperKube) NewClient(cmd *cobra.Command, args []string) {
	if s.common != nil {
		s.common.NewClient(cmd, args)
		return
	}
	exitOnError := true
	if cmd.Name() == "version" {
		exitOnError = false
	}
	s.Cli = NewClientHandleError(s.Namespace, s.KubeContext, s.KubeConfigPath, exitOnError)
}
