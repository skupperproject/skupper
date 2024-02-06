package main

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/pkg/network"
	"github.com/skupperproject/skupper/pkg/utils"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/api/types"
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

	configSyncVersion := utils.GetVersionTag(cli.GetVersion(types.TransportContainerName, types.ConfigSyncContainerName))
	if configSyncVersion != "" && !utils.IsValidFor(configSyncVersion, network.MINIMUM_VERSION) {
		fmt.Printf(network.MINIMUM_VERSION_MESSAGE, configSyncVersion, network.MINIMUM_VERSION)
		fmt.Println()
		return nil
	}

	currentNetworkStatus, err := cli.NetworkStatus(context.Background())
	if err != nil && strings.Contains(err.Error(), "Skupper is not installed") {
		fmt.Printf("Skupper is not enabled in namespace: %s \n", cli.GetNamespace())
		return nil
	} else if err != nil && err.Error() == "status not ready" {
		fmt.Println("Status pending...")
		return nil
	} else if err != nil {
		return fmt.Errorf("Could not retrieve services: %w", err)
	}

	vsis, err := s.kube.Cli.ServiceInterfaceList(context.Background())
	statusManager := network.SkupperStatus{
		NetworkStatus: currentNetworkStatus,
	}

	mapServiceSites := statusManager.GetServiceSitesMap()
	mapSiteTarget := statusManager.GetSiteTargetMap()

	var mapServiceLabels map[string]map[string]string
	if err == nil {
		mapServiceLabels = getServiceLabelsMap(vsis)
	}

	if len(currentNetworkStatus.Addresses) == 0 {
		fmt.Println("No services defined")
	} else {
		l := formatter.NewList()
		l.Item("Services exposed through Skupper:")

		for _, si := range currentNetworkStatus.Addresses {
			svc := l.NewChild(fmt.Sprintf("%s (%s)", si.Name, si.Protocol))

			if verboseServiceStatus {
				sites := svc.NewChild("Sites:")

				if mapServiceSites[si.Name] != nil {
					for _, site := range mapServiceSites[si.Name] {
						item := site.Site.Identity + "(" + site.Site.Namespace + ")\n"
						policy := "-"
						if len(site.Site.Policy) > 0 {
							policy = site.Site.Policy
						}
						theSite := sites.NewChildWithDetail(item, map[string]string{"policy": policy})

						if si.ConnectorCount > 0 {
							t := mapSiteTarget[site.Site.Identity][si.Name]

							if len(t.Address) > 0 {
								targets := theSite.NewChild("Targets:")
								var name string
								if t.Target != "" {
									name = fmt.Sprintf("name=%s", t.Target)
								}
								targetInfo := fmt.Sprintf("%s %s", t.Address, name)
								targets.NewChild(targetInfo)
							}
						}
					}
				}
			}

			if showLabels && len(mapServiceLabels[si.Name]) > 0 {
				labels := svc.NewChild("Labels:")
				for k, v := range mapServiceLabels[si.Name] {
					labels.NewChild(fmt.Sprintf("%s=%s", k, v))
				}
			}
		}
		l.Print()
	}

	return nil
}

