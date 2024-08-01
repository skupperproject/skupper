package site

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/kube/certificates"
	kubeqdr "github.com/skupperproject/skupper/pkg/kube/qdr"
	"github.com/skupperproject/skupper/pkg/kube/securedaccess"
	"github.com/skupperproject/skupper/pkg/kube/site/resources"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/site"
	"github.com/skupperproject/skupper/pkg/version"
)

type Site struct {
	initialised bool
	site        *skupperv1alpha1.Site
	name        string
	namespace   string
	controller  *kube.Controller
	bindings    *site.Bindings
	links       map[string]*site.Link
	errors      map[string]string
	linkAccess  site.RouterAccessMap
	certs       certificates.CertificateManager
	access      securedaccess.Factory
	adaptor     BindingAdaptor
	routerPods  map[string]*corev1.Pod
}

func NewSite(namespace string, controller *kube.Controller, certs certificates.CertificateManager, access securedaccess.Factory) *Site {
	return &Site{
		bindings:   site.NewBindings(),
		namespace:  namespace,
		controller: controller,
		links:      map[string]*site.Link{},
		linkAccess: site.RouterAccessMap{},
		certs:      certs,
		access:     access,
		routerPods: map[string]*corev1.Pod{},
	}
}

func (s *Site) Recover(site *skupperv1alpha1.Site) error {
	//TODO: check version and perform any necessary update tasks
	return s.Reconcile(site)
}

func (s *Site) isEdge() bool {
	return s.routerMode() == qdr.ModeEdge
}

func (s *Site) routerMode() qdr.Mode {
	if s.site != nil && s.site.Spec.RouterMode == string(qdr.ModeEdge) {
		return qdr.ModeEdge
	} else {
		return qdr.ModeInterior
	}
}

const SSL_PROFILE_PATH = "/etc/skupper-router-certs"

func (s *Site) Reconcile(siteDef *skupperv1alpha1.Site) error {
	err := s.reconcile(siteDef)
	return s.updateConfigured(err)
}

