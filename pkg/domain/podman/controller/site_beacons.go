package controller

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"time"

	"github.com/skupperproject/skupper/api/types"
	clientpodman "github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/pkg/flow"
	"github.com/skupperproject/skupper/pkg/fs"
	"github.com/skupperproject/skupper/pkg/utils"
)

//
// Podman site beacons
//

func SendPodmanHostRecord(cli *clientpodman.PodmanRestClient, site *podman.Site, origin string,
	flowController *flow.FlowController, startTime uint64) {

	var platform string = types.PlatformPodman
	versionInfo, err := cli.Version()
	if err != nil {
		log.Fatalf("error retrieving podman host info - %s", err)
	}
	siteName := site.GetName()
	host := &flow.HostRecord{}
	host.Identity = site.GetName()
	host.Parent = origin
	host.StartTime = startTime
	host.Name = &siteName
	host.EndTime = 0
	host.Platform = &platform
	host.Name = &versionInfo.Hostname
	host.Arch = &versionInfo.Arch
	host.OperatingSystem = &versionInfo.OS
	host.ContainerRuntime = &platform
	host.KernelVersion = &versionInfo.Kernel
	err = flow.UpdateHost(flowController, false, site.GetName(), host)
	if err != nil {
		log.Printf("error providing host information - %s", err)
	}
}

// ServiceTargetWatcher watches the skupper-services.json file and
// updates ProcessRecords to the collector based on service targets
// defined using IP addresses only
type ServiceTargetWatcher struct {
	site                *podman.Site
	flowController      *flow.FlowController
	addressHosts        map[string][]string
	processes           map[string]*flow.ProcessRecord
	skupperServicesFile string
}

func NewServiceTargetWatcher(site *podman.Site, flowController *flow.FlowController) *ServiceTargetWatcher {
	return &ServiceTargetWatcher{
		site:           site,
		flowController: flowController,
		addressHosts:   map[string][]string{},
		processes:      map[string]*flow.ProcessRecord{},
	}
}

func (s *ServiceTargetWatcher) getSkupperServicesFilename() string {
	return utils.DefaultStr(s.skupperServicesFile,
		path.Join(podman.ServiceInterfaceMount, podman.SkupperServicesFilename))
}

func (s *ServiceTargetWatcher) Watch(stopCh <-chan struct{}) error {
	s.addressHosts = map[string][]string{}
	s.processes = map[string]*flow.ProcessRecord{}
	w, err := fs.NewWatcher()
	if err != nil {
		return err
	}
	w.Add(s.getSkupperServicesFilename(), s)
	w.Start(stopCh)
	return nil
}

// load reads the local skupper-services file and returns a map
// of addresses and egress hosts (that are not container names)
func (s *ServiceTargetWatcher) load() map[string][]string {
	res := map[string]*types.ServiceInterface{}
	addressHosts := map[string][]string{}
	data, err := os.ReadFile(s.getSkupperServicesFilename())
	if err != nil {
		log.Printf("%s does not exist", s.getSkupperServicesFilename())
		return addressHosts
	}
	err = json.Unmarshal(data, &res)
	for addr, svcIface := range res {
		addressHosts[addr] = []string{}
		if len(svcIface.Targets) == 0 {
			continue
		}
		for _, target := range svcIface.Targets {
			egresses, err := domain.EgressResolverFromString(target.Name).Resolve()
			if err != nil {
				log.Printf("unable to resolve egresses for %s - %s", addr, err)
				continue
			}
			for _, egress := range egresses {
				host := egress.GetHost()
				if ip := net.ParseIP(host); ip != nil {
					addressHosts[addr] = append(addressHosts[addr], host)
				}
			}
		}
	}
	return addressHosts
}

func (s *ServiceTargetWatcher) processChanges() {
	newAddressHosts := s.load()
	oldAddressHosts := s.addressHosts
	added := map[string][]string{}
	deleted := map[string][]string{}
	addresses := map[string]any{}

	// added and modified services
	for addr, newHosts := range newAddressHosts {
		addresses[addr] = nil
		// new service
		if oldHosts, ok := oldAddressHosts[addr]; !ok {
			added[addr] = newHosts
		} else {
			// verify if any old host has been removed
			for _, oldHost := range oldHosts {
				if !utils.StringSliceContains(newHosts, oldHost) {
					deleted[addr] = append(deleted[addr], oldHost)
				}
			}
			// verify if any new host has been added
			for _, newHost := range newHosts {
				if !utils.StringSliceContains(oldHosts, newHost) {
					added[addr] = append(added[addr], newHost)
				}
			}
		}
	}

	// deleted services
	for addr, oldHosts := range oldAddressHosts {
		addresses[addr] = nil
		if _, ok := newAddressHosts[addr]; !ok {
			deleted[addr] = append(deleted[addr], oldHosts...)
		}
	}

	// Updating process records
	for addr, _ := range addresses {
		s.updateFlowCollector(addr, added[addr], deleted[addr])
	}
	s.addressHosts = newAddressHosts
}

