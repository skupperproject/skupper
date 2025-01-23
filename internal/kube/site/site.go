package site

import (
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/kube/certificates"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	kubeqdr "github.com/skupperproject/skupper/internal/kube/qdr"
	"github.com/skupperproject/skupper/internal/kube/site/resources"
	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/internal/site"
	"github.com/skupperproject/skupper/internal/version"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type SecuredAccessFactory interface {
	Ensure(namespace string, name string, spec skupperv2alpha1.SecuredAccessSpec, annotations map[string]string, refs []metav1.OwnerReference) error
	Delete(namespace string, name string) error
	IsValidAccessType(accessType string) bool
}

type Site struct {
	initialised   bool
	site          *skupperv2alpha1.Site
	name          string
	namespace     string
	controller    *internalclient.Controller
	bindings      *ExtendedBindings
	links         map[string]*site.Link
	errors        map[string]string
	linkAccess    site.RouterAccessMap
	certs         certificates.CertificateManager
	access        SecuredAccessFactory
	routerPods    map[string]*corev1.Pod
	logger        *slog.Logger
	currentGroups []string
}

func NewSite(namespace string, controller *internalclient.Controller, certs certificates.CertificateManager, access SecuredAccessFactory) *Site {
	return &Site{
		bindings:   NewExtendedBindings(controller, SSL_PROFILE_PATH),
		namespace:  namespace,
		controller: controller,
		links:      map[string]*site.Link{},
		linkAccess: site.RouterAccessMap{},
		certs:      certs,
		access:     access,
		routerPods: map[string]*corev1.Pod{},
		logger: slog.New(slog.Default().Handler()).With(
			slog.String("component", "kube.site.site"),
		),
	}
}

func (s *Site) verifySiteSpec(site *skupperv2alpha1.Site) error {
	if site.Spec.LinkAccess != "" && site.Spec.LinkAccess != "none" && site.Spec.LinkAccess != "default" && !s.access.IsValidAccessType(site.Spec.LinkAccess) {
		return fmt.Errorf("Unsupported value for LinkAccess: %s", site.Spec.LinkAccess)
	}
	return nil
}

func (s *Site) StartRecovery(site *skupperv2alpha1.Site) error {
	//TODO: check version and perform any necessary update tasks
	return s.reconcile(site, true)
}

func (s *Site) isEdge() bool {
	return s.routerMode() == qdr.ModeEdge
}

func (s *Site) routerMode() qdr.Mode {
	if s.site != nil && s.site.Spec.Edge {
		return qdr.ModeEdge
	} else {
		return qdr.ModeInterior
	}
}

const SSL_PROFILE_PATH = "/etc/skupper-router-certs"

func (s *Site) Reconcile(siteDef *skupperv2alpha1.Site) error {
	err := s.reconcile(siteDef, false)
	return s.updateConfigured(err)
}

func (s *Site) reconcile(siteDef *skupperv2alpha1.Site, inRecovery bool) error {
	if s.site != nil && s.site.Name != siteDef.Name {
		s.logger.Error("Rejecting sitedef as active site already exists in the namespace",
			slog.String("sitedef_namespace", siteDef.Namespace),
			slog.String("sitedef_name", siteDef.Name),
			slog.String("name", s.site.Name))
		return s.markSiteInactive(siteDef, fmt.Errorf("An active site already exists in the namespace (%s)", s.site.Name))
	}
	s.site = siteDef
	s.name = string(siteDef.ObjectMeta.Name)
	s.logger.Debug("Checking site",
		slog.String("namespace", siteDef.Namespace),
		slog.String("name", siteDef.Name),
		slog.String("id", s.site.GetSiteId()))
	if err := s.verifySiteSpec(siteDef); err != nil {
		return err
	}
	// ensure necessary resources:
	// 1. skupper-internal configmap
	if !s.initialised {
		s.logger.Info("Initialising site",
			slog.String("namespace", siteDef.Namespace),
			slog.String("name", siteDef.Name))
		routerConfigs, err := s.recoverRouterConfig(!inRecovery)
		if err != nil {
			return err
		}

		var routerConfig *qdr.RouterConfig
		if len(routerConfigs) > 0 {
			routerConfig = routerConfigs[0]
		}
		s.initialised = true
		s.currentGroups = s.groups()
		s.bindings.init(s, routerConfig)
		s.bindings.SetSite(s)
		s.setBindingsConfiguredStatus(nil)
		s.checkSecuredAccess()
	} else if len(s.currentGroups) != len(s.groups()) {
		s.logger.Info("HA setting changed for site",
			slog.String("namespace", siteDef.Namespace),
			slog.String("name", siteDef.Name),
			slog.String("latest", strings.Join(s.groups(), ",")),
			slog.String("previous", strings.Join(s.currentGroups, ",")),
		)
		s.currentGroups = s.groups()
		if _, err := s.recoverRouterConfig(true); err != nil {
			return err
		}
		if err := s.checkSecuredAccess(); err != nil {
			return err
		}
	} else {
		if err := s.updateRouterConfig(s); err != nil {
			return err
		}
	}
	ctxt := context.TODO()
	// 2. service account (optional)
	if s.site.Spec.ServiceAccount == "" {
		if err := s.checkRole(ctxt); err != nil {
			return err
		}
		if err := s.checkServiceAccount(ctxt); err != nil {
			return err
		}
		if err := s.checkRoleBinding(ctxt); err != nil {
			return err
		}
	}
	// CAs for local and site access
	if err := s.certs.EnsureCA(s.namespace, "skupper-site-ca", fmt.Sprintf("%s site CA", s.name), s.ownerReferences()); err != nil {
		return err
	}
	if err := s.certs.EnsureCA(s.namespace, "skupper-local-ca", fmt.Sprintf("%s local CA", s.name), s.ownerReferences()); err != nil {
		return err
	}
	if err := s.certs.Ensure(s.namespace, "skupper-local-server", "skupper-local-ca", "skupper-router-local", s.qualified("skupper-router-local"), false, true, s.ownerReferences()); err != nil {
		return err
	}
	// RouterAccess for router
	if err := s.checkDefaultRouterAccess(ctxt, siteDef); err != nil {
		return err
	}

	// 3. deployment
	for _, group := range s.groups() {
		//TODO: if change from HA=true to HA=false, will need to remove previous resources
		if err := resources.Apply(s.controller, ctxt, s.site, group); err != nil {
			return err
		}
	}
	return nil
}

func (s *Site) initialRouterConfig() *qdr.RouterConfig {
	rc := qdr.InitialConfig(s.name+"-${HOSTNAME}", s.site.GetSiteId(), version.Version, s.isEdge(), 3)
	rc.AddAddress(qdr.Address{
		Prefix:       "mc",
		Distribution: "multicast",
	})
	rc.AddHealthAndMetricsListener(9090)
	rc.AddListener(qdr.Listener{
		Name: "amqp",
		Host: "localhost",
		Port: 5672,
	})
	rc.AddSslProfile(qdr.ConfigureSslProfile("skupper-local-server", SSL_PROFILE_PATH, true))
	rc.AddListener(qdr.Listener{
		Name:             "amqps",
		Port:             5671,
		SslProfile:       "skupper-local-server",
		SaslMechanisms:   "EXTERNAL",
		AuthenticatePeer: true,
	})
	return &rc
}

func (s *Site) groups() []string {
	if s.site.Spec.HA {
		return []string{"skupper-router", "skupper-router-2"}
	} else {
		return []string{"skupper-router"}
	}
}

func (s *Site) checkDefaultRouterAccess(ctxt context.Context, site *skupperv2alpha1.Site) error {
	if site.Spec.LinkAccess == "" || site.Spec.LinkAccess == "none" {
		return nil
	}
	name := "skupper-router"
	accessType := site.Spec.LinkAccess
	if site.Spec.LinkAccess == "default" {
		accessType = ""
	}
	desired := &skupperv2alpha1.RouterAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "RouterAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			OwnerReferences: s.ownerReferences(),
			Annotations: map[string]string{
				"internal.skupper.io/controlled": "true",
			},
		},
		Spec: skupperv2alpha1.RouterAccessSpec{
			AccessType:             accessType,
			TlsCredentials:         "skupper-site-server",
			Issuer:                 "skupper-site-ca", //TODO: can rely ondefault here
			GenerateTlsCredentials: true,
			Roles: []skupperv2alpha1.RouterAccessRole{
				{
					Name: "inter-router",
					Port: 55671,
				},
				{
					Name: "edge",
					Port: 45671,
				},
			},
		},
	}
	current, ok := s.linkAccess[name]
	if ok {
		if reflect.DeepEqual(current.Spec, desired.Spec) {
			return nil
		}
		current.Spec = desired.Spec
		updated, err := s.controller.GetSkupperClient().SkupperV2alpha1().RouterAccesses(s.namespace).Update(context.Background(), current, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		s.linkAccess[name] = updated
		return nil
	} else {
		created, err := s.controller.GetSkupperClient().SkupperV2alpha1().RouterAccesses(s.namespace).Create(context.Background(), desired, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		s.linkAccess[name] = created
		return nil
	}
}

func (s *Site) checkServiceAccount(ctxt context.Context) error {
	sa := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "skupper-router",
			OwnerReferences: s.ownerReferences(),
		},
	}
	_, err := s.controller.GetKubeClient().CoreV1().ServiceAccounts(s.namespace).Create(ctxt, sa, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (s *Site) checkRoleBinding(ctxt context.Context) error {
	name := "skupper-router"
	existing, err := s.controller.GetKubeClient().RbacV1().RoleBindings(s.namespace).Get(ctxt, name, metav1.GetOptions{})
	desired := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			OwnerReferences: s.ownerReferences(),
		},
		Subjects: []rbacv1.Subject{{
			Kind: "ServiceAccount",
			Name: name,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: name,
		},
	}
	if errors.IsNotFound(err) {
		_, err := s.controller.GetKubeClient().RbacV1().RoleBindings(s.namespace).Create(ctxt, desired, metav1.CreateOptions{})
		return err
	} else if err != nil {
		return err
	} else if !reflect.DeepEqual(existing.Subjects, desired.Subjects) || !reflect.DeepEqual(existing.RoleRef, desired.RoleRef) {
		existing.Subjects = desired.Subjects
		existing.RoleRef = desired.RoleRef
		_, err := s.controller.GetKubeClient().RbacV1().RoleBindings(s.namespace).Update(ctxt, existing, metav1.UpdateOptions{})
		return err
	}
	return nil
}