func (s *Site) reconcile(siteDef *skupperv1alpha1.Site) error {
	if s.site != nil && s.site.Name != siteDef.Name {
		log.Printf("Rejecting site %s/%s as %s is already active", siteDef.Namespace, siteDef.Name, s.site.Name)
		return s.markSiteInactive(siteDef, fmt.Errorf("An active site already exists in the namespace (%s)", s.site.Name))
	}
	s.site = siteDef
	s.name = string(siteDef.ObjectMeta.Name)
	log.Printf("Checking site %s/%s (uid %s)", siteDef.Namespace, siteDef.Name, s.site.GetSiteId())
	// ensure necessary resources:
	// 1. skupper-internal configmap
	if !s.initialised {
		log.Printf("Initialising site %s/%s", siteDef.Namespace, siteDef.Name)
		routerConfig, err := s.getRouterConfig()
		if err != nil {
			return err
		}
		createRouterConfig := false
		if routerConfig == nil {
			createRouterConfig = true
			rc := qdr.InitialConfig(s.name+"-${HOSTNAME}", s.site.GetSiteId(), version.Version, s.isEdge(), 3)
			rc.AddAddress(qdr.Address{
				Prefix:       "mc",
				Distribution: "multicast",
			})
			rc.SetNormalListeners(SSL_PROFILE_PATH)
			routerConfig = &rc
		}
		s.initialised = true
		s.adaptor.init(s, routerConfig)
		s.bindings.SetBindingEventHandler(&s.adaptor)
		s.bindings.SetConnectorConfiguration(s.adaptor.updateBridgeConfigForConnector)
		s.bindings.SetListenerConfiguration(s.adaptor.updateBridgeConfigForListener)
		if createRouterConfig {
			s.bindings.Apply(routerConfig)
			//TODO: apply any recovered RouterAccess configuration
			err = s.createRouterConfig(routerConfig)
			if err != nil {
				return err
			}
			log.Printf("Router config created for site %s/%s", siteDef.Namespace, siteDef.Name)
		} else {
			//TODO: include any RouterAccess configuration
			if err := s.updateRouterConfigForGroups(ConfigUpdateList{s.bindings, s}); err != nil {
				return err
			}
		}
		s.checkSecuredAccess()
	} else {
		if err := s.updateRouterConfigForGroups(s); err != nil {
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

func (s *Site) groups() []string {
	if s.site.Spec.HA {
		return []string{"skupper-router-1", "skupper-router-2"}
	} else {
		return []string{"skupper-router"}
	}
}

func (s *Site) checkDefaultRouterAccess(ctxt context.Context, site *skupperv1alpha1.Site) error {
	if site.Spec.LinkAccess == "" || site.Spec.LinkAccess == "none" {
		return nil
	}
	name := "skupper-router"
	accessType := site.Spec.LinkAccess
	if site.Spec.LinkAccess == "default" {
		accessType = ""
	}
	desired := &skupperv1alpha1.RouterAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "RouterAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			OwnerReferences: s.ownerReferences(),
			Annotations: map[string]string{
				"internal.skupper.io/controlled": "true",
			},
		},
		Spec: skupperv1alpha1.RouterAccessSpec{
			AccessType:             accessType,
			TlsCredentials:         "skupper-site-server",
			Issuer:                 "skupper-site-ca", //TODO: can rely ondefault here
			GenerateTlsCredentials: true,
			Roles: []skupperv1alpha1.RouterAccessRole{
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
		updated, err := s.controller.GetSkupperClient().SkupperV1alpha1().RouterAccesses(s.namespace).Update(context.Background(), current, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		s.linkAccess[name] = updated
		return nil
	} else {
		created, err := s.controller.GetSkupperClient().SkupperV1alpha1().RouterAccesses(s.namespace).Create(context.Background(), desired, metav1.CreateOptions{})
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
	}
	available := kube.GetSupportedIngressResources(s.controller.GetDiscoveryClient())
	for _, resource := range available {
		//needed for determining token urls
		rules = append(rules, rbacv1.PolicyRule{
			Verbs:     []string{"get", "list", "watch"},
			APIGroups: []string{resource.Group},
			Resources: []string{resource.Resource},
		})
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

func (s *Site) endpoints() []skupperv1alpha1.Endpoint {
	var endpoints []skupperv1alpha1.Endpoint
	for _, la := range s.linkAccess {
		for _, endpoint := range la.Status.Endpoints {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints
}

func (s *Site) recordError(key string, detail string) {

}

func (s *Site) clearError(key string) {

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
		log.Printf("Invalid value for router logging in settings for %s/%s", s.namespace, s.name)
	}
	return updated
}

func (s *Site) IsInitialised() bool {
	return s.initialised
}

func (s *Site) Select(connector *skupperv1alpha1.Connector) TargetSelection {
	name := connector.Name
	selector := connector.Spec.Selector
	includeNotReady := connector.Spec.IncludeNotReady
	if selector == "" {
		return nil
	}
	handler := &TargetSelectionImpl{
		stopCh:          make(chan struct{}),
		site:            s,
		name:            name,
		selector:        selector,
		namespace:       s.namespace,
		includeNotReady: includeNotReady,
	}
	log.Printf("Watching pods matching %s in %s for %s", selector, s.namespace, name)
	handler.watcher = s.controller.WatchPods(selector, s.namespace, handler.handle)
	handler.watcher.Start(handler.stopCh)

	return handler
}

func (s *Site) Expose(exposed *ExposedPortSet) {
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
				Selector: kube.GetLabelsForRouter(), //TODO: handle external bridges
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
				log.Printf("Error creating service %q in %q: %s", exposed.Host, s.namespace, err)
			} else {
				log.Printf("Created service %q in %q", exposed.Host, s.namespace)
			}
		} else {
			log.Printf("Did not create service %q in %q as ports were not updated", exposed.Host, s.namespace)
		}
	} else if err != nil {
		log.Printf("Error checking service %q in %q: %s", exposed.Host, s.namespace, err)
	} else {
		updated := false
		if kube.UpdateSelectorFromMap(&current.Spec, kube.GetLabelsForRouter()) {
			updated = true
		}
		if updatePorts(&current.Spec, exposed.Ports) {
			updated = true
		}
		//TODO: update labels and annotations
		if updated {
			_, err := s.controller.GetKubeClient().CoreV1().Services(s.namespace).Update(ctxt, current, metav1.UpdateOptions{})
			if err != nil {
				log.Printf("Error creating service %q in %q: %s", exposed.Host, s.namespace, err)
			} else {
				log.Printf("Updated service %q in %q", exposed.Host, s.namespace)
			}
		}
	}
}

func (s *Site) Unexpose(name string) {
	ctxt := context.TODO()
	current, err := s.controller.GetKubeClient().CoreV1().Services(s.namespace).Get(ctxt, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			log.Printf("Error cekcing service %s to be deleted from %s: %s", name, s.namespace, err)
		}
	} else if isOwned(current) {
		err = s.controller.GetKubeClient().CoreV1().Services(s.namespace).Delete(ctxt, name, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("Error deleting service %s in %s: %s", name, s.namespace, err)
		}
		//TODO: ideally error should be propagated back to controller loop
	}
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
			APIVersion: "skupper.io/v1alpha1",
			Name:       s.name,
			UID:        s.site.ObjectMeta.UID,
		},
	}
}

func (s *Site) getRouterConfig() (*qdr.RouterConfig, error) {
	current, err := s.controller.GetKubeClient().CoreV1().ConfigMaps(s.namespace).Get(context.TODO(), s.groups()[0], metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return qdr.GetRouterConfigFromConfigMap(current)
}

func (s *Site) createRouterConfig(config *qdr.RouterConfig) error {
	for _, group := range s.groups() {
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
			},
			Data: data,
		}
		if _, err = s.controller.GetKubeClient().CoreV1().ConfigMaps(s.namespace).Create(context.TODO(), cm, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create config map %s/%s: %s", s.namespace, group, err)
			return err
		} else {
			log.Printf("Config map %s/%s created successfully", s.namespace, group)
		}
	}
	return nil
}

