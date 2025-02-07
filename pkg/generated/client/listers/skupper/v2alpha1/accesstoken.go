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

// Code generated by lister-gen. DO NOT EDIT.

package v2alpha1

import (
	v2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/listers"
	"k8s.io/client-go/tools/cache"
)

// AccessTokenLister helps list AccessTokens.
// All objects returned here must be treated as read-only.
type AccessTokenLister interface {
	// List lists all AccessTokens in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v2alpha1.AccessToken, err error)
	// AccessTokens returns an object that can list and get AccessTokens.
	AccessTokens(namespace string) AccessTokenNamespaceLister
	AccessTokenListerExpansion
}

// accessTokenLister implements the AccessTokenLister interface.
type accessTokenLister struct {
	listers.ResourceIndexer[*v2alpha1.AccessToken]
}

// NewAccessTokenLister returns a new AccessTokenLister.
func NewAccessTokenLister(indexer cache.Indexer) AccessTokenLister {
	return &accessTokenLister{listers.New[*v2alpha1.AccessToken](indexer, v2alpha1.Resource("accesstoken"))}
}

// AccessTokens returns an object that can list and get AccessTokens.
func (s *accessTokenLister) AccessTokens(namespace string) AccessTokenNamespaceLister {
	return accessTokenNamespaceLister{listers.NewNamespaced[*v2alpha1.AccessToken](s.ResourceIndexer, namespace)}
}

// AccessTokenNamespaceLister helps list and get AccessTokens.
// All objects returned here must be treated as read-only.
type AccessTokenNamespaceLister interface {
	// List lists all AccessTokens in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v2alpha1.AccessToken, err error)
	// Get retrieves the AccessToken from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v2alpha1.AccessToken, error)
	AccessTokenNamespaceListerExpansion
}

// accessTokenNamespaceLister implements the AccessTokenNamespaceLister
// interface.
type accessTokenNamespaceLister struct {
	listers.ResourceIndexer[*v2alpha1.AccessToken]
}
