package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/skupperproject/skupper/api/types"
	v1alpha12 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils"
	apiv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/fake"
	authv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

type PolicyValidationResult struct {
	err             error
	enabled         bool
	matchingAllowed []v1alpha12.SkupperClusterPolicy
}

func (p *PolicyValidationResult) Enabled() bool {
	return p.enabled
}

func (p *PolicyValidationResult) Allowed() bool {
	return !p.Enabled() || p.err == nil && len(p.matchingAllowed) > 0
}

func (p *PolicyValidationResult) AllowPolicies() []v1alpha12.SkupperClusterPolicy {
	return p.matchingAllowed
}

func (p *PolicyValidationResult) AllowPolicyNames() []string {
	var names []string
	for _, p := range p.matchingAllowed {
		names = append(names, p.Name)
	}
	return names
}

func (p *PolicyValidationResult) Error() error {
	return p.err
}

// ClusterPolicyValidator The policy validator component must be
// used internally by the service-controller only. Client applications
// must use the PolicyAPIClient (rest client).
type ClusterPolicyValidator struct {
	cli                    *VanClient
	dc                     *discovery.DiscoveryClient
	skupperPolicy          v1alpha1.SkupperClusterPolicyInterface
	disablePolicyDiscovery bool
	staticPolicyList       []v1alpha12.SkupperClusterPolicy
}

func NewClusterPolicyValidator(cli *VanClient) *ClusterPolicyValidator {
	return &ClusterPolicyValidator{
		cli: cli,
	}
}

func (p *PolicyValidationResult) addMatchingPolicy(policy v1alpha12.SkupperClusterPolicy) {
	p.matchingAllowed = append(p.matchingAllowed, policy)
}

func (p *ClusterPolicyValidator) getSkupperPolicy() (v1alpha1.SkupperClusterPolicyInterface, error) {
	if p.skupperPolicy == nil {
		if p.cli.RestConfig == nil {
			return nil, fmt.Errorf("RestConfig not defined")
		}
		skupperCli, err := v1alpha1.NewForConfig(p.cli.RestConfig)
		if err != nil {
			return nil, err
		}
		p.skupperPolicy = skupperCli.SkupperClusterPolicies()
	}
	return p.skupperPolicy, nil
}

func (p *ClusterPolicyValidator) SetStaticPolicyList(policies []v1alpha12.SkupperClusterPolicy) {
	p.staticPolicyList = policies
}

func (p *ClusterPolicyValidator) LoadNamespacePolicies() ([]v1alpha12.SkupperClusterPolicy, error) {
	if p.staticPolicyList != nil {
		return p.staticPolicyList, nil
	}
	policies := []v1alpha12.SkupperClusterPolicy{}
	skupperPolicy, err := p.getSkupperPolicy()
	if err != nil {
		if _, mock := p.cli.KubeClient.(*fake.Clientset); mock {
			return policies, err
		}
		return policies, nil
	}

	policyList, err := skupperPolicy.List(v1.ListOptions{})
	if err != nil {
		if errors.IsForbidden(err) || !p.CrdDefined() {
			return policies, nil
		}
		return policies, err
	}
	namespace, _ := p.cli.KubeClient.CoreV1().Namespaces().Get(p.cli.Namespace, v1.GetOptions{})
	for _, pol := range policyList.Items {
		if len(pol.Name) > 0 && p.appliesToNS(&pol, namespace) {
			policies = append(policies, pol)
		}
	}
	return policies, nil
}

func (p *ClusterPolicyValidator) AppliesToNS(policyName string) bool {
	skupperPolicy, err := p.getSkupperPolicy()
	// If unable to determine, revalidate
	if err != nil {
		return true
	}
	pol, err := skupperPolicy.Get(policyName, v1.GetOptions{})
	// If policy not found, revalidate
	if err != nil {
		return true
	}
	namespace, _ := p.cli.KubeClient.CoreV1().Namespaces().Get(p.cli.Namespace, v1.GetOptions{})
	return p.appliesToNS(pol, namespace)
}