func (s *Site) updateRouterConfigForGroups(update qdr.ConfigUpdate) error {
	for _, group := range s.groups() {
		if err := s.updateRouterConfig(update, group); err != nil {
			return err
		}
	}
	log.Printf("Router config updated for site %s/%s", s.namespace, s.name)
	return nil
}

func (s *Site) updateRouterConfig(update qdr.ConfigUpdate, group string) error {
	if !s.initialised {
		log.Printf("Cannot update router config for site in %s", s.namespace)
		return nil
	}
	if err := kubeqdr.UpdateRouterConfig(s.controller.GetKubeClient(), group, s.namespace, context.TODO(), update); err != nil {
		return err
	}
	return nil
}

func (s *Site) updateConnectorStatus(connector *skupperv1alpha1.Connector) error {
	updated, err := updateConnectorStatus(s.controller, connector)
	if err != nil {
		return err
	}
	s.bindings.UpdateConnector(updated.Name, updated)
	return nil
}

func (s *Site) updateConnectorConfiguredStatus(connector *skupperv1alpha1.Connector, err error) error {
	if connector.SetConfigured(err) {
		return s.updateConnectorStatus(connector)
	}
	return nil
}

func (s *Site) updateConnectorConfiguredStatusWithSelectedPods(connector *skupperv1alpha1.Connector, selected []skupperv1alpha1.PodDetails) error {
	var err error
	if len(selected) == 0 {
		log.Printf("No pods selected for %s/%s", connector.Namespace, connector.Name)
		err = fmt.Errorf("No pods match selector")
	} else {

	}
	if connector.SetConfigured(err) || connector.SetSelectedPods(selected) {
		return s.updateConnectorStatus(connector)
	}
	return nil
}

func (s *Site) CheckConnector(name string, connector *skupperv1alpha1.Connector) error {
	update := s.bindings.UpdateConnector(name, connector)
	if update == nil || s.site == nil {
		return nil
	}
	err := s.updateRouterConfigForGroups(update)
	if connector == nil {
		return err
	}
	return s.updateConnectorConfiguredStatus(connector, err)
}

func (s *Site) updateListenerStatus(listener *skupperv1alpha1.Listener, err error) error {
	if listener.SetConfigured(err) {
		updated, err := s.controller.GetSkupperClient().SkupperV1alpha1().Listeners(listener.ObjectMeta.Namespace).UpdateStatus(context.TODO(), listener, metav1.UpdateOptions{})
		if err == nil {
			return err
		}
		s.bindings.UpdateListener(updated.Name, updated)
	}
	return err
}

func (s *Site) CheckListener(name string, listener *skupperv1alpha1.Listener) error {
	update := s.bindings.UpdateListener(name, listener)
	if update == nil || s.site == nil {
		return nil
	}
	err := s.updateRouterConfigForGroups(update)
	if listener == nil {
		return err
	}
	return s.updateListenerStatus(listener, err)
}

func (s *Site) newLink(linkconfig *skupperv1alpha1.Link) *site.Link {
	config := site.NewLink(linkconfig.ObjectMeta.Name, SSL_PROFILE_PATH)
	config.Update(linkconfig)
	return config
}

