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

	v1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeAccessTokens implements AccessTokenInterface
type FakeAccessTokens struct {
	Fake *FakeSkupperV1alpha1
	ns   string
}

var accesstokensResource = schema.GroupVersionResource{Group: "skupper.io", Version: "v1alpha1", Resource: "accesstokens"}

var accesstokensKind = schema.GroupVersionKind{Group: "skupper.io", Version: "v1alpha1", Kind: "AccessToken"}

// Get takes name of the accessToken, and returns the corresponding accessToken object, and an error if there is any.
func (c *FakeAccessTokens) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.AccessToken, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(accesstokensResource, c.ns, name), &v1alpha1.AccessToken{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AccessToken), err
}

// List takes label and field selectors, and returns the list of AccessTokens that match those selectors.
func (c *FakeAccessTokens) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.AccessTokenList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(accesstokensResource, accesstokensKind, c.ns, opts), &v1alpha1.AccessTokenList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.AccessTokenList{ListMeta: obj.(*v1alpha1.AccessTokenList).ListMeta}
	for _, item := range obj.(*v1alpha1.AccessTokenList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested accessTokens.
func (c *FakeAccessTokens) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(accesstokensResource, c.ns, opts))

}

// Create takes the representation of a accessToken and creates it.  Returns the server's representation of the accessToken, and an error, if there is any.
func (c *FakeAccessTokens) Create(ctx context.Context, accessToken *v1alpha1.AccessToken, opts v1.CreateOptions) (result *v1alpha1.AccessToken, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(accesstokensResource, c.ns, accessToken), &v1alpha1.AccessToken{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AccessToken), err
}

// Update takes the representation of a accessToken and updates it. Returns the server's representation of the accessToken, and an error, if there is any.
func (c *FakeAccessTokens) Update(ctx context.Context, accessToken *v1alpha1.AccessToken, opts v1.UpdateOptions) (result *v1alpha1.AccessToken, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(accesstokensResource, c.ns, accessToken), &v1alpha1.AccessToken{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AccessToken), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeAccessTokens) UpdateStatus(ctx context.Context, accessToken *v1alpha1.AccessToken, opts v1.UpdateOptions) (*v1alpha1.AccessToken, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(accesstokensResource, "status", c.ns, accessToken), &v1alpha1.AccessToken{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AccessToken), err
}

// Delete takes name of the accessToken and deletes it. Returns an error if one occurs.
func (c *FakeAccessTokens) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(accesstokensResource, c.ns, name), &v1alpha1.AccessToken{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeAccessTokens) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(accesstokensResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.AccessTokenList{})
	return err
}

// Patch applies the patch and returns the patched accessToken.
func (c *FakeAccessTokens) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.AccessToken, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(accesstokensResource, c.ns, name, pt, data, subresources...), &v1alpha1.AccessToken{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AccessToken), err
}