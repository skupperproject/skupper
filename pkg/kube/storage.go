package kube

import (
	v13 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ConfigMapManager struct {
	KubeClient kubernetes.Interface
	Namespace  string
}

func (c *ConfigMapManager) GetConfigMap(name string) (*v12.ConfigMap, bool, error) {
	cmCli := c.KubeClient.CoreV1().ConfigMaps(c.Namespace)
	cm, err := cmCli.Get(name, v1.GetOptions{})
	if err != nil {
		return nil, false, err
	}
	return cm, true, nil
}

func (c *ConfigMapManager) DeleteConfigMap(cm string) error {
	cmCli := c.KubeClient.CoreV1().ConfigMaps(c.Namespace)
	return cmCli.Delete(cm, &v1.DeleteOptions{})
}

func (c *ConfigMapManager) ListConfigMaps(options *v1.ListOptions) ([]v12.ConfigMap, error) {
	cmCli := c.KubeClient.CoreV1().ConfigMaps(c.Namespace)
	list, err := cmCli.List(*options)
	if list != nil {
		return list.Items, err
	}
	return nil, err
}

func (c *ConfigMapManager) CreateConfigMap(cm *v12.ConfigMap) (*v12.ConfigMap, error) {
	cmCli := c.KubeClient.CoreV1().ConfigMaps(c.Namespace)
	return cmCli.Create(cm)
}

func (c *ConfigMapManager) UpdateConfigMap(cm *v12.ConfigMap) (*v12.ConfigMap, error) {
	cmCli := c.KubeClient.CoreV1().ConfigMaps(c.Namespace)
	return cmCli.Update(cm)
}

func (c *ConfigMapManager) IsOwned(cm *v12.ConfigMap) bool {
	return IsOwnedBySkupper(cm.ObjectMeta.OwnerReferences)
}

type DeploymentManager struct {
	KubeClient kubernetes.Interface
	Namespace  string
}

func (d *DeploymentManager) GetDeployment(name string) (*v13.Deployment, bool, error) {
	depCli := d.KubeClient.AppsV1().Deployments(d.Namespace)
	dep, err := depCli.Get(name, v1.GetOptions{})
	if err != nil {
		return nil, false, err
	}
	return dep, true, nil
}

func (d *DeploymentManager) DeleteDeployment(dep string) error {
	depCli := d.KubeClient.AppsV1().Deployments(d.Namespace)
	return depCli.Delete(dep, &v1.DeleteOptions{})
}

func (d *DeploymentManager) ListDeployments(options *v1.ListOptions) ([]v13.Deployment, error) {
	depCli := d.KubeClient.AppsV1().Deployments(d.Namespace)
	list, err := depCli.List(*options)
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (d *DeploymentManager) CreateDeployment(dep *v13.Deployment) (*v13.Deployment, error) {
	depCli := d.KubeClient.AppsV1().Deployments(d.Namespace)
	return depCli.Create(dep)
}

func (d *DeploymentManager) UpdateDeployment(dep *v13.Deployment) (*v13.Deployment, error) {
	depCli := d.KubeClient.AppsV1().Deployments(d.Namespace)
	return depCli.Update(dep)
}

func (d *DeploymentManager) IsOwned(dep *v13.Deployment) bool {
	return IsOwnedBySkupper(dep.ObjectMeta.OwnerReferences)
}

type SecretManager struct {
	KubeClient kubernetes.Interface
	Namespace  string
}

func (s *SecretManager) GetSecret(name string) (*v12.Secret, bool, error) {
	secCli := s.KubeClient.CoreV1().Secrets(s.Namespace)
	sec, err := secCli.Get(name, v1.GetOptions{})
	if err != nil {
		return nil, false, err
	}
	return sec, true, nil
}

func (s *SecretManager) DeleteSecret(secret string) error {
	secCli := s.KubeClient.CoreV1().Secrets(s.Namespace)
	return secCli.Delete(secret, &v1.DeleteOptions{})
}

func (s *SecretManager) ListSecrets(options *v1.ListOptions) ([]v12.Secret, error) {
	secCli := s.KubeClient.CoreV1().Secrets(s.Namespace)
	list, err := secCli.List(*options)
	if err != nil {
		return nil, err
	}
	return list.Items, err
}

func (s *SecretManager) CreateSecret(secret *v12.Secret) (*v12.Secret, error) {
	secCli := s.KubeClient.CoreV1().Secrets(s.Namespace)
	return secCli.Create(secret)
}

func (s *SecretManager) UpdateSecret(secret *v12.Secret) (*v12.Secret, error) {
	secCli := s.KubeClient.CoreV1().Secrets(s.Namespace)
	return secCli.Update(secret)
}

func (s *SecretManager) IsOwned(secret *v12.Secret) bool {
	return IsOwnedBySkupper(secret.ObjectMeta.OwnerReferences)
}

type ServiceManager struct {
	KubeClient kubernetes.Interface
	Namespace  string
}

func (s *ServiceManager) GetService(name string) (*v12.Service, bool, error) {
	svcCli := s.KubeClient.CoreV1().Services(s.Namespace)
	svc, err := svcCli.Get(name, v1.GetOptions{})
	if err != nil {
		return nil, false, err
	}
	return svc, true, nil
}

func (s *ServiceManager) DeleteService(svc string) error {
	svcCli := s.KubeClient.CoreV1().Services(s.Namespace)
	return svcCli.Delete(svc, &v1.DeleteOptions{})
}

func (s *ServiceManager) ListServices(options *v1.ListOptions) ([]v12.Service, error) {
	svcCli := s.KubeClient.CoreV1().Services(s.Namespace)
	list, err := svcCli.List(*options)
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (s *ServiceManager) CreateService(svc *v12.Service) (*v12.Service, error) {
	svcCli := s.KubeClient.CoreV1().Services(s.Namespace)
	return svcCli.Create(svc)
}

func (s *ServiceManager) UpdateService(svc *v12.Service) (*v12.Service, error) {
	svcCli := s.KubeClient.CoreV1().Services(s.Namespace)
	return svcCli.Update(svc)
}

func (s *ServiceManager) IsOwned(service *v12.Service) bool {
	return IsOwnedBySkupper(service.ObjectMeta.OwnerReferences)
}

type StatefulSetManager struct {
	KubeClient kubernetes.Interface
	Namespace  string
}

func (s *StatefulSetManager) GetStatefulSet(name string) (*v13.StatefulSet, bool, error) {
	depCli := s.KubeClient.AppsV1().StatefulSets(s.Namespace)
	dep, err := depCli.Get(name, v1.GetOptions{})
	if err != nil {
		return nil, false, err
	}
	return dep, true, nil
}

func (s *StatefulSetManager) DeleteStatefulSet(ss string) error {
	ssCli := s.KubeClient.AppsV1().StatefulSets(s.Namespace)
	return ssCli.Delete(ss, &v1.DeleteOptions{})
}

func (s *StatefulSetManager) ListStatefulSets(options *v1.ListOptions) ([]v13.StatefulSet, error) {
	ssCli := s.KubeClient.AppsV1().StatefulSets(s.Namespace)
	list, err := ssCli.List(*options)
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (s *StatefulSetManager) CreateStatefulSet(ss *v13.StatefulSet) (*v13.StatefulSet, error) {
	ssCli := s.KubeClient.AppsV1().StatefulSets(s.Namespace)
	return ssCli.Create(ss)
}

func (s *StatefulSetManager) UpdateStatefulSet(ss *v13.StatefulSet) (*v13.StatefulSet, error) {
	ssCli := s.KubeClient.AppsV1().StatefulSets(s.Namespace)
	return ssCli.Update(ss)
}

func (s *StatefulSetManager) IsOwned(ss *v13.StatefulSet) bool {
	return IsOwnedBySkupper(ss.ObjectMeta.OwnerReferences)
}