func (s *Site) CheckLink(name string, linkconfig *skupperv1alpha1.Link) error {
	log.Printf("checkLink(%s)", name)
	if linkconfig == nil {
		return s.unlink(name)
	}
	return s.link(linkconfig)
}

func (s *Site) link(linkconfig *skupperv1alpha1.Link) error {
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
			log.Printf("Connecting site in %s using token %s", s.namespace, linkconfig.ObjectMeta.Name)
			err := s.updateRouterConfigForGroups(config)
			config.UpdateStatus(linkconfig)
			return s.updateLinkStatus(linkconfig, err)
		} else {
			log.Printf("No update to router config required for link %s in %s", linkconfig.ObjectMeta.Name, linkconfig.ObjectMeta.Namespace)
		}
	} else {
		log.Printf("Site is not yet initialised, cannot configure router for link %s in %s", linkconfig.ObjectMeta.Name, linkconfig.ObjectMeta.Namespace)
	}
	return nil
}

func (s *Site) unlink(name string) error {
	if _, ok := s.links[name]; ok {
		log.Printf("Disconnecting connector %s from site in %s", name, s.namespace)
		delete(s.links, name)
		if s.initialised {
			return s.updateRouterConfigForGroups(site.NewRemoveConnector(name))
		}
	}
	return nil
}

func (s *Site) updateLinkStatus(link *skupperv1alpha1.Link, err error) error {
	if link == nil {
		return nil
	}
	if link.SetConfigured(err) {
		updated, updateErr := s.controller.GetSkupperClient().SkupperV1alpha1().Links(link.ObjectMeta.Namespace).UpdateStatus(context.TODO(), link, metav1.UpdateOptions{})
		if updateErr != nil {
			return updateErr
		}
		s.links[link.ObjectMeta.Name].Update(updated)
	}
	return nil
}

func (s *Site) Deleted() {
	s.adaptor.cleanup()
}

func (s *Site) setDefaultIssuerInStatus() bool {
	if issuer := s.defaultIssuer(); s.site.Status.DefaultIssuer != issuer {
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
	}
	if changed {
		return s.updateSiteStatus()
	}
	return nil
}

func (s *Site) updateResolved() error {
	endpoints := s.endpoints()
	changed := false
	if !reflect.DeepEqual(s.site.Status.Endpoints, endpoints) {
		s.site.Status.Endpoints = endpoints
		changed = true
	}
	if len(endpoints) > 0 && s.site.SetResolved(nil) {
		changed = true
	}
	if changed {
		return s.updateSiteStatus()
	}
	return nil
}

