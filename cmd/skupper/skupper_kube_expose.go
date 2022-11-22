package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/spf13/cobra"
)

var validExposeTargetsKube = []string{"deployment", "statefulset", "pods", "service", "deploymentconfig"}

func (s *SkupperKubeService) verifyTargetTypeFromArgs(args []string) error {
	targetType, _ := parseTargetTypeAndName(args)
	if !utils.StringSliceContains(validExposeTargetsKube, targetType) {
		return fmt.Errorf("target type must be one of: [%s]", strings.Join(validExposeTargetsKube, ", "))
	}
	return nil
}

func (s *SkupperKubeService) Expose(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)

	targetType, targetName := parseTargetTypeAndName(args)

	// silence cobra may be moved below the "if" we want to print
	// the usage message along with this error
	if exposeOpts.Address == "" {
		if targetType == "service" {
			return fmt.Errorf("--address option is required for target type 'service'")
		}
		if !exposeOpts.Headless {
			exposeOpts.Address = targetName
		}
	}
	if !exposeOpts.Headless {
		if exposeOpts.ProxyTuning.Cpu != "" {
			return fmt.Errorf("--proxy-cpu option is only valid for headless services")
		}
		if exposeOpts.ProxyTuning.Memory != "" {
			return fmt.Errorf("--proxy-memory option is only valid for headless services")
		}
		if exposeOpts.ProxyTuning.CpuLimit != "" {
			return fmt.Errorf("--proxy-cpu-limit option is only valid for headless services")
		}
		if exposeOpts.ProxyTuning.MemoryLimit != "" {
			return fmt.Errorf("--proxy-memory-limit option is only valid for headless services")
		}
		if exposeOpts.ProxyTuning.Affinity != "" {
			return fmt.Errorf("--proxy-pod-affinity option is only valid for headless services")
		}
		if exposeOpts.ProxyTuning.AntiAffinity != "" {
			return fmt.Errorf("--proxy-pod-antiaffinity option is only valid for headless services")
		}
		if exposeOpts.ProxyTuning.NodeSelector != "" {
			return fmt.Errorf("--proxy-node-selector option is only valid for headless services")
		}
	}

	tlsTrustOptionIsMissing := exposeOpts.TlsCredentials != "" && exposeOpts.TlsCertAuthority == ""
	tlsCredentialsOptionIsMissing := exposeOpts.TlsCredentials == "" && exposeOpts.TlsCertAuthority != ""
	tlsWithSkupperGeneratedCerts := exposeOpts.GeneratedCerts && exposeOpts.TlsCredentials == "" && exposeOpts.TlsCertAuthority == ""
	tlsCustomCertsAndGeneratedCerts := exposeOpts.GeneratedCerts && (exposeOpts.TlsCredentials != "" || exposeOpts.TlsCertAuthority != "")

	if tlsTrustOptionIsMissing {
		return fmt.Errorf("to enable TLS with custom certs, it is necessary to specify --tls-trust option")
	}

	if tlsCredentialsOptionIsMissing {
		return fmt.Errorf("to enable TLS with custom certs, it is necessary to specify --tls-cert option")
	}

	if tlsCustomCertsAndGeneratedCerts {
		return fmt.Errorf("the option --generate-tls-secrets can not be used with custom certificates")
	}

	if tlsWithSkupperGeneratedCerts {
		exposeOpts.TlsCredentials = types.SkupperServiceCertPrefix + exposeOpts.Address
		exposeOpts.TlsCertAuthority = types.ServiceClientSecret
	}

	if exposeOpts.PublishNotReadyAddresses && targetType == "service" {
		return fmt.Errorf("--publish-not-ready-addresses option is only valid for headless services and deployments")
	}

	addr, err := expose(s.kube.Cli, context.Background(), targetType, targetName, exposeOpts)
	if err == nil {
		fmt.Printf("%s %s exposed as %s\n", targetType, targetName, addr)
	}
	return err
}

func (s *SkupperKubeService) ExposeArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 1 || (!strings.Contains(args[0], "/") && len(args) < 2) {
		return fmt.Errorf("expose target and name must be specified (e.g. 'skupper expose deployment <name>')")
	}
	if len(args) > 2 {
		return fmt.Errorf("illegal argument: %s", args[2])
	}
	if len(args) > 1 && strings.Contains(args[0], "/") {
		return fmt.Errorf("extra argument: %s", args[1])
	}
	return s.verifyTargetTypeFromArgs(args)
}

func (s *SkupperKubeService) ExposeFlags(cmd *cobra.Command) {
	cmd.Use = "expose [deployment <name>|pods <selector>|statefulset <statefulsetname>|service <name>|deploymentconfig <name>]"

	cmd.Flags().StringVar(&exposeOpts.TlsCredentials, "tls-cert", "", "K8s secret name with custom certificates to expose the service over TLS (valid only for http2 and tcp protocols)")
	cmd.Flags().StringVar(&exposeOpts.TlsCertAuthority, "tls-trust", "", "K8s secret name with the CA to expose the service over TLS (valid only for http2 and tcp protocols)")
	cmd.Flags().StringVar(&exposeOpts.BridgeImage, "bridge-image", "", "The image to use for a bridge running external to the skupper router")

	cmd.Flags().BoolVar(&(exposeOpts.Headless), "headless", false, "Expose through a headless service (valid only for a statefulset target)")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.Cpu, "proxy-cpu", "", "CPU request for router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.Memory, "proxy-memory", "", "Memory request for router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.CpuLimit, "proxy-cpu-limit", "", "CPU limit for router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.MemoryLimit, "proxy-memory-limit", "", "Memory limit for router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.NodeSelector, "proxy-node-selector", "", "Node selector to control placement of router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.Affinity, "proxy-pod-affinity", "", "Pod affinity label matches to control placement of router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.AntiAffinity, "proxy-pod-antiaffinity", "", "Pod antiaffinity label matches to control placement of router pods")
	cmd.Flags().BoolVar(&exposeOpts.PublishNotReadyAddresses, "publish-not-ready-addresses", false, "If specified, skupper will not wait for pods to be ready")
	cmd.Flags().StringVar(&exposeOpts.Namespace, "target-namespace", "", "Expose resources from a specific namespace")
}

func (s *SkupperKubeService) Unexpose(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	targetType, targetName := parseTargetTypeAndName(args)

	err := s.kube.Cli.ServiceInterfaceUnbind(context.Background(), targetType, targetName, unexposeAddress, true)
	if err == nil {
		fmt.Printf("%s %s unexposed\n", targetType, targetName)
	} else {
		return fmt.Errorf("Unable to unbind skupper service: %w", err)
	}
	return nil
}

func (s *SkupperKubeService) UnexposeFlags(cmd *cobra.Command) error {
	cmd.Use = "unexpose [deployment <name>|pods <selector>|statefulset <statefulsetname>|service <name>|deploymentconfig <name>]"
	return nil
}