func (s *Site) checkRole(ctxt context.Context) error {
	rules := []rbacv1.PolicyRule{
		{
			Verbs:     []string{"get", "list", "watch"},
			APIGroups: []string{""},
			Resources: []string{"secrets", "pods"},
		},
		{
			Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
		},
		{
			Verbs:     []string{"get"},
			APIGroups: []string{"apps"},
			Resources: []string{"deployments"},
		},
		//needed for redeeming token claims
		{
			Verbs:     []string{"update", "delete"},
			APIGroups: []string{""},
			Resources: []string{"secrets"},
		},
		//needed for determining token urls
		{
			Verbs:     []string{"get", "list", "watch"},
			APIGroups: []string{""},
			Resources: []string{"services"},
		},
		//needed for leader election
		{
			Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
			APIGroups: []string{"coordination.k8s.io"},
			Resources: []string{"leases"},
		},
	}
	desired := &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "skupper-router",
			OwnerReferences: s.ownerReferences(),
		},
		Rules: rules,
	}
	roles := s.controller.GetKubeClient().RbacV1().Roles(s.namespace)
	existing, err := roles.Get(ctxt, desired.ObjectMeta.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err := roles.Create(ctxt, desired, metav1.CreateOptions{})
		return err
	} else if err != nil {
		return err
	} else if !reflect.DeepEqual(existing.Rules, desired.Rules) {
		existing.Rules = desired.Rules
		_, err = roles.Update(ctxt, existing, metav1.UpdateOptions{})
		return err
	}

	return nil
}