func (s *Site) updateSiteStatus() error {
	updated, err := s.controller.GetSkupperClient().SkupperV1alpha1().Sites(s.site.ObjectMeta.Namespace).UpdateStatus(context.TODO(), s.site, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	s.site = updated
	return nil
}

func (s *Site) NetworkStatusUpdated(network []skupperv1alpha1.SiteRecord) error {
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

	bindingStatus := newBindingStatus(s.controller, network)
	s.bindings.Map(bindingStatus.updateMatchingListenerCount, bindingStatus.updateMatchingConnectorCount)
	return bindingStatus.error()
}

func (s *Site) markSiteInactive(site *skupperv1alpha1.Site, err error) error {
	if site.SetConfigured(err) {
		if _, err := s.UpdateSiteStatus(site); err != nil {
			return err
		}
	}
	return nil
}

func (s *Site) UpdateSiteStatus(site *skupperv1alpha1.Site) (*skupperv1alpha1.Site, error) {
	updated, err := s.controller.GetSkupperClient().SkupperV1alpha1().Sites(site.ObjectMeta.Namespace).UpdateStatus(context.TODO(), site, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Site) CheckSecuredAccess(sa *skupperv1alpha1.SecuredAccess) {
	log.Printf("Checking SecuredAccess %s", sa.Name)
	name, ok := sa.ObjectMeta.Annotations["internal.skupper.io/routeraccess"]
	if !ok {
		name = sa.Name
	}
	la, ok := s.linkAccess[name]
	if !ok {
		log.Printf("No RouterAccess %s found for SecuredAccess %s", name, sa.Name)
		return
	}
	if la.Resolve(sa.Status.Endpoints, sa.Name) {
		s.updateRouterAccessStatus(la)
	}
}

func (s *Site) updateRouterAccessStatus(la *skupperv1alpha1.RouterAccess) {
	updated, err := s.controller.GetSkupperClient().SkupperV1alpha1().RouterAccesses(la.Namespace).UpdateStatus(context.TODO(), la, metav1.UpdateOptions{})
	if err != nil {
		log.Printf("Error updating RouterAccess status for %s/%s: %s", la.Namespace, la.Name, err)
	} else {
		s.linkAccess[la.Name] = updated
	}
}

func (s *Site) defaultIssuer() string {
	return site.DefaultIssuer(s.site)
}

func asSecuredAccessSpec(la *skupperv1alpha1.RouterAccess, group string, defaultIssuer string) skupperv1alpha1.SecuredAccessSpec {
	issuer := la.Spec.Issuer
	if issuer == "" {
		issuer = defaultIssuer
	}
	spec := skupperv1alpha1.SecuredAccessSpec{
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
		spec.Ports = append(spec.Ports, skupperv1alpha1.SecuredAccessPort{
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
			if len(groups) > 0 {
				name = fmt.Sprintf("%s-%d", la.Name, (i + 1))
			}
			annotations := map[string]string{
				"internal.skupper.io/controlled":   "true",
				"internal.skupper.io/routeraccess": la.Name,
			}
			if err := s.access.Ensure(s.namespace, name, asSecuredAccessSpec(la, group, s.defaultIssuer()), annotations, s.ownerReferences()); err != nil {
				//TODO: add message to site status
				log.Printf("Error ensuring SecuredAccess for RouterAccess %s: %s", la.Key(), err)
			}
		}
	}
	return nil
}

func (s *Site) CheckRouterAccess(name string, la *skupperv1alpha1.RouterAccess) error {
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
			if err := s.updateRouterConfig(s.linkAccess.DesiredConfig(previousGroups, SSL_PROFILE_PATH), group); err != nil {
				log.Printf("Error updating router config for %s: %s", s.namespace, err)
				errors = append(errors, err.Error())
			}
			if la != nil {
				name := la.Name
				if len(groups) > 0 {
					name = fmt.Sprintf("%s-%d", la.Name, (i + 1))
				}
				annotations := map[string]string{
					"internal.skupper.io/controlled":   "true",
					"internal.skupper.io/routeraccess": la.Name,
				}
				if err := s.access.Ensure(s.namespace, name, asSecuredAccessSpec(la, group, s.defaultIssuer()), annotations, s.ownerReferences()); err != nil {
					log.Printf("Error ensuring SecuredAccess for RouterAccess %s: %s", la.Key(), err)
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

func (s *Site) GetSite() *skupperv1alpha1.Site {
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

func (s *Site) isRouterPodRunning() error {
	if len(s.routerPods) == 0 {
		return fmt.Errorf("No router pod created in %s", s.namespace)
	}
	var err error
	for _, pod := range s.routerPods {
		if kube.IsPodRunning(pod) && kube.IsPodReady(pod) {
			return nil
		} else {
			err = podError(pod)
		}
	}
	if err == nil {
		err = fmt.Errorf("No router pod is ready")
	}
	return err
}

func podError(pod *corev1.Pod) error {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			var err error
			if c.Reason == "ContainersNotReady" {
				err = containerError(pod)
			}
			if err == nil {
				err = fmt.Errorf(c.Message)
			}
			return err
		}
	}
	return fmt.Errorf("Pod %s not ready", pod.Name)
}

func containerError(pod *corev1.Pod) error {
	for _, s := range pod.Status.ContainerStatuses {
		if s.State.Waiting != nil {
			return fmt.Errorf(s.State.Waiting.Message)
		}
	}
	return nil
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

func updateConnectorStatus(client internalclient.Clients, connector *skupperv1alpha1.Connector) (*skupperv1alpha1.Connector, error) {
	return client.GetSkupperClient().SkupperV1alpha1().Connectors(connector.ObjectMeta.Namespace).UpdateStatus(context.TODO(), connector, metav1.UpdateOptions{})
}

func updateListenerStatus(client internalclient.Clients, listener *skupperv1alpha1.Listener) (*skupperv1alpha1.Listener, error) {
	return client.GetSkupperClient().SkupperV1alpha1().Listeners(listener.ObjectMeta.Namespace).UpdateStatus(context.TODO(), listener, metav1.UpdateOptions{})
}
