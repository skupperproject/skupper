package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/utils/formatter"
	"github.com/spf13/cobra"
)

type SkupperKubeService struct {
	kube *SkupperKube
}

func (s *SkupperKubeService) NewClient(cmd *cobra.Command, args []string) {
	s.kube.NewClient(cmd, args)
}

func (s *SkupperKubeService) Platform() types.Platform {
	return s.kube.Platform()
}

func (s *SkupperKubeService) Create(cmd *cobra.Command, args []string) error {
	err := s.kube.Cli.ServiceInterfaceCreate(context.Background(), &serviceToCreate)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func (s *SkupperKubeService) CreateFlags(cmd *cobra.Command) {}

func (s *SkupperKubeService) Delete(cmd *cobra.Command, args []string) error {
	err := s.kube.Cli.ServiceInterfaceRemove(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func (s *SkupperKubeService) DeleteFlags(cmd *cobra.Command) {}

func (s *SkupperKubeService) List(cmd *cobra.Command, args []string) error {
	return nil
}

func (s *SkupperKubeService) ListFlags(cmd *cobra.Command) {}

func (s *SkupperKubeService) Status(cmd *cobra.Command, args []string) error {
	cli := s.kube.Cli
	vsis, err := s.kube.Cli.ServiceInterfaceList(context.Background())
	if err == nil {
		if len(vsis) == 0 {
			fmt.Println("No services defined")
		} else {
			l := formatter.NewList()
			l.Item("Services exposed through Skupper:")
			addresses := []string{}
			for _, si := range vsis {
				addresses = append(addresses, si.Address)
			}
			svcAuth := map[string]bool{}
			for _, addr := range addresses {
				svcAuth[addr] = true
			}
			if vc, ok := cli.(*client.VanClient); ok {
				policy := client.NewPolicyValidatorAPI(vc)
				res, _ := policy.Services(addresses...)
				for addr, auth := range res {
					svcAuth[addr] = auth.Allowed
				}
			}

			for _, si := range vsis {
				portStr := "port"
				if len(si.Ports) > 1 {
					portStr = "ports"
				}
				for _, port := range si.Ports {
					portStr += fmt.Sprintf(" %d", port)
				}
				authSuffix := ""
				if !svcAuth[si.Address] {
					authSuffix = " - not authorized"
				}
				svc := l.NewChild(fmt.Sprintf("%s (%s %s)%s", si.Address, si.Protocol, portStr, authSuffix))
				if len(si.Targets) > 0 {
					targets := svc.NewChild("Targets:")
					for _, t := range si.Targets {
						var name string
						if t.Name != "" {
							name = fmt.Sprintf("name=%s", t.Name)
						}
						targetInfo := ""
						if t.Selector != "" {
							targetInfo = fmt.Sprintf("%s %s", t.Selector, name)
						} else if t.Service != "" {
							targetInfo = fmt.Sprintf("%s %s", t.Service, name)
						} else {
							targetInfo = fmt.Sprintf("%s (no selector)", name)
						}
						targets.NewChild(targetInfo)
					}
				}
				if showLabels && len(si.Labels) > 0 {
					labels := svc.NewChild("Labels:")
					for k, v := range si.Labels {
						labels.NewChild(fmt.Sprintf("%s=%s", k, v))
					}
				}
			}
			l.Print()
		}
	} else {
		return fmt.Errorf("Could not retrieve services: %w", err)
	}
	return nil
}

func (s *SkupperKubeService) StatusFlags(cmd *cobra.Command) {}

func (s *SkupperKubeService) Label(cmd *cobra.Command, args []string) error {
	name := args[0]
	si, err := s.kube.Cli.ServiceInterfaceInspect(context.Background(), name)
	if si == nil {
		return fmt.Errorf("invalid service name")
	}
	if err != nil {
		return fmt.Errorf("error retrieving service: %v", err)
	}
	if showLabels {
		showServiceLabels(si, name)
		return nil
	}
	updateServiceLabels(si)
	err = s.kube.Cli.ServiceInterfaceUpdate(context.Background(), si)
	if err != nil {
		return fmt.Errorf("error updating service labels: %v", err)
	}
	return nil
}

func (s *SkupperKubeService) Bind(cmd *cobra.Command, args []string) error {
	targetType, targetName := parseTargetTypeAndName(args[1:])

	if bindOptions.PublishNotReadyAddresses && targetType == "service" {
		return fmt.Errorf("--publish-not-ready-addresses option is only valid for headless services and deployments")
	}

	service, err := s.kube.Cli.ServiceInterfaceInspect(context.Background(), args[0])

	if err != nil {
		return fmt.Errorf("%w", err)
	} else if service == nil {
		return fmt.Errorf("Service %s not found", args[0])
	}

	// validating ports
	portMapping, err := parsePortMapping(service, bindOptions.TargetPorts)
	if err != nil {
		return err
	}

	service.PublishNotReadyAddresses = bindOptions.PublishNotReadyAddresses

	err = s.kube.Cli.ServiceInterfaceBind(context.Background(), service, targetType, targetName, bindOptions.Protocol, portMapping, bindOptions.Namespace)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func (s *SkupperKubeService) BindArgs(cmd *cobra.Command, args []string) error {
	return s.bindArgs(cmd, args)
}

func (s *SkupperKubeService) BindFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&bindOptions.PublishNotReadyAddresses, "publish-not-ready-addresses", false, "If specified, skupper will not wait for pods to be ready")
	cmd.Flags().StringVar(&bindOptions.Namespace, "target-namespace", "", "Expose resources from a specific namespace")
}

func (s *SkupperKubeService) Unbind(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)

	targetType, targetName := parseTargetTypeAndName(args[1:])

	err := s.kube.Cli.ServiceInterfaceUnbind(context.Background(), targetType, targetName, args[0], false)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func (s *SkupperKubeService) bindArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 2 || (!strings.Contains(args[1], "/") && len(args) < 3) {
		return fmt.Errorf("Service name, target type and target name must all be specified (e.g. 'skupper bind <service-name> <target-type> <target-name>')")
	}
	if len(args) > 3 {
		return fmt.Errorf("illegal argument: %s", args[3])
	}
	if len(args) > 2 && strings.Contains(args[1], "/") {
		return fmt.Errorf("extra argument: %s", args[2])
	}
	return s.verifyTargetTypeFromArgs(args[1:])
}
