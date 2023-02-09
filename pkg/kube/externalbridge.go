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
	"context"
	jsonencoding "encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/service"
)

type Deployments interface {
	GetDeployment(name string) (*appsv1.Deployment, bool, error)
	CreateDeployment(d *appsv1.Deployment) error
	UpdateDeployment(d *appsv1.Deployment) error
}

type ExternalBridge struct {
	deployments Deployments
	definition  *types.ServiceInterface
	name        string
	image       string
}

func NewExternalBridge(deployments Deployments, def *types.ServiceInterface) service.ExternalBridge {
	return &ExternalBridge{
		deployments: deployments,
		definition:  def,
		name:        "skupper-" + def.Address + "-bridge",
		image:       getBridgeImage(def),
	}
}

func (b *ExternalBridge) Matches(def *types.ServiceInterface) bool {
	return reflect.DeepEqual(def, b.definition)
}

func (b *ExternalBridge) Realise() error {
	dep, exists, err := b.deployments.GetDeployment(b.name)
	if err != nil {
		return err
	}
	if exists {
		if b.update(dep) {
			return b.deployments.UpdateDeployment(dep)
		}
	} else {
		return b.deployments.CreateDeployment(b.deployment())
	}
	return nil
}

func (b *ExternalBridge) getDefinitionAsString() (string, error) {
	encoded, err := jsonencoding.Marshal(b.definition)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (b *ExternalBridge) deployment() *appsv1.Deployment {
	var replicas int32
	replicas = 1
	labels := map[string]string{
		"skupper.io/external-bridge": b.definition.Address,
	}
	annotations := map[string]string{} //TODO: ???
	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   b.name,
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: types.ControllerServiceAccountName,
					Containers:         b.containers(),
					Volumes:            b.volumes(),
				},
			},
		},
	}
	return dep
}

func (b *ExternalBridge) volumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: types.LocalClientSecret,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: types.LocalClientSecret,
				},
			},
		},
	}
}

func (b *ExternalBridge) containers() []corev1.Container {
	return []corev1.Container{
		{
			Image: b.image,
			Name:  "bridge",
			LivenessProbe: &corev1.Probe{
				InitialDelaySeconds: 60,
				Handler: corev1.Handler{
					HTTPGet: &corev1.HTTPGetAction{
						Port: intstr.FromInt(b.livenessPort()),
						Path: "/healthz",
					},
				},
			},
			Env:   b.envVar(),
			Ports: b.ports(),
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      types.LocalClientSecret,
					MountPath: "/etc/messaging",
				},
			},
		},
	}
}

func (b *ExternalBridge) envVar() []corev1.EnvVar {
	value, _ := b.getDefinitionAsString()
	return []corev1.EnvVar{
		{
			Name:  "SKUPPER_SERVICE_DEFINITION",
			Value: value,
		},
	}
}

func (b *ExternalBridge) ports() []corev1.ContainerPort {
	ports := []corev1.ContainerPort{
		{
			Name:          "healthz",
			ContainerPort: int32(b.livenessPort()),
		},
	}
	for _, port := range b.definition.Ports {
		ports = append(ports, corev1.ContainerPort{
			Name:          "port" + strconv.Itoa(port),
			Protocol:      protocol(b.definition.Protocol),
			ContainerPort: int32(port),
		})
	}
	return ports
}

func (b *ExternalBridge) livenessPort() int {
	port := 9090
	for {
		if b.portInUse(port) {
			port = port + 1
		} else {
			break
		}
	}
	return port
}

func (b *ExternalBridge) portInUse(port int) bool {
	for _, p := range b.definition.Ports {
		if port == p {
			return true
		}
	}
	return false
}

func (b *ExternalBridge) needsUpdate(dep *appsv1.Deployment) bool {
	if len(dep.Spec.Template.Spec.Containers) != 1 {
		return true
	}
	actual := dep.Spec.Template.Spec.Containers[0]
	desired := b.containers()[0]
	if !reflect.DeepEqual(actual.Env, desired.Env) ||
		!reflect.DeepEqual(actual.Ports, desired.Ports) ||
		!reflect.DeepEqual(actual.LivenessProbe, desired.LivenessProbe) ||
		!reflect.DeepEqual(actual.VolumeMounts, desired.VolumeMounts) ||
		actual.Image != desired.Image {

		return true
	}
	if !reflect.DeepEqual(dep.Spec.Template.Spec.Volumes, b.volumes()) {
		return true
	}
	return false
}

func (b *ExternalBridge) update(dep *appsv1.Deployment) bool {
	if !b.needsUpdate(dep) {
		return false
	}
	dep.Spec.Template.Spec.Containers = b.containers()
	dep.Spec.Template.Spec.Volumes = b.volumes()
	return true
}

type CachedDeploymentsImpl struct {
	namespace string
	client    kubernetes.Interface
	informer  cache.SharedIndexInformer
	ownerRefs []metav1.OwnerReference
}

func NewDeployments(client kubernetes.Interface, namespace string, informer cache.SharedIndexInformer, ownerRefs []metav1.OwnerReference) Deployments {
	return &CachedDeploymentsImpl{
		namespace: namespace,
		client:    client,
		informer:  informer,
		ownerRefs: ownerRefs,
	}
}

func (i *CachedDeploymentsImpl) GetDeployment(name string) (*appsv1.Deployment, bool, error) {
	obj, exists, err := i.informer.GetStore().GetByKey(i.namespaced(name))
	if err != nil {
		return nil, false, err
	} else if !exists {
		return nil, false, nil
	}
	dep, ok := obj.(*appsv1.Deployment)
	if ok {
		return dep, true, nil
	}
	return nil, true, fmt.Errorf("Invalid type for %s, expected Deployment got %v", name, obj)
}

func (i *CachedDeploymentsImpl) CreateDeployment(d *appsv1.Deployment) error {
	d.ObjectMeta.OwnerReferences = i.ownerRefs
	_, err := i.client.AppsV1().Deployments(i.namespace).Create(context.TODO(), d, metav1.CreateOptions{})
	return err
}

func (i *CachedDeploymentsImpl) UpdateDeployment(d *appsv1.Deployment) error {
	_, err := i.client.AppsV1().Deployments(i.namespace).Update(context.TODO(), d, metav1.UpdateOptions{})
	return err
}

func (i *CachedDeploymentsImpl) namespaced(name string) string {
	return i.namespace + "/" + name
}

const (
	SkupperImageRegistryEnvKey string = "SKUPPER_IMAGE_REGISTRY"
	DefaultImageRegistry       string = "quay.io/skupper"
)

func getImageRegistry() string {
	imageRegistry := os.Getenv(SkupperImageRegistryEnvKey)
	if imageRegistry == "" {
		return DefaultImageRegistry
	}
	return imageRegistry
}

func getDefaultBridgeImage(protocol string) string {
	return getImageRegistry() + "/skupper-" + protocol + "-bridge"
}

func getBridgeImage(def *types.ServiceInterface) string {
	if def.BridgeImage == "" {
		return getDefaultBridgeImage(def.Protocol)
	}
	return def.BridgeImage
}