func (s *Site) endpoints() []skupperv2alpha1.Endpoint {
	var endpoints []skupperv2alpha1.Endpoint
	for _, la := range s.linkAccess {
		for _, endpoint := range la.Status.Endpoints {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints
}

func (s *Site) qualified(svc string) []string {
	return []string{
		svc,
		strings.Join([]string{svc, s.namespace}, "."),
		strings.Join([]string{svc, s.namespace, "svc.cluster.local"}, "."),
	}
}

func (s *Site) Apply(config *qdr.RouterConfig) bool {
	updated := false
	if mode := s.routerMode(); config.Metadata.Mode != mode {
		updated = true
		config.Metadata.Mode = mode
	}
	if dcc := s.site.Spec.GetRouterDataConnectionCount(); config.Metadata.DataConnectionCount != dcc {
		updated = true
		config.Metadata.DataConnectionCount = dcc
	}
	if logging, err := qdr.ParseRouterLogConfig(s.site.Spec.GetRouterLogging()); err != nil {
		if qdr.ConfigureRouterLogging(config, logging) {
			updated = true
		}
	} else {
		s.logger.Error("Invalid value for router logging in settings",
			slog.String("namespace", s.namespace),
			slog.String("name", s.name))
	}
	return updated
}

func (s *Site) IsInitialised() bool {
	return s.initialised
}

func (s *Site) Select(connector *skupperv2alpha1.Connector) TargetSelection {
	name := connector.Name
	selector := connector.Spec.Selector
	includeNotReadyPods := connector.Spec.IncludeNotReadyPods
	if selector == "" {
		return nil
	}
	handler := &TargetSelectionImpl{
		site:                s,
		name:                name,
		selector:            selector,
		namespace:           s.namespace,
		includeNotReadyPods: includeNotReadyPods,
	}
	handler.watcher = s.WatchPods(handler, s.namespace)
	return handler
}

func (s *Site) WatchPods(context PodWatchingContext, namespace string) *PodWatcher {
	w := &PodWatcher{
		stopCh:  make(chan struct{}),
		context: context,
	}
	w.watcher = s.controller.WatchPods(context.Selector(), namespace, w.handle)
	w.watcher.Start(w.stopCh)
	return w
}

func (s *Site) Expose(exposed *ExposedPortSet) error {
	ctxt := context.TODO()
	current, err := s.controller.GetKubeClient().CoreV1().Services(s.namespace).Get(ctxt, exposed.Host, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		service := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: exposed.Host,
				Annotations: map[string]string{
					"internal.skupper.io/controlled": "true",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: getLabelsForRouter(), //TODO: handle external bridges
			},
		}
		//TODO: add user specified labels and annotations
		//for key, value := range desired.Labels {
		//	service.ObjectMeta.Labels[key] = value
		//}
		//for key, value := range desired.Annotations {
		//	service.ObjectMeta.Annotations[key] = value
		//}
		if updatePorts(&service.Spec, exposed.Ports) {
			_, err := s.controller.GetKubeClient().CoreV1().Services(s.namespace).Create(ctxt, service, metav1.CreateOptions{})
			if err != nil {
				s.logger.Error("Error creating service",
					slog.String("service", exposed.Host),
					slog.String("namespace", s.namespace),
					slog.Any("error", err))
				return err
			} else {
				s.logger.Info("Created service",
					slog.String("service", exposed.Host),
					slog.String("namespace", s.namespace))
				return nil
			}
		} else {
			s.logger.Info("Did not create service as ports were not updated",
				slog.String("service", exposed.Host),
				slog.String("namespace", s.namespace))
			return nil
		}
	} else if err != nil {
		s.logger.Error("Error checking service",
			slog.String("service", exposed.Host),
			slog.String("namespace", s.namespace),
			slog.Any("error", err))
		return err
	} else {
		updated := false
		if updateSelectorFromMap(&current.Spec, getLabelsForRouter()) {
			updated = true
		}
		if updatePorts(&current.Spec, exposed.Ports) {
			updated = true
		}
		//TODO: update labels and annotations
		if updated {
			_, err := s.controller.GetKubeClient().CoreV1().Services(s.namespace).Update(ctxt, current, metav1.UpdateOptions{})
			if err != nil {
				s.logger.Error("Error creating service",
					slog.String("service", exposed.Host),
					slog.String("namespace", s.namespace),
					slog.Any("error", err))
				return err
			} else {
				s.logger.Info("Updated service",
					slog.String("service", exposed.Host),
					slog.String("namespace", s.namespace))
			}
		}
		return nil
	}
}

func (s *Site) Unexpose(name string) error {
	ctxt := context.TODO()
	current, err := s.controller.GetKubeClient().CoreV1().Services(s.namespace).Get(ctxt, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			s.logger.Error("Error checking service to be deleted",
				slog.String("service", name),
				slog.String("namespace", s.namespace),
				slog.Any("error", err))
		}
		return err
	} else if isOwned(current) {
		err = s.controller.GetKubeClient().CoreV1().Services(s.namespace).Delete(ctxt, name, metav1.DeleteOptions{})
		if err != nil {
			s.logger.Error("Error deleting service",
				slog.String("service", name),
				slog.String("namespace", s.namespace),
				slog.Any("error", err))
			return err
		} else {
			s.logger.Info("Deleted service",
				slog.String("service", name),
				slog.String("namespace", s.namespace))
		}
	}
	return nil
}