func (p *ClusterPolicyValidator) appliesToNS(pol *v1alpha12.SkupperClusterPolicy, namespace *corev1.Namespace) bool {
	if utils.StringSliceContains(pol.Spec.Namespaces, "*") {
		return true
	}
	hasNsLabels := namespace != nil && len(namespace.Name) > 0 && len(namespace.Labels) > 0
	for _, ns := range pol.Spec.Namespaces {
		if match, err := regexp.Match(ns, []byte(p.cli.Namespace)); err == nil && match {
			return true
		}
		if selector, err := labels.Parse(ns); err == nil && hasNsLabels {
			if selector.Matches(labels.Set(namespace.Labels)) {
				return true
			}
		}
	}
	return false
}

func (p *ClusterPolicyValidator) Enabled() bool {
	if p.cli.RestConfig == nil {
		return false
	}
	return p.CrdDefined()
}

func (p *ClusterPolicyValidator) HasPermission() bool {
	authCli, err := authv1.NewForConfig(p.cli.RestConfig)
	sar, err := authCli.SelfSubjectAccessReviews().Create(&apiv1.SelfSubjectAccessReview{
		Spec: apiv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &apiv1.ResourceAttributes{
				Verb:     "list",
				Group:    "skupper.io",
				Version:  "v1alpha1",
				Resource: "skupperclusterpolicies",
			},
		},
	})
	return err == nil && sar.Status.Allowed
}

func (p *ClusterPolicyValidator) CrdDefined() bool {
	if p.disablePolicyDiscovery {
		return true
	}
	if p.dc == nil {
		if p.cli.RestConfig == nil {
			return false
		}
		dc, err := discovery.NewDiscoveryClientForConfig(p.cli.RestConfig)
		if err != nil {
			log.Printf("Cannot determine if policy is enabled: %v", err)
			return false
		}
		p.dc = dc
	}
	resources, err := p.dc.ServerResourcesForGroupVersion("skupper.io/v1alpha1")
	if errors.IsNotFound(err) {
		return false
	} else if err != nil {
		log.Printf("Cannot determine if policy is enabled: %v", err)
		return false
	} else {
		for _, resource := range resources.APIResources {
			if resource.Kind == "SkupperClusterPolicy" {
				return true
			}
		}
	}
	return false
}

func (p *ClusterPolicyValidator) policyValidationResult(err error) *PolicyValidationResult {
	return &PolicyValidationResult{
		err:     err,
		enabled: p.CrdDefined(),
	}
}

func (p *ClusterPolicyValidator) ValidateIncomingLink() *PolicyValidationResult {
	policies, err := p.LoadNamespacePolicies()
	res := p.policyValidationResult(err)
	if err != nil || len(policies) == 0 {
		return res
	}

	for _, pol := range policies {
		if pol.Spec.AllowIncomingLinks {
			res.addMatchingPolicy(pol)
		}
	}

	return res
}

func (p *ClusterPolicyValidator) ValidateOutgoingLink(hostname string) *PolicyValidationResult {
	policies, err := p.LoadNamespacePolicies()
	res := p.policyValidationResult(err)
	if err != nil || len(policies) == 0 {
		return res
	}

	for _, pol := range policies {
		if utils.StringSliceContains(pol.Spec.AllowedOutgoingLinksHostnames, "*") {
			res.addMatchingPolicy(pol)
		} else if utils.RegexpStringSliceContains(pol.Spec.AllowedOutgoingLinksHostnames, hostname) {
			res.addMatchingPolicy(pol)
		}
	}

	return res
}

func (p *ClusterPolicyValidator) ValidateExpose(resourceType, resourceName string) *PolicyValidationResult {
	policies, err := p.LoadNamespacePolicies()
	res := p.policyValidationResult(err)
	if err != nil || len(policies) == 0 {
		return res
	}

	resource := resourceType + "/" + resourceName
	for _, pol := range policies {
		if utils.StringSliceContains(pol.Spec.AllowedExposedResources, "*") {
			res.addMatchingPolicy(pol)
		} else if utils.StringSliceContains(pol.Spec.AllowedExposedResources, resource) {
			res.addMatchingPolicy(pol)
		} else if resourceType == "" && utils.StringSliceEndsWith(pol.Spec.AllowedExposedResources, resource) {
			res.addMatchingPolicy(pol)
		}
	}

	return res
}

func (p *ClusterPolicyValidator) ValidateImportService(serviceName string) *PolicyValidationResult {
	policies, err := p.LoadNamespacePolicies()
	res := p.policyValidationResult(err)
	if err != nil || len(policies) == 0 {
		return res
	}

	for _, pol := range policies {
		if utils.StringSliceContains(pol.Spec.AllowedServices, "*") {
			res.addMatchingPolicy(pol)
		} else if utils.RegexpStringSliceContains(pol.Spec.AllowedServices, serviceName) {
			res.addMatchingPolicy(pol)
		}
	}

	return res
}

