package kube

import (
	"github.com/skupperproject/skupper/api/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Storage interface {
	Services
	ConfigMaps
	Deployments
	Secrets
	StatefulSets
}

type Services interface {
	GetService(name string) (*corev1.Service, bool, error)
	DeleteService(name string) error
	ListServices(options *types.ListFilter) ([]corev1.Service, error)
	CreateService(svc *corev1.Service) (*corev1.Service, error)
	UpdateService(svc *corev1.Service) (*corev1.Service, error)
	IsOwnedService(service *corev1.Service) bool
}

type ConfigMaps interface {
	GetConfigMap(name string) (*corev1.ConfigMap, bool, error)
	DeleteConfigMap(cm string) error
	ListConfigMaps(options *types.ListFilter) ([]corev1.ConfigMap, error)
	CreateConfigMap(cm *corev1.ConfigMap) (*corev1.ConfigMap, error)
	UpdateConfigMap(cm *corev1.ConfigMap) (*corev1.ConfigMap, error)
	IsOwnedConfigMap(cm *corev1.ConfigMap) bool
}

type Deployments interface {
	GetDeployment(name string) (*appsv1.Deployment, bool, error)
	DeleteDeployment(dep string) error
	ListDeployments(options *types.ListFilter) ([]appsv1.Deployment, error)
	CreateDeployment(dep *appsv1.Deployment) (*appsv1.Deployment, error)
	UpdateDeployment(dep *appsv1.Deployment) (*appsv1.Deployment, error)
	IsOwnedDeployment(dep *appsv1.Deployment) bool
}

type Secrets interface {
	GetSecret(name string) (*corev1.Secret, bool, error)
	DeleteSecret(secret string) error
	ListSecrets(options *types.ListFilter) ([]corev1.Secret, error)
	CreateSecret(secret *corev1.Secret) (*corev1.Secret, error)
	UpdateSecret(secret *corev1.Secret) (*corev1.Secret, error)
	IsOwnedSecret(secret *corev1.Secret) bool
}

type StatefulSets interface {
	GetStatefulSet(name string) (*appsv1.StatefulSet, bool, error)
	DeleteStatefulSet(ss string) error
	ListStatefulSets(options *types.ListFilter) ([]appsv1.StatefulSet, error)
	CreateStatefulSet(ss *appsv1.StatefulSet) (*appsv1.StatefulSet, error)
	UpdateStatefulSet(ss *appsv1.StatefulSet) (*appsv1.StatefulSet, error)
	IsOwnedStatefulSet(ss *appsv1.StatefulSet) bool
}

type ConfigMapManager struct {
	KubeClient kubernetes.Interface
	Namespace  string
}