func isOwned(service *corev1.Service) bool {
	if service.ObjectMeta.Annotations == nil {
		return false
	}
	// assume that if annotation is set, irrespective of value, the service is owned by skupper
	if _, ok := service.ObjectMeta.Annotations[types.ControlledQualifier]; !ok {
		return false
	}
	return true
}

func (s *Site) ownerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			Kind:       "Site",
			APIVersion: "skupper.io/v2alpha1",
			Name:       s.name,
			UID:        s.site.ObjectMeta.UID,
		},
	}
}

func (s *Site) recoverRouterConfig(update bool) ([]*qdr.RouterConfig, error) {
	list, err := s.controller.GetKubeClient().CoreV1().ConfigMaps(s.namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "internal.skupper.io/router-config",
	})
	if err != nil {
		return nil, err
	}
	byName := map[string]*qdr.RouterConfig{}
	for _, cm := range list.Items {
		config, err := qdr.GetRouterConfigFromConfigMap(&cm)
		if err != nil {
			s.logger.Error("Error parsing router config from config map",
				slog.String("namespace", s.namespace),
				slog.String("name", cm.Name),
				slog.Any("error", err))
		} else {
			byName[cm.Name] = config
		}
	}
	//need to ensure that the list of configs is in the right order, i.e. matching s.groups()
	var configs []*qdr.RouterConfig
	groups := s.groups()
	for i, group := range groups {
		if config, ok := byName[group]; ok {
			if update {
				op := ConfigUpdateList{s.bindings, s, s.linkAccess.DesiredConfig(groups[:i], SSL_PROFILE_PATH)}
				if err := kubeqdr.UpdateRouterConfig(s.controller.GetKubeClient(), group, s.namespace, context.TODO(), op); err != nil {
					s.logger.Error("Failed to update router config map",
						slog.String("namespace", s.namespace),
						slog.String("name", group),
						slog.Any("error", err))
				}
			}
			configs = append(configs, config)
			delete(byName, group)
		} else {
			routerConfig := s.initialRouterConfig()
			s.bindings.Apply(routerConfig)
			s.linkAccess.DesiredConfig(groups[:i], SSL_PROFILE_PATH).Apply(routerConfig)
			if err := s.createRouterConfigForGroup(group, routerConfig); err != nil {
				s.logger.Error("Failed to create router config map",
					slog.String("namespace", s.namespace),
					slog.String("name", group),
					slog.Any("error", err))
			} else {
				s.logger.Info("Router config created for site",
					slog.String("namespace", s.namespace),
					slog.String("name", group))
			}
		}
	}
	for name, _ := range byName {
		// no longer needed, delete it (and other associated router resources?)
		s.deleteRouterResources(name)
	}
	return configs, nil
}

func (s *Site) deleteRouterResources(group string) error {
	var errs []error
	if err := s.controller.GetKubeClient().CoreV1().ConfigMaps(s.namespace).Delete(context.TODO(), group, metav1.DeleteOptions{}); err != nil {
		s.logger.Error("Failed to delete router config map",
			slog.String("namespace", s.namespace),
			slog.String("name", group),
			slog.Any("error", err))
		errs = append(errs, err)
	}
	if err := s.controller.GetKubeClient().AppsV1().Deployments(s.namespace).Delete(context.TODO(), group, metav1.DeleteOptions{}); err != nil {
		s.logger.Error("Failed to delete router deployment",
			slog.String("namespace", s.namespace),
			slog.String("name", group),
			slog.Any("error", err))
		errs = append(errs, err)
	}
	if err := s.access.Delete(s.namespace, group); err != nil {
		s.logger.Error("Failed to delete securedaccess for router",
			slog.String("namespace", s.namespace),
			slog.String("name", group),
			slog.Any("error", err))
		errs = append(errs, err)
	}
	return stderrors.Join(errs...)
}