type PolicyAPIClient struct {
	cli *VanClient
}

type PolicyAPIResult struct {
	Allowed   bool     `json:"allowed"`
	AllowedBy []string `json:"allowedBy"`
	Enabled   bool     `json:"enabled"`
	Error     string   `json:"error"`
}

func NewPolicyValidatorAPI(cli *VanClient) *PolicyAPIClient {
	return &PolicyAPIClient{
		cli: cli,
	}
}

func (p *PolicyAPIClient) execGet(args ...string) (*PolicyAPIResult, error) {
	if _, mock := p.cli.KubeClient.(*fake.Clientset); mock {
		return &PolicyAPIResult{
			Allowed: true,
			Enabled: false,
		}, nil
	}
	ctx, cn := context.WithTimeout(context.Background(), time.Second*30)
	defer cn()
	notEnabledErr := fmt.Errorf("Skupper is not enabled in namespace '%s'", p.cli.Namespace)
	policyRetryInterval := time.Millisecond * 200
	err := utils.RetryWithContext(ctx, policyRetryInterval, func() (bool, error) {
		_, err := p.cli.exec([]string{"get", "policies", "-h"}, p.cli.GetNamespace())
		if err != nil {
			if _, err := getRootObject(p.cli); err != nil && errors.IsNotFound(err) {
				return true, notEnabledErr
			}
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		if err.Error() == "command terminated with exit code 1" {
			// site is running an older version without policy support
			return &PolicyAPIResult{
				Allowed: true,
				Enabled: false,
			}, nil
		}
		if os.IsTimeout(err) {
			err = notEnabledErr
		} else if err != notEnabledErr {
			err = fmt.Errorf("Unable to communicate with the API: %v", err)
		}
		if event.DefaultStore != nil {
			event.Recordf("PolicyAPIError", err.Error())
		}
		return &PolicyAPIResult{
			Allowed: false,
			Enabled: false,
		}, fmt.Errorf("Policy validation error: %v", err)
	}
	fullArgs := []string{"get", "policies"}
	fullArgs = append(fullArgs, args...)
	fullArgs = append(fullArgs, "-o", "json")
	var out *bytes.Buffer
	var retryErr error
	err = utils.RetryWithContext(ctx, policyRetryInterval, func() (bool, error) {
		out, err = p.cli.exec(fullArgs, p.cli.GetNamespace())
		if err != nil {
			if _, err := getRootObject(p.cli); err != nil && errors.IsNotFound(err) {
				return true, notEnabledErr
			}
			retryErr = err
			return false, nil
		}
		retryErr = nil
		return true, nil
	})
	if err != nil {
		if os.IsTimeout(err) {
			return nil, fmt.Errorf("Policy validation error: %s not ready - %v", types.ControllerDeploymentName, retryErr)
		}
		return nil, notEnabledErr
	}
	res := &PolicyAPIResult{}
	err = json.Unmarshal(out.Bytes(), res)
	if err != nil {
		return nil, fmt.Errorf("Policy validation error: %v", err)
	}
	return res, nil
}

func (p *PolicyAPIResult) Err() error {
	if p.Error != "" {
		return fmt.Errorf(p.Error)
	}
	return nil
}

func (p *PolicyAPIClient) Expose(resourceType, resourceName string) (*PolicyAPIResult, error) {
	return p.execGet("expose", resourceType, resourceName)
}

func (p *PolicyAPIClient) Service(name string) (*PolicyAPIResult, error) {
	return p.execGet("service", name)
}

func (p *PolicyAPIClient) Services(names ...string) (map[string]*PolicyAPIResult, error) {
	results := map[string]*PolicyAPIResult{}
	for _, name := range names {
		res, err := p.execGet("service", name)
		if err != nil {
			return results, err
		}
		results[name] = res
	}
	return results, nil
}

func (p *PolicyAPIClient) IncomingLink() (*PolicyAPIResult, error) {
	return p.execGet("incominglink")
}

func (p *PolicyAPIClient) OutgoingLink(hostname string) (*PolicyAPIResult, error) {
	return p.execGet("outgoinglink", hostname)
}
