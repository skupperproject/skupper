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

type ServiceIngressAlways struct {
	s Services
}

type ServiceIngressHeadlessInOrigin struct {
	s Services
}

type ServiceIngressHeadlessRemote struct {
	s Services
}

func NewHeadlessServiceIngress(s Services, origin string) service.ServiceIngress {
	if origin == "" {
		return &ServiceIngressHeadlessInOrigin{
			s: s,
		}
	} else {
		return &ServiceIngressHeadlessRemote{
			s: s,
		}
	}
}

func NewServiceIngressAlways(s Services) service.ServiceIngress {
	return &ServiceIngressAlways{
		s: s,
	}
}

func (si *ServiceIngressAlways) Matches(def *types.ServiceInterface) bool {
	return def.Headless == nil
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
	UpdatePorts(&service.Spec, desired.PortMap())
	UpdateSelectorFromMap(&service.Spec, GetLabelsForRouter())

	_, err := si.s.CreateService(service)
	return err
}

func (si *ServiceIngressAlways) update(actual *corev1.Service, desired *service.ServiceBindings) error {
	originalPorts := PortsAsString(actual.Spec.Ports)
	originalSelector := GetApplicationSelector(actual)

	updatedPorts := UpdatePorts(&actual.Spec, desired.PortMap())
	updatedSelector := UpdateSelectorFromMap(&actual.Spec, GetLabelsForRouter())
	updatedLabels := UpdateLabels(&actual.ObjectMeta, desired.Labels)

	if updatedPorts && !si.s.IsOwnedService(actual) {
		SetAnnotation(&actual.ObjectMeta, types.OriginalTargetPortQualifier, originalPorts)
		SetAnnotation(&actual.ObjectMeta, types.OriginalAssignedQualifier, PortsAsString(actual.Spec.Ports))
	}
	if updatedSelector && originalSelector != "" {
		SetAnnotation(&actual.ObjectMeta, types.OriginalSelectorQualifier, originalSelector)
	}

	if !(updatedPorts || updatedSelector || updatedLabels) {
		return nil // nothing changed
	}
	_, err := si.s.UpdateService(actual)
	return err
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

func (si *ServiceIngressHeadlessInOrigin) Matches(def *types.ServiceInterface) bool {
	return def.Headless != nil && def.Origin == ""
}

func (si *ServiceIngressHeadlessInOrigin) Realise(desired *service.ServiceBindings) error {
	return nil
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
	UpdatePorts(&service.Spec, desired.PortMap())
	UpdateSelectorFromMap(&service.Spec, map[string]string{"internal.skupper.io/service": desired.Address})
	_, err := si.s.CreateService(service)
	return err
}

func (si *ServiceIngressHeadlessRemote) update(actual *corev1.Service, desired *service.ServiceBindings) error {
	updatedPorts := UpdatePorts(&actual.Spec, desired.PortMap())
	updatedLabels := UpdateLabels(&actual.ObjectMeta, desired.Labels)

	if !(updatedPorts || updatedLabels) {
		return nil // nothing changed
	}
	_, err := si.s.UpdateService(actual)
	return err
}