func (s *Site) createRouterConfig(config *qdr.RouterConfig) error {
	for _, group := range s.groups() {
		if err := s.createRouterConfigForGroup(group, config); err != nil {
			return err
		}
	}
	return nil
}
func (s *Site) createRouterConfigForGroup(group string, config *qdr.RouterConfig) error {
	data, err := config.AsConfigMapData()
	if err != nil {
		return err
	}
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            group,
			OwnerReferences: s.ownerReferences(),
			//TODO: Labels & Annotations?
			Labels: map[string]string{
				"internal.skupper.io/router-config": "",
			},
		},
		Data: data,
	}
	if _, err = s.controller.GetKubeClient().CoreV1().ConfigMaps(s.namespace).Create(context.TODO(), cm, metav1.CreateOptions{}); err != nil {
		s.logger.Error("Failed to create config map",
			slog.String("namespace", s.namespace),
			slog.String("name", group),
			slog.Any("error", err))
		return err
	}
	s.logger.Info("Config map created successfully",
		slog.String("namespace", s.namespace),
		slog.String("name", group))
	return nil
}

func (s *Site) updateRouterConfig(update qdr.ConfigUpdate) error {
	for _, group := range s.groups() {
		if err := s.updateRouterConfigForGroup(update, group); err != nil {
			return err
		}
	}
	s.logger.Debug("Router config updated for site",
		slog.String("namespace", s.namespace),
		slog.String("name", s.name))
	return nil
}

func (s *Site) updateRouterConfigForGroup(update qdr.ConfigUpdate, group string) error {
	if !s.initialised {
		return nil
	}
	if err := kubeqdr.UpdateRouterConfig(s.controller.GetKubeClient(), group, s.namespace, context.TODO(), update); err != nil {
		return err
	}
	return nil
}

func (s *Site) updateConnectorStatus(connector *skupperv2alpha1.Connector) error {
	updated, err := updateConnectorStatus(s.controller, connector)
	if err != nil {
		return err
	}
	s.bindings.UpdateConnector(updated.Name, updated)
	return nil
}

func (s *Site) updateConnectorConfiguredStatus(connector *skupperv2alpha1.Connector, err error) error {
	if connector.SetConfigured(err) {
		return s.updateConnectorStatus(connector)
	}
	return nil
}

func (s *Site) updateConnectorConfiguredStatusWithSelectedPods(connector *skupperv2alpha1.Connector, selected []skupperv2alpha1.PodDetails) error {
	var err error
	if len(selected) == 0 {
		s.logger.Error("No pods selected for connector",
			slog.String("namespace", connector.Namespace),
			slog.String("name", connector.Name))
		err = fmt.Errorf("No pods match selector")
	} else {

	}
	if connector.SetConfigured(err) || connector.SetSelectedPods(selected) {
		return s.updateConnectorStatus(connector)
	}
	return nil
}

func (s *Site) CheckConnector(name string, connector *skupperv2alpha1.Connector) error {
	update := s.bindings.UpdateConnector(name, connector)
	if s.site == nil {
		return s.updateConnectorConfiguredStatus(connector, stderrors.New("No active site in namespace"))
	}
	if update == nil {
		return nil
	}
	err := s.updateRouterConfig(update)
	if connector == nil {
		return err
	}
	return s.updateConnectorConfiguredStatus(connector, err)
}

func (s *Site) updateListenerStatus(listener *skupperv2alpha1.Listener, err error) error {
	if listener.SetConfigured(err) {
		updated, err := s.controller.GetSkupperClient().SkupperV2alpha1().Listeners(listener.ObjectMeta.Namespace).UpdateStatus(context.TODO(), listener, metav1.UpdateOptions{})
		if err == nil {
			return err
		}
		s.bindings.UpdateListener(updated.Name, updated)
	}
	return nil
}

func (s *Site) CheckListener(name string, listener *skupperv2alpha1.Listener) error {
	update, err1 := s.bindings.UpdateListener(name, listener)
	if s.site == nil {
		if listener == nil {
			return nil
		}
		return s.updateListenerStatus(listener, stderrors.New("No active site in namespace"))
	}
	if update == nil {
		return nil
	}
	err2 := s.updateRouterConfig(update)
	if listener == nil {
		return stderrors.Join(err1, err2)
	}
	return s.updateListenerStatus(listener, stderrors.Join(err1, err2))
}

