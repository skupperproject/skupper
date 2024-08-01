/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kube

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/service"
	"github.com/skupperproject/skupper/pkg/utils"
)

func GetApplicationSelector(service *corev1.Service) string {
	if HasRouterSelector(*service) {
		selector := map[string]string{}
		for key, value := range service.Spec.Selector {
			if key != types.ComponentAnnotation && !(key == "application" && value == "skupper-router") {
				selector[key] = value
			}
		}
		return utils.StringifySelector(selector)
	} else {
		return utils.StringifySelector(service.Spec.Selector)
	}
}

func HasRouterSelector(service corev1.Service) bool {
	value, ok := service.Spec.Selector[types.ComponentAnnotation]
	return ok && value == types.RouterComponent
}

func GetOriginalPorts(service *corev1.Service) string {
	if service.ObjectMeta.Annotations != nil {
		if _, ok := service.ObjectMeta.Annotations[types.OriginalTargetPortQualifier]; ok {
			return ""
		}
	}
	return PortsAsString(service.Spec.Ports)
}

type Services interface {
	GetService(name string) (*corev1.Service, bool, error)
	DeleteService(svc *corev1.Service) error
	CreateService(svc *corev1.Service) error
	UpdateService(svc *corev1.Service) error
	IsOwned(service *corev1.Service) bool
}

type ServiceIngressAlways struct {
	s        Services
	selector map[string]string
}

type IsOwned func(*corev1.Service) bool

type ServiceIngressNever struct {
	s       Services
	isowned IsOwned
}

type ServiceIngressHeadlessInOrigin struct {
	s Services
}

type ServiceIngressHeadlessRemote struct {
	s Services
}

func NewHeadlessServiceIngress(s Services, origin string) service.ServiceIngress {
	if types.IsOfLocalOrigin(origin) {
		return &ServiceIngressHeadlessInOrigin{
			s: s,
		}
	} else {
		return &ServiceIngressHeadlessRemote{
			s: s,
		}
	}
}

func NewServiceIngressExternalBridge(s Services, address string) service.ServiceIngress {
	return &ServiceIngressAlways{
		s: s,
		selector: map[string]string{
			"skupper.io/external-bridge": address,
		},
	}
}

func NewServiceIngressAlways(s Services) service.ServiceIngress {
	return &ServiceIngressAlways{
		s:        s,
		selector: GetLabelsForRouter(),
	}
}

func NewServiceIngressNever(s Services, isowned IsOwned) service.ServiceIngress {
	return &ServiceIngressNever{
		s:       s,
		isowned: isowned,
	}
}

func (si *ServiceIngressAlways) Mode() types.ServiceIngressMode {
	return types.ServiceIngressModeAlways
}

func (si *ServiceIngressNever) Mode() types.ServiceIngressMode {
	return types.ServiceIngressModeNever
}

func (si *ServiceIngressAlways) Matches(def *types.ServiceInterface) bool {
	return def.Headless == nil && (def.ExposeIngress == "" || def.ExposeIngress == types.ServiceIngressModeAlways)
}

func (si *ServiceIngressNever) Matches(def *types.ServiceInterface) bool {
	return def.Headless == nil && def.ExposeIngress == types.ServiceIngressModeNever
}

func (si *ServiceIngressAlways) create(desired *service.ServiceBindings) error {
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: desired.Address,
			Annotations: map[string]string{
				"internal.skupper.io/controlled": "true",
			},
			Labels: desired.Labels,
		},
		Spec: corev1.ServiceSpec{
			PublishNotReadyAddresses: desired.PublishNotReadyAddresses,
		},
	}
	for key, value := range desired.Annotations {
		service.ObjectMeta.Annotations[key] = value
	}
	UpdatePorts(&service.Spec, desired.PortMap(), protocol(desired.Protocol()))
	UpdateSelectorFromMap(&service.Spec, si.selector)

	return si.s.CreateService(service)
}