func (c *ConfigMapManager) GetConfigMap(name string) (*corev1.ConfigMap, bool, error) {
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

func (c *ConfigMapManager) ListConfigMaps(options *types.ListFilter) ([]corev1.ConfigMap, error) {
	listOptions := v1.ListOptions{}
	if options != nil {
		listOptions.LabelSelector = options.LabelSelector
		listOptions.Limit = options.Limit
	}

	cmCli := c.KubeClient.CoreV1().ConfigMaps(c.Namespace)
	list, err := cmCli.List(listOptions)
	if list != nil {
		return list.Items, err
	}
	return nil, err
}

func (c *ConfigMapManager) CreateConfigMap(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	cmCli := c.KubeClient.CoreV1().ConfigMaps(c.Namespace)
	return cmCli.Create(cm)
}

func (c *ConfigMapManager) UpdateConfigMap(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	cmCli := c.KubeClient.CoreV1().ConfigMaps(c.Namespace)
	return cmCli.Update(cm)
}

func (c *ConfigMapManager) IsOwnedConfigMap(cm *corev1.ConfigMap) bool {
	return IsOwnedBySkupper(cm.ObjectMeta.OwnerReferences)
}

type DeploymentManager struct {
	KubeClient kubernetes.Interface
	Namespace  string
}

func (d *DeploymentManager) GetDeployment(name string) (*appsv1.Deployment, bool, error) {
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

func (d *DeploymentManager) ListDeployments(options *types.ListFilter) ([]appsv1.Deployment, error) {
	listOptions := v1.ListOptions{}
	if options != nil {
		listOptions.LabelSelector = options.LabelSelector
		listOptions.Limit = options.Limit
	}
	depCli := d.KubeClient.AppsV1().Deployments(d.Namespace)
	list, err := depCli.List(listOptions)
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (d *DeploymentManager) CreateDeployment(dep *appsv1.Deployment) (*appsv1.Deployment, error) {
	depCli := d.KubeClient.AppsV1().Deployments(d.Namespace)
	return depCli.Create(dep)
}

func (d *DeploymentManager) UpdateDeployment(dep *appsv1.Deployment) (*appsv1.Deployment, error) {
	depCli := d.KubeClient.AppsV1().Deployments(d.Namespace)
	return depCli.Update(dep)
}

func (d *DeploymentManager) IsOwnedDeployment(dep *appsv1.Deployment) bool {
	return IsOwnedBySkupper(dep.ObjectMeta.OwnerReferences)
}

type SecretManager struct {
	KubeClient kubernetes.Interface
	Namespace  string
}

func (s *SecretManager) GetSecret(name string) (*corev1.Secret, bool, error) {
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

func (s *SecretManager) ListSecrets(options *types.ListFilter) ([]corev1.Secret, error) {
	listOptions := v1.ListOptions{}
	if options != nil {
		listOptions.LabelSelector = options.LabelSelector
		listOptions.Limit = options.Limit
	}

	secCli := s.KubeClient.CoreV1().Secrets(s.Namespace)
	list, err := secCli.List(listOptions)
	if err != nil {
		return nil, err
	}
	return list.Items, err
}

func (s *SecretManager) CreateSecret(secret *corev1.Secret) (*corev1.Secret, error) {
	secCli := s.KubeClient.CoreV1().Secrets(s.Namespace)
	return secCli.Create(secret)
}

func (s *SecretManager) UpdateSecret(secret *corev1.Secret) (*corev1.Secret, error) {
	secCli := s.KubeClient.CoreV1().Secrets(s.Namespace)
	return secCli.Update(secret)
}

func (s *SecretManager) IsOwnedSecret(secret *corev1.Secret) bool {
	return IsOwnedBySkupper(secret.ObjectMeta.OwnerReferences)
}

type StorageManager struct {
	KubeClient kubernetes.Interface
	Namespace  string
	*ServiceManager
	*ConfigMapManager
	*SecretManager
	*DeploymentManager
	*StatefulSetManager
}

func NewStorageManager(cli kubernetes.Interface, namespace string) Storage {
	return &StorageManager{
		KubeClient:         cli,
		Namespace:          namespace,
		ServiceManager:     &ServiceManager{KubeClient: cli, Namespace: namespace},
		ConfigMapManager:   &ConfigMapManager{KubeClient: cli, Namespace: namespace},
		SecretManager:      &SecretManager{KubeClient: cli, Namespace: namespace},
		DeploymentManager:  &DeploymentManager{KubeClient: cli, Namespace: namespace},
		StatefulSetManager: &StatefulSetManager{KubeClient: cli, Namespace: namespace},
	}
}

type ServiceManager struct {
	KubeClient kubernetes.Interface
	Namespace  string
}

func (s *ServiceManager) GetService(name string) (*corev1.Service, bool, error) {
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

func (s *ServiceManager) ListServices(options *types.ListFilter) ([]corev1.Service, error) {
	listOptions := v1.ListOptions{}
	if options != nil {
		listOptions.LabelSelector = options.LabelSelector
		listOptions.Limit = options.Limit
	}
	svcCli := s.KubeClient.CoreV1().Services(s.Namespace)
	list, err := svcCli.List(listOptions)
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (s *ServiceManager) CreateService(svc *corev1.Service) (*corev1.Service, error) {
	svcCli := s.KubeClient.CoreV1().Services(s.Namespace)
	return svcCli.Create(svc)
}

func (s *ServiceManager) UpdateService(svc *corev1.Service) (*corev1.Service, error) {
	svcCli := s.KubeClient.CoreV1().Services(s.Namespace)
	return svcCli.Update(svc)
}

func (s *ServiceManager) IsOwnedService(service *corev1.Service) bool {
	return IsOwnedBySkupper(service.ObjectMeta.OwnerReferences)
}

type StatefulSetManager struct {
	KubeClient kubernetes.Interface
	Namespace  string
}

func (s *StatefulSetManager) GetStatefulSet(name string) (*appsv1.StatefulSet, bool, error) {
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

func (s *StatefulSetManager) ListStatefulSets(options *types.ListFilter) ([]appsv1.StatefulSet, error) {
	listOptions := v1.ListOptions{}
	if options != nil {
		listOptions.LabelSelector = options.LabelSelector
		listOptions.Limit = options.Limit
	}
	ssCli := s.KubeClient.AppsV1().StatefulSets(s.Namespace)
	list, err := ssCli.List(listOptions)
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (s *StatefulSetManager) CreateStatefulSet(ss *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	ssCli := s.KubeClient.AppsV1().StatefulSets(s.Namespace)
	return ssCli.Create(ss)
}

func (s *StatefulSetManager) UpdateStatefulSet(ss *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	ssCli := s.KubeClient.AppsV1().StatefulSets(s.Namespace)
	return ssCli.Update(ss)
}

func (s *StatefulSetManager) IsOwnedStatefulSet(ss *appsv1.StatefulSet) bool {
	return IsOwnedBySkupper(ss.ObjectMeta.OwnerReferences)
}