func (s *Site) setBindingsConfiguredStatus(err error) {
	lf := func(listener *skupperv2alpha1.Listener) *skupperv2alpha1.Listener {
		if listener.SetConfigured(nil) {
			updated, err := s.controller.GetSkupperClient().SkupperV2alpha1().Listeners(listener.ObjectMeta.Namespace).UpdateStatus(context.TODO(), listener, metav1.UpdateOptions{})
			if err == nil {
				return updated
			} else {
				s.logger.Error("Could not update listener status",
					slog.String("namespace", listener.ObjectMeta.Namespace),
					slog.String("listener", listener.ObjectMeta.Name),
					slog.Any("error", err))
			}
		}
		return nil
	}
	cf := func(connector *skupperv2alpha1.Connector) *skupperv2alpha1.Connector {
		if connector.SetConfigured(nil) {
			updated, err := s.controller.GetSkupperClient().SkupperV2alpha1().Connectors(connector.ObjectMeta.Namespace).UpdateStatus(context.TODO(), connector, metav1.UpdateOptions{})
			if err == nil {
				return updated
			} else {
				s.logger.Error("Could not update connector status",
					slog.String("namespace", connector.ObjectMeta.Namespace),
					slog.String("connector", connector.ObjectMeta.Name),
					slog.Any("error", err))
			}
		}
		return nil
	}
	s.bindings.Map(cf, lf)
}

func (s *Site) newLink(linkconfig *skupperv2alpha1.Link) *site.Link {
	config := site.NewLink(linkconfig.ObjectMeta.Name, SSL_PROFILE_PATH)
	config.Update(linkconfig)
	return config
}

func (s *Site) CheckLink(name string, linkconfig *skupperv2alpha1.Link) error {
	s.logger.Debug("checkLink",
		slog.String("name", name))
	if linkconfig == nil {
		return s.unlink(name)
	}
	return s.link(linkconfig)
}

func (s *Site) link(linkconfig *skupperv2alpha1.Link) error {
	var config *site.Link
	if existing, ok := s.links[linkconfig.ObjectMeta.Name]; ok {
		if existing.Update(linkconfig) {
			config = existing
		}
	} else {
		config = s.newLink(linkconfig)
		s.links[linkconfig.ObjectMeta.Name] = config
	}
	if s.initialised {
		if config != nil {
			s.logger.Info("Connecting site using token",
				slog.String("namespace", s.namespace),
				slog.String("token", linkconfig.ObjectMeta.Name))
			err := s.updateRouterConfig(config)
			return s.updateLinkConfiguredCondition(linkconfig, err)
		} else {
			s.logger.Debug("No update to router config required for link",
				slog.String("namespace", linkconfig.ObjectMeta.Namespace),
				slog.String("token", linkconfig.ObjectMeta.Name))
		}
	} else {
		s.logger.Info("Site is not yet initialised, cannot configure router for link",
			slog.String("namespace", linkconfig.ObjectMeta.Namespace),
			slog.String("token", linkconfig.ObjectMeta.Name))
		return s.updateLinkConfiguredCondition(linkconfig, stderrors.New("No active site in namespace"))
	}
	return nil
}

func (s *Site) unlink(name string) error {
	if _, ok := s.links[name]; ok {
		s.logger.Info("Disconnecting connector from site",
			slog.String("name", name),
			slog.String("namespace", s.namespace))
		delete(s.links, name)
		if s.initialised {
			return s.updateRouterConfig(site.NewRemoveConnector(name))
		}
	}
	return nil
}

func (s *Site) updateLinkConfiguredCondition(link *skupperv2alpha1.Link, err error) error {
	if link == nil {
		return nil
	}
	if link.SetConfigured(err) {
		return s.updateLinkStatus(link)
	}
	return nil
}