func (si *ServiceIngressAlways) update(actual *corev1.Service, desired *service.ServiceBindings) error {
	originalPorts := GetOriginalPorts(actual)
	originalSelector := GetApplicationSelector(actual)

	updatedPorts := UpdatePorts(&actual.Spec, desired.PortMap(), protocol(desired.Protocol()))
	updatedSelector := UpdateSelectorFromMap(&actual.Spec, si.selector)
	updatedLabels := UpdateLabels(&actual.ObjectMeta, desired.Labels)
	updatedAnnotations := UpdateAnnotations(&actual.ObjectMeta, desired.Annotations)

	if updatedPorts && !si.s.IsOwned(actual) && originalPorts != "" {
		SetAnnotation(&actual.ObjectMeta, types.OriginalTargetPortQualifier, originalPorts)
		SetAnnotation(&actual.ObjectMeta, types.OriginalAssignedQualifier, PortsAsString(actual.Spec.Ports))
	}
	if updatedSelector && originalSelector != "" {
		SetAnnotation(&actual.ObjectMeta, types.OriginalSelectorQualifier, originalSelector)
	}

	if !(updatedPorts || updatedSelector || updatedLabels || updatedAnnotations) {
		return nil //nothing changed
	}
	return si.s.UpdateService(actual)
}

func (si *ServiceIngressAlways) Realise(desired *service.ServiceBindings) error {
	actual, exists, err := si.s.GetService(desired.Address)
	if err != nil {
		return err
	}
	if !exists {
		return si.create(desired)
	}
	return si.update(actual, desired)
}

func (si *ServiceIngressNever) Realise(desired *service.ServiceBindings) error {
	actual, exists, err := si.s.GetService(desired.Address)
	if err != nil {
		return err
	}
	if exists && si.isowned(actual) {
		return si.s.DeleteService(actual)
	}
	return nil
}

func (si *ServiceIngressHeadlessInOrigin) Mode() types.ServiceIngressMode {
	return ""
}

func (si *ServiceIngressHeadlessInOrigin) Matches(def *types.ServiceInterface) bool {
	return def.Headless != nil && def.Origin == ""
}

func (si *ServiceIngressHeadlessInOrigin) Realise(desired *service.ServiceBindings) error {
	return nil
}

func (si *ServiceIngressHeadlessRemote) Mode() types.ServiceIngressMode {
	return ""
}

func (si *ServiceIngressHeadlessRemote) Matches(def *types.ServiceInterface) bool {
	return def.Headless != nil && def.Origin != ""
}

func (si *ServiceIngressHeadlessRemote) Realise(desired *service.ServiceBindings) error {
	actual, exists, err := si.s.GetService(desired.Address)
	if err != nil {
		return err
	}
	if !exists {
		return si.create(desired)
	}
	return si.update(actual, desired)
}

func (si *ServiceIngressHeadlessRemote) create(desired *service.ServiceBindings) error {
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: desired.Address,
			Annotations: map[string]string{
				"internal.skupper.io/controlled": "true",
			},
			Labels: desired.Labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP:                "None",
			PublishNotReadyAddresses: desired.PublishNotReadyAddresses,
		},
	}
	UpdatePorts(&service.Spec, desired.PortMap(), protocol(desired.Protocol()))
	UpdateSelectorFromMap(&service.Spec, map[string]string{"internal.skupper.io/service": desired.Address})
	return si.s.CreateService(service)
}

func (si *ServiceIngressHeadlessRemote) update(actual *corev1.Service, desired *service.ServiceBindings) error {
	updatedPorts := UpdatePorts(&actual.Spec, desired.PortMap(), protocol(desired.Protocol()))
	updatedLabels := UpdateLabels(&actual.ObjectMeta, desired.Labels)
	updatedAnnotations := UpdateAnnotations(&actual.ObjectMeta, desired.Annotations)

	if !(updatedPorts || updatedLabels || updatedAnnotations) {
		return nil //nothing changed
	}
	return si.s.UpdateService(actual)
}
