package client

import (
	"github.com/skupperproject/skupper/pkg/kube"
	v13 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigMapManager struct {
	Client    *VanClient
	Namespace string
}

func (c *ConfigMapManager) GetConfigMap(name string, options *v1.GetOptions) (*v12.ConfigMap, bool, error) {
	cmCli := c.Client.KubeClient.CoreV1().ConfigMaps(c.Namespace)
	cm, err := cmCli.Get(name, *options)
	if err != nil {
		return nil, false, err
	}
	return cm, true, nil
}

func (c *ConfigMapManager) DeleteConfigMap(cm *v12.ConfigMap, options *v1.DeleteOptions) error {
	cmCli := c.Client.KubeClient.CoreV1().ConfigMaps(c.Namespace)
	return cmCli.Delete(cm.Name, options)
}

func (c *ConfigMapManager) ListConfigMaps(options *v1.ListOptions) ([]v12.ConfigMap, error) {
	cmCli := c.Client.KubeClient.CoreV1().ConfigMaps(c.Namespace)
	list, err := cmCli.List(*options)
	if list != nil {
		return list.Items, err
	}
	return nil, err
}

func (c *ConfigMapManager) CreateConfigMap(cm *v12.ConfigMap) (*v12.ConfigMap, error) {
	cmCli := c.Client.KubeClient.CoreV1().ConfigMaps(c.Namespace)
	return cmCli.Create(cm)
}

func (c *ConfigMapManager) UpdateConfigMap(cm *v12.ConfigMap) (*v12.ConfigMap, error) {
	cmCli := c.Client.KubeClient.CoreV1().ConfigMaps(c.Namespace)
	return cmCli.Update(cm)
}

func (c *ConfigMapManager) IsOwned(cm *v12.ConfigMap) bool {
	return kube.IsOwnedBySkupper(cm.ObjectMeta.OwnerReferences)
}

type DeploymentManager struct {
	Client    *VanClient
	Namespace string
}

func (d *DeploymentManager) GetDeployment(name string, options *v1.GetOptions) (*v13.Deployment, bool, error) {
	depCli := d.Client.KubeClient.AppsV1().Deployments(d.Namespace)
	dep, err := depCli.Get(name, *options)
	if err != nil {
		return nil, false, err
	}
	return dep, true, nil
}

func (d *DeploymentManager) DeleteDeployment(dep *v13.Deployment, options *v1.DeleteOptions) error {
	depCli := d.Client.KubeClient.AppsV1().Deployments(d.Namespace)
	return depCli.Delete(dep.Name, options)
}

func (d *DeploymentManager) ListDeployments(options *v1.ListOptions) ([]v13.Deployment, error) {
	depCli := d.Client.KubeClient.AppsV1().Deployments(d.Namespace)
	list, err := depCli.List(*options)
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (d *DeploymentManager) CreateDeployment(dep *v13.Deployment) (*v13.Deployment, error) {
	depCli := d.Client.KubeClient.AppsV1().Deployments(d.Namespace)
	return depCli.Create(dep)
}

func (d *DeploymentManager) UpdateDeployment(dep *v13.Deployment) (*v13.Deployment, error) {
	depCli := d.Client.KubeClient.AppsV1().Deployments(d.Namespace)
	return depCli.Update(dep)
}

func (d *DeploymentManager) IsOwned(dep *v13.Deployment) bool {
	return kube.IsOwnedBySkupper(dep.ObjectMeta.OwnerReferences)
}

type SecretManager struct {
	Client    *VanClient
	Namespace string
}

func (s *SecretManager) GetSecret(name string, options *v1.GetOptions) (*v12.Secret, bool, error) {
	secCli := s.Client.KubeClient.CoreV1().Secrets(s.Namespace)
	sec, err := secCli.Get(name, *options)
	if err != nil {
		return nil, false, err
	}
	return sec, true, nil
}

func (s *SecretManager) DeleteSecret(secret *v12.Secret, options *v1.DeleteOptions) error {
	secCli := s.Client.KubeClient.CoreV1().Secrets(s.Namespace)
	return secCli.Delete(secret.Name, options)
}

func (s *SecretManager) ListSecrets(options *v1.ListOptions) ([]v12.Secret, error) {
	secCli := s.Client.KubeClient.CoreV1().Secrets(s.Namespace)
	list, err := secCli.List(*options)
	if err != nil {
		return nil, err
	}
	return list.Items, err
}

func (s *SecretManager) CreateSecret(secret *v12.Secret) (*v12.Secret, error) {
	secCli := s.Client.KubeClient.CoreV1().Secrets(s.Namespace)
	return secCli.Create(secret)
}

func (s *SecretManager) UpdateSecret(secret *v12.Secret) (*v12.Secret, error) {
	secCli := s.Client.KubeClient.CoreV1().Secrets(s.Namespace)
	return secCli.Update(secret)
}

func (s *SecretManager) IsOwned(secret *v12.Secret) bool {
	return kube.IsOwnedBySkupper(secret.ObjectMeta.OwnerReferences)
}

type ServiceManager struct {
	Client    *VanClient
	Namespace string
}

func (s *ServiceManager) GetService(name string, options *v1.GetOptions) (*v12.Service, bool, error) {
	svcCli := s.Client.KubeClient.CoreV1().Services(s.Namespace)
	svc, err := svcCli.Get(name, *options)
	if err != nil {
		return nil, false, err
	}
	return svc, true, nil
}

func (s *ServiceManager) DeleteService(svc *v12.Service, options *v1.DeleteOptions) error {
	svcCli := s.Client.KubeClient.CoreV1().Services(s.Namespace)
	return svcCli.Delete(svc.Name, options)
}

func (s *ServiceManager) ListServices(options *v1.ListOptions) ([]v12.Service, error) {
	svcCli := s.Client.KubeClient.CoreV1().Services(s.Namespace)
	list, err := svcCli.List(*options)
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (s *ServiceManager) CreateService(svc *v12.Service) (*v12.Service, error) {
	svcCli := s.Client.KubeClient.CoreV1().Services(s.Namespace)
	return svcCli.Create(svc)
}

func (s *ServiceManager) UpdateService(svc *v12.Service) (*v12.Service, error) {
	svcCli := s.Client.KubeClient.CoreV1().Services(s.Namespace)
	return svcCli.Update(svc)
}

func (s *ServiceManager) IsOwned(service *v12.Service) bool {
	return kube.IsOwnedBySkupper(service.ObjectMeta.OwnerReferences)
}
