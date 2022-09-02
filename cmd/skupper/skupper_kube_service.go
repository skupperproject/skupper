package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/utils/formatter"
	"github.com/spf13/cobra"
)

func (s *SkupperKube) bindArgs(cmd *cobra.Command, args []string) error {
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

func (s *SkupperKube) ServiceCreate(cmd *cobra.Command, args []string) error {
	err := s.Cli.ServiceInterfaceCreate(context.Background(), &serviceToCreate)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func (s *SkupperKube) ServiceDelete(cmd *cobra.Command, args []string) error {
	err := s.Cli.ServiceInterfaceRemove(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func (s *SkupperKube) ServiceStatus(cmd *cobra.Command, args []string) error {
	cli := s.Cli
	vsis, err := s.Cli.ServiceInterfaceList(context.Background())
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

func (s *SkupperKube) ServiceLabel(cmd *cobra.Command, args []string) error {
	name := args[0]
	si, err := s.Cli.ServiceInterfaceInspect(context.Background(), name)
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
	err = s.Cli.ServiceInterfaceUpdate(context.Background(), si)
	if err != nil {
		return fmt.Errorf("error updating service labels: %v", err)
	}
	return nil
}

func (s *SkupperKube) ServiceBind(cmd *cobra.Command, args []string) error {
	targetType, targetName := parseTargetTypeAndName(args[1:])

	if publishNotReadyAddresses && targetType == "service" {
		return fmt.Errorf("--publish-not-ready-addresses option is only valid for headless services and deployments")
	}

	service, err := s.Cli.ServiceInterfaceInspect(context.Background(), args[0])

	if err != nil {
		return fmt.Errorf("%w", err)
	} else if service == nil {
		return fmt.Errorf("Service %s not found", args[0])
	}

	// validating ports
	portMapping, err := parsePortMapping(service, targetPorts)
	if err != nil {
		return err
	}

	service.PublishNotReadyAddresses = publishNotReadyAddresses

	err = s.Cli.ServiceInterfaceBind(context.Background(), service, targetType, targetName, protocol, portMapping)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func (s *SkupperKube) ServiceBindArgs(cmd *cobra.Command, args []string) error {
	return s.bindArgs(cmd, args)
}

func (s *SkupperKube) ServiceBindFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&publishNotReadyAddresses, "publish-not-ready-addresses", false, "If specified, skupper will not wait for pods to be ready")
}

func (s *SkupperKube) ServiceUnbind(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)

	targetType, targetName := parseTargetTypeAndName(args[1:])

	err := s.Cli.ServiceInterfaceUnbind(context.Background(), targetType, targetName, args[0], false)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}
