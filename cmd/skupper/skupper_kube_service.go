package main

import (
	"context"
	"fmt"
	"strings"

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
	err := cli.ServiceInterfaceCreate(context.Background(), &serviceToCreate)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func (s *SkupperKube) ServiceDelete(cmd *cobra.Command, args []string) error {
	err := cli.ServiceInterfaceRemove(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func (s *SkupperKube) ServiceStatus(cmd *cobra.Command, args []string) error {
	vsis, err := cli.ServiceInterfaceList(context.Background())
	if err == nil {
		listServices(vsis, showLabels)
	} else {
		return fmt.Errorf("Could not retrieve services: %w", err)
	}
	return nil
}

func (s *SkupperKube) ServiceLabel(cmd *cobra.Command, args []string) error {
	name := args[0]
	si, err := cli.ServiceInterfaceInspect(context.Background(), name)
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
	err = cli.ServiceInterfaceUpdate(context.Background(), si)
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

	service, err := cli.ServiceInterfaceInspect(context.Background(), args[0])

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

	err = cli.ServiceInterfaceBind(context.Background(), service, targetType, targetName, protocol, portMapping)
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

	err := cli.ServiceInterfaceUnbind(context.Background(), targetType, targetName, args[0], false)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}