func (s *Site) updateLinkStatus(link *skupperv2alpha1.Link) error {
	updated, err := s.controller.GetSkupperClient().SkupperV2alpha1().Links(link.ObjectMeta.Namespace).UpdateStatus(context.TODO(), link, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	s.links[link.ObjectMeta.Name].Update(updated)
	return nil
}

func (s *Site) Deleted() {
	s.logger.Info("Deleting site",
		slog.String("namespace", s.namespace),
		slog.String("name", s.name))
	s.bindings.cleanup()
	s.setBindingsConfiguredStatus(stderrors.New("No active site"))
}

func (s *Site) setDefaultIssuerInStatus() bool {
	if issuer := s.site.DefaultIssuer(); s.site.Status.DefaultIssuer != issuer {
		s.site.Status.DefaultIssuer = issuer
		return true
	}
	return false
}

func (s *Site) updateConfigured(err error) error {
	changed := false
	if s.setDefaultIssuerInStatus() {
		changed = true
	}
	if s.site.SetConfigured(err) {
		changed = true
		if err != nil {
			s.logger.Error("Error configuring site",
				slog.String("namespace", s.site.Namespace),
				slog.String("name", s.site.Name),
				slog.String("id", s.site.GetSiteId()),
				slog.Any("error", err))
		}
	}
	if changed {
		return s.updateSiteStatus()
	}
	return nil
}

func (s *Site) updateResolved() error {
	if s.site.SetEndpoints(s.endpoints()) {
		return s.updateSiteStatus()
	}
	return nil
}

func (s *Site) updateSiteStatus() error {
	updated, err := s.controller.GetSkupperClient().SkupperV2alpha1().Sites(s.site.ObjectMeta.Namespace).UpdateStatus(context.TODO(), s.site, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	s.site = updated
	return nil
}

func (s *Site) updateLinkOperationalCondition(link *skupperv2alpha1.Link, operational bool, remoteSiteId string, remoteSiteName string) error {
	if link.SetOperational(operational, remoteSiteId, remoteSiteName) {
		return s.updateLinkStatus(link)
	}
	return nil
}

func getLinkRecordsForSite(siteId string, network []skupperv2alpha1.SiteRecord) []skupperv2alpha1.LinkRecord {
	for _, siteRecord := range network {
		if siteRecord.Id == siteId {
			return siteRecord.Links
		}
	}
	return nil
}

func (s *Site) NetworkStatusUpdated(network []skupperv2alpha1.SiteRecord) error {
	if s.site == nil || reflect.DeepEqual(s.site.Status.Network, network) {
		return nil
	}
	s.site.Status.Network = network
	s.site.Status.SitesInNetwork = len(network)
	updated, err := s.UpdateSiteStatus(s.site)
	if err != nil {
		return err
	}
	s.site = updated

	// find the site record for this site, then process the link records it contains
	linkRecords := getLinkRecordsForSite(s.site.GetSiteId(), network)
	for _, linkRecord := range linkRecords {
		if link, ok := s.links[linkRecord.Name]; ok {
			if err := s.updateLinkOperationalCondition(link.Definition(), linkRecord.Operational, linkRecord.RemoteSiteId, linkRecord.RemoteSiteName); err != nil {
				s.logger.Error("Error updating operational status of link",
					slog.String("namespace", s.site.Namespace),
					slog.String("link", linkRecord.Name),
					slog.Any("error", err))
			}
		}
	}
	if config := s.bindings.networkUpdated(network); config != nil {
		if err := s.updateRouterConfig(config); err != nil {
			return err
		}
	}

	bindingStatus := newBindingStatus(s.controller, network)
	s.bindings.Map(bindingStatus.updateMatchingListenerCount, bindingStatus.updateMatchingConnectorCount)
	s.logger.Debug("Updating matching listeners for attached connectors")
	s.bindings.MapOverAttachedConnectors(bindingStatus.updateMatchingListenerCountForAttachedConnector)
	return bindingStatus.error()
}

func (s *Site) markSiteInactive(site *skupperv2alpha1.Site, err error) error {
	if site.SetConfigured(err) {
		if _, err := s.UpdateSiteStatus(site); err != nil {
			return err
		}
	}
	return nil
}

func (s *Site) UpdateSiteStatus(site *skupperv2alpha1.Site) (*skupperv2alpha1.Site, error) {
	updated, err := s.controller.GetSkupperClient().SkupperV2alpha1().Sites(site.ObjectMeta.Namespace).UpdateStatus(context.TODO(), site, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Site) CheckSecuredAccess(sa *skupperv2alpha1.SecuredAccess) {
	name, ok := sa.ObjectMeta.Annotations["internal.skupper.io/routeraccess"]
	if !ok {
		name = sa.Name
	}
	la, ok := s.linkAccess[name]
	if !ok {
		return
	}
	if la.Resolve(sa.Status.Endpoints, sa.Name) {
		s.updateRouterAccessStatus(la)
	}
}

func (s *Site) updateRouterAccessStatus(la *skupperv2alpha1.RouterAccess) {
	updated, err := s.controller.GetSkupperClient().SkupperV2alpha1().RouterAccesses(la.Namespace).UpdateStatus(context.TODO(), la, metav1.UpdateOptions{})

	if err != nil {
		s.logger.Error("Error updating RouterAccess status",
			slog.String("la_namespace", la.Namespace),
			slog.String("la_name", la.Name),
			slog.Any("error", err))
	} else {
		s.linkAccess[la.Name] = updated
	}
}

func asSecuredAccessSpec(la *skupperv2alpha1.RouterAccess, group string, defaultIssuer string) skupperv2alpha1.SecuredAccessSpec {
	issuer := la.Spec.Issuer
	if issuer == "" {
		issuer = defaultIssuer
	}
	spec := skupperv2alpha1.SecuredAccessSpec{
		AccessType: la.Spec.AccessType,
		Selector: map[string]string{
			"skupper.io/component": "router",
		},
		Certificate: la.Spec.TlsCredentials,
		Issuer:      issuer,
		Options:     la.Spec.Options,
	}
	if group != "" {
		//add extra label to allow for distinct sets of routers in HA
		spec.Selector["skupper.io/group"] = group
	}
	for _, role := range la.Spec.Roles {
		spec.Ports = append(spec.Ports, skupperv2alpha1.SecuredAccessPort{
			Name:       role.Name,
			Port:       role.Port,
			TargetPort: role.Port,
			Protocol:   "TCP",
		})
	}
	return spec
}

func (s *Site) checkSecuredAccess() error {
	groups := s.groups()
	for i, group := range groups {
		for _, la := range s.linkAccess {
			name := la.Name
			if i > 0 {
				name = fmt.Sprintf("%s-%d", la.Name, (i + 1))
			}
			annotations := map[string]string{
				"internal.skupper.io/controlled":   "true",
				"internal.skupper.io/routeraccess": la.Name,
			}
			if err := s.access.Ensure(s.namespace, name, asSecuredAccessSpec(la, group, s.site.DefaultIssuer()), annotations, s.ownerReferences()); err != nil {
				//TODO: add message to site status
				s.logger.Error("Error ensuring SecuredAccess for RouterAccess",
					slog.String("key", la.Key()),
					slog.Any("error", err))
			}
		}
	}
	return nil
}

func (s *Site) CheckRouterAccess(name string, la *skupperv2alpha1.RouterAccess) error {
	specChanged := false
	if la == nil {
		delete(s.linkAccess, name)
		specChanged = true
	} else {
		if existing, ok := s.linkAccess[name]; ok {
			specChanged = !reflect.DeepEqual(existing.Spec, la.Spec)
		}
		s.linkAccess[name] = la
	}
	if !s.initialised {
		return nil
	}
	if specChanged || !la.IsConfigured() {
		var previousGroups []string
		groups := s.groups()
		var errors []string
		for i, group := range groups {
			if err := s.updateRouterConfigForGroup(s.linkAccess.DesiredConfig(previousGroups, SSL_PROFILE_PATH), group); err != nil {
				s.logger.Error("Error updating router config",
					slog.String("namespace", s.namespace),
					slog.Any("error", err))
				errors = append(errors, err.Error())
			}
			if la != nil {
				name := la.Name
				if i > 0 {
					name = fmt.Sprintf("%s-%d", la.Name, (i + 1))
				}
				annotations := map[string]string{
					"internal.skupper.io/controlled":   "true",
					"internal.skupper.io/routeraccess": la.Name,
				}
				if err := s.access.Ensure(s.namespace, name, asSecuredAccessSpec(la, group, s.site.DefaultIssuer()), annotations, s.ownerReferences()); err != nil {
					s.logger.Error("Error ensuring SecuredAccess for RouterAccess",
						slog.String("key", la.Key()),
						slog.Any("error", err))
					errors = append(errors, err.Error())
				}
			}
			previousGroups = append(previousGroups, group)
		}
		var err error
		if len(errors) > 0 {
			err = fmt.Errorf(strings.Join(errors, ", "))
		}
		if la != nil && la.SetConfigured(err) {
			s.updateRouterAccessStatus(la)
		}
	}
	return s.updateResolved()
}

func (s *Site) CheckAttachedConnectorBinding(namespace string, name string, binding *skupperv2alpha1.AttachedConnectorBinding) error {
	return s.bindings.checkAttachedConnectorBinding(namespace, name, binding)
}

func (s *Site) AttachedConnectorUpdated(connector *skupperv2alpha1.AttachedConnector) error {
	return s.bindings.attachedConnectorUpdated(connector.Name, connector)
}

func (s *Site) AttachedConnectorDeleted(namespace string, name string) error {
	return s.bindings.attachedConnectorDeleted(name, namespace)
}

func (s *Site) GetSite() *skupperv2alpha1.Site {
	return s.site
}

func (s *Site) RouterPodEvent(key string, pod *corev1.Pod) error {
	if pod == nil {
		delete(s.routerPods, key)
	} else {
		s.routerPods[key] = pod
	}
	if s.site == nil {
		return nil
	}
	if s.site.SetRunning(s.isRouterPodRunning()) {
		return s.updateSiteStatus()
	}
	return nil
}

func (s *Site) isRouterPodRunning() skupperv2alpha1.ConditionState {
	state := skupperv2alpha1.PendingCondition("No router pod is ready")
	for _, pod := range s.routerPods {
		if internalclient.IsPodRunning(pod) && internalclient.IsPodReady(pod) {
			return skupperv2alpha1.ReadyCondition()
		} else {
			state = podState(pod)
		}
	}
	return state
}

func podState(pod *corev1.Pod) skupperv2alpha1.ConditionState {
	for _, c := range pod.Status.Conditions {
		if c.Status == corev1.ConditionFalse {
			return skupperv2alpha1.PendingCondition(c.Message)
		}
	}
	return skupperv2alpha1.PendingCondition(fmt.Sprintf("Pod %s not ready", pod.Name))
}

type ConfigUpdateList []qdr.ConfigUpdate

func (l ConfigUpdateList) Apply(config *qdr.RouterConfig) bool {
	updated := false
	for _, u := range l {
		if u.Apply(config) {
			updated = true
		}
	}
	return updated
}

func updateConnectorStatus(client internalclient.Clients, connector *skupperv2alpha1.Connector) (*skupperv2alpha1.Connector, error) {
	return client.GetSkupperClient().SkupperV2alpha1().Connectors(connector.ObjectMeta.Namespace).UpdateStatus(context.TODO(), connector, metav1.UpdateOptions{})
}

func updateListenerStatus(client internalclient.Clients, listener *skupperv2alpha1.Listener) (*skupperv2alpha1.Listener, error) {
	return client.GetSkupperClient().SkupperV2alpha1().Listeners(listener.ObjectMeta.Namespace).UpdateStatus(context.TODO(), listener, metav1.UpdateOptions{})
}

func getLabelsForRouter() map[string]string {
	return map[string]string{
		"application":          types.TransportDeploymentName,
		"skupper.io/component": "router",
	}
}

func equivalentSelectors(a map[string]string, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if v2, ok := b[k]; !ok || v != v2 {
			return false
		}
	}
	for k, v := range b {
		if v2, ok := a[k]; !ok || v != v2 {
			return false
		}
	}
	return true
}

func updateSelectorFromMap(spec *corev1.ServiceSpec, desired map[string]string) bool {
	if !equivalentSelectors(spec.Selector, desired) {
		spec.Selector = desired
		return true
	}
	return false
}
