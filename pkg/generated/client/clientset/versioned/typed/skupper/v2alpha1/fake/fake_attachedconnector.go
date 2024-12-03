/*
Copyright 2021 The Skupper Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeAttachedConnectors implements AttachedConnectorInterface
type FakeAttachedConnectors struct {
	Fake *FakeSkupperV2alpha1
	ns   string
}

var attachedconnectorsResource = v2alpha1.SchemeGroupVersion.WithResource("attachedconnectors")

var attachedconnectorsKind = v2alpha1.SchemeGroupVersion.WithKind("AttachedConnector")

// Get takes name of the attachedConnector, and returns the corresponding attachedConnector object, and an error if there is any.
func (c *FakeAttachedConnectors) Get(ctx context.Context, name string, options v1.GetOptions) (result *v2alpha1.AttachedConnector, err error) {
	emptyResult := &v2alpha1.AttachedConnector{}
	obj, err := c.Fake.
		Invokes(testing.NewGetActionWithOptions(attachedconnectorsResource, c.ns, name, options), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v2alpha1.AttachedConnector), err
}

// List takes label and field selectors, and returns the list of AttachedConnectors that match those selectors.
func (c *FakeAttachedConnectors) List(ctx context.Context, opts v1.ListOptions) (result *v2alpha1.AttachedConnectorList, err error) {
	emptyResult := &v2alpha1.AttachedConnectorList{}
	obj, err := c.Fake.
		Invokes(testing.NewListActionWithOptions(attachedconnectorsResource, attachedconnectorsKind, c.ns, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v2alpha1.AttachedConnectorList{ListMeta: obj.(*v2alpha1.AttachedConnectorList).ListMeta}
	for _, item := range obj.(*v2alpha1.AttachedConnectorList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested attachedConnectors.
func (c *FakeAttachedConnectors) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchActionWithOptions(attachedconnectorsResource, c.ns, opts))

}

// Create takes the representation of a attachedConnector and creates it.  Returns the server's representation of the attachedConnector, and an error, if there is any.
func (c *FakeAttachedConnectors) Create(ctx context.Context, attachedConnector *v2alpha1.AttachedConnector, opts v1.CreateOptions) (result *v2alpha1.AttachedConnector, err error) {
	emptyResult := &v2alpha1.AttachedConnector{}
	obj, err := c.Fake.
		Invokes(testing.NewCreateActionWithOptions(attachedconnectorsResource, c.ns, attachedConnector, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v2alpha1.AttachedConnector), err
}

// Update takes the representation of a attachedConnector and updates it. Returns the server's representation of the attachedConnector, and an error, if there is any.
func (c *FakeAttachedConnectors) Update(ctx context.Context, attachedConnector *v2alpha1.AttachedConnector, opts v1.UpdateOptions) (result *v2alpha1.AttachedConnector, err error) {
	emptyResult := &v2alpha1.AttachedConnector{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateActionWithOptions(attachedconnectorsResource, c.ns, attachedConnector, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v2alpha1.AttachedConnector), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeAttachedConnectors) UpdateStatus(ctx context.Context, attachedConnector *v2alpha1.AttachedConnector, opts v1.UpdateOptions) (result *v2alpha1.AttachedConnector, err error) {
	emptyResult := &v2alpha1.AttachedConnector{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceActionWithOptions(attachedconnectorsResource, "status", c.ns, attachedConnector, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v2alpha1.AttachedConnector), err
}

// Delete takes name of the attachedConnector and deletes it. Returns an error if one occurs.
func (c *FakeAttachedConnectors) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(attachedconnectorsResource, c.ns, name, opts), &v2alpha1.AttachedConnector{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeAttachedConnectors) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionActionWithOptions(attachedconnectorsResource, c.ns, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v2alpha1.AttachedConnectorList{})
	return err
}

// Patch applies the patch and returns the patched attachedConnector.
func (c *FakeAttachedConnectors) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2alpha1.AttachedConnector, err error) {
	emptyResult := &v2alpha1.AttachedConnector{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(attachedconnectorsResource, c.ns, name, pt, data, opts, subresources...), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v2alpha1.AttachedConnector), err
}