func (s *SkupperKubeService) StatusFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&verboseServiceStatus, "verbose", "v", false, "more detailed output")
}

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

	if bindOptions.Namespace != "" && targetType == "service" {
		return targetTypeServiceTargetNamespaceError()
	}

	if bindOptions.Headless && targetType != "statefulset" {
		return fmt.Errorf("--headless option is only valid for statefulsets")
	}

	if !bindOptions.Headless {
		if bindOptions.ProxyTuning.Cpu != "" {
			return fmt.Errorf("--proxy-cpu option is only valid for binding statefulsets using headless services")
		}
		if bindOptions.ProxyTuning.Memory != "" {
			return fmt.Errorf("--proxy-memory option is only valid for binding statefulsets using headless services")
		}
		if bindOptions.ProxyTuning.CpuLimit != "" {
			return fmt.Errorf("--proxy-cpu-limit option is only valid for binding statefulsets using headless services")
		}
		if bindOptions.ProxyTuning.MemoryLimit != "" {
			return fmt.Errorf("--proxy-memory-limit option is only valid for binding statefulsets using headless services")
		}
		if bindOptions.ProxyTuning.Affinity != "" {
			return fmt.Errorf("--proxy-pod-affinity option is only valid for binding statefulsets using headless services")
		}
		if bindOptions.ProxyTuning.AntiAffinity != "" {
			return fmt.Errorf("--proxy-pod-antiaffinity option is only valid for binding statefulsets using headless services")
		}
		if bindOptions.ProxyTuning.NodeSelector != "" {
			return fmt.Errorf("--proxy-node-selector option is only valid for binding statefulsets using headless services")
		}
	}

	service, err := s.kube.Cli.ServiceInterfaceInspect(context.Background(), args[0])

	if err != nil {
		return fmt.Errorf("%w", err)
	} else if service == nil {
		return fmt.Errorf("Service %s not found", args[0])
	}

	if targetType == "statefulset" {

		if bindOptions.Headless {
			err := s.kube.Cli.ServiceInterfaceRemove(context.Background(), service.Address)
			if err != nil {
				return err
			}

			service, err = s.kube.Cli.GetHeadlessServiceConfiguration(targetName, service.Protocol, service.Address, service.Ports, bindOptions.PublishNotReadyAddresses, bindOptions.Namespace)
			if err != nil {
				return err
			}

			err = configureHeadlessProxy(service.Headless, &bindOptions.ProxyTuning)

			return s.kube.Cli.ServiceInterfaceUpdate(context.Background(), service)
		}

	}

	// validating ports
	portMapping, err := parsePortMapping(service, bindOptions.TargetPorts)
	if err != nil {
		return err
	}

	service.PublishNotReadyAddresses = bindOptions.PublishNotReadyAddresses

	service.TlsCertAuthority = bindOptions.tlsCertAuthority

	err = s.kube.Cli.ServiceInterfaceBind(context.Background(), service, targetType, targetName, portMapping, bindOptions.Namespace)
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
	cmd.Flags().BoolVar(&bindOptions.Headless, "headless", false, "Bind through a headless service (valid only for a statefulset target)")
	cmd.Flags().StringVar(&bindOptions.ProxyTuning.Cpu, "proxy-cpu", "", "CPU request for router pods")
	cmd.Flags().StringVar(&bindOptions.ProxyTuning.Memory, "proxy-memory", "", "Memory request for router pods")
	cmd.Flags().StringVar(&bindOptions.ProxyTuning.CpuLimit, "proxy-cpu-limit", "", "CPU limit for router pods")
	cmd.Flags().StringVar(&bindOptions.ProxyTuning.MemoryLimit, "proxy-memory-limit", "", "Memory limit for router pods")
	cmd.Flags().StringVar(&bindOptions.ProxyTuning.NodeSelector, "proxy-node-selector", "", "Node selector to control placement of router pods")
	cmd.Flags().StringVar(&bindOptions.ProxyTuning.Affinity, "proxy-pod-affinity", "", "Pod affinity label matches to control placement of router pods")
	cmd.Flags().StringVar(&bindOptions.ProxyTuning.AntiAffinity, "proxy-pod-antiaffinity", "", "Pod antiaffinity label matches to control placement of router pods")
}

func (s *SkupperKubeService) Unbind(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)

	targetType, targetName := parseTargetTypeAndName(args[1:])

	if unbindNamespace != "" && targetType == "service" {
		return targetTypeServiceTargetNamespaceError()
	}

	err := s.kube.Cli.ServiceInterfaceUnbind(context.Background(), targetType, targetName, args[0], false, unbindNamespace)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func (s *SkupperKubeService) UnbindFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&unbindNamespace, "target-namespace", "", "Target namespace for exposed resource")
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

func getServiceLabelsMap(services []*types.ServiceInterface) map[string]map[string]string {

	mapServiceLabels := make(map[string]map[string]string)

	for _, svc := range services {
		if svc.Labels != nil {
			for _, port := range svc.Ports {
				serviceName := svc.Address + ":" + strconv.Itoa(port)
				mapServiceLabels[serviceName] = svc.Labels
			}
		}

	}

	return mapServiceLabels
}