func (s *ServiceTargetWatcher) updateFlowCollector(address string, added, deleted []string) {
	processes := map[string]*flow.ProcessRecord{}
	var p *flow.ProcessRecord
	var err error

	toProcessRecord := func(address, host string, deleted bool) *flow.ProcessRecord {
		id := fmt.Sprintf("%s-%s", address, host)
		if !deleted {
			p = &flow.ProcessRecord{
				Base: flow.Base{
					Identity:  id,
					Parent:    s.site.Id,
					StartTime: uint64(time.Now().UnixMicro()),
				},
				Name:        &id,
				ParentName:  &address,
				GroupName:   &address,
				HostName:    &host,
				SourceHost:  &host,
				ProcessRole: &flow.External,
			}
		} else {
			p = s.processes[id]
			p.EndTime = uint64(time.Now().UnixMicro())
		}
		return p
	}
	for _, host := range added {
		p = toProcessRecord(address, host, false)
		if err = flow.UpdateProcess(s.flowController, false, *p.Name, p); err != nil {
			log.Printf("error creating process information for: %s - %s", *p.Name, err)
		}
		processes[p.Identity] = p
	}
	for _, host := range deleted {
		p = toProcessRecord(address, host, true)
		if err = flow.UpdateProcess(s.flowController, true, *p.Name, p); err != nil {
			log.Printf("error deleting process information for: %s - %s", *p.Name, err)
		}
		delete(processes, p.Identity)
	}
	s.processes = processes
}

func (s *ServiceTargetWatcher) OnCreate(name string) {
	s.processChanges()
}

func (s *ServiceTargetWatcher) OnUpdate(name string) {
	s.processChanges()
}

func (s *ServiceTargetWatcher) OnRemove(name string) {
	s.processChanges()
}

// ContainerProcessInformer monitors podman containers, sending ProcessRecord
// instances to the flow controller
type ContainerProcessInformer struct {
	cli            *clientpodman.PodmanRestClient
	origin         string
	site           *podman.Site
	flowController *flow.FlowController
}

func NewContainerProcessInformer(cli *clientpodman.PodmanRestClient, origin string, site *podman.Site, flowController *flow.FlowController) *ContainerProcessInformer {
	return &ContainerProcessInformer{
		cli:            cli,
		origin:         origin,
		site:           site,
		flowController: flowController,
	}
}

func (p *ContainerProcessInformer) UpdateProcesses(obj *container.Container, deleted bool) {
	process := p.containerToProcess(obj)
	if err := flow.UpdateProcess(p.flowController, deleted, obj.Name, process); err != nil {
		log.Printf("error updating process record - %s [deleted: %v] - %s", obj.Name, deleted, err)
	}
}

func (p *ContainerProcessInformer) containerToProcess(cc *container.Container) *flow.ProcessRecord {
	pp := &flow.ProcessRecord{}
	var ipAddress string
	if ci, ok := cc.Networks[p.site.ContainerNetwork]; ok {
		ipAddress = ci.IPAddress
	} else {
		for _, ni := range cc.Networks {
			ipAddress = ni.IPAddress
			break
		}
	}
	groupName := utils.DefaultStr(cc.Pod, cc.Name)
	pp.Identity = cc.ID
	pp.Parent = p.origin
	pp.StartTime = uint64(cc.StartedAt.UnixMicro())
	if !cc.Running {
		pp.EndTime = uint64(cc.ExitedAt.UnixMicro())
	}
	pp.Name = &cc.Name
	pp.ImageName = &cc.Image
	pp.HostName = &groupName
	pp.GroupName = &groupName
	if ipAddress != "" {
		pp.SourceHost = &ipAddress
	}
	_, isServiceContainer := cc.Labels[types.AddressQualifier]
	if container.IsOwnedBySkupper(cc.Labels) && !isServiceContainer {
		pp.ProcessRole = &flow.Internal
	} else {
		pp.ProcessRole = &flow.External
	}
	return pp
}

func (p *ContainerProcessInformer) OnAdd(obj *container.Container) {
	p.UpdateProcesses(obj, false)
}

func (p *ContainerProcessInformer) OnUpdate(oldObj, newObj *container.Container) {
	p.UpdateProcesses(newObj, false)
}

func (p *ContainerProcessInformer) OnDelete(obj *container.Container) {
	p.UpdateProcesses(obj, true)
}
