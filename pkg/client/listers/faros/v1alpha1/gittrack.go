/*
Copyright 2018 Pusher Ltd.

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

// Code generated by main. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// GitTrackLister helps list GitTracks.
type GitTrackLister interface {
	// List lists all GitTracks in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.GitTrack, err error)
	// GitTracks returns an object that can list and get GitTracks.
	GitTracks(namespace string) GitTrackNamespaceLister
	GitTrackListerExpansion
}

// gitTrackLister implements the GitTrackLister interface.
type gitTrackLister struct {
	indexer cache.Indexer
}

// NewGitTrackLister returns a new GitTrackLister.
func NewGitTrackLister(indexer cache.Indexer) GitTrackLister {
	return &gitTrackLister{indexer: indexer}
}

// List lists all GitTracks in the indexer.
func (s *gitTrackLister) List(selector labels.Selector) (ret []*v1alpha1.GitTrack, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.GitTrack))
	})
	return ret, err
}

// GitTracks returns an object that can list and get GitTracks.
func (s *gitTrackLister) GitTracks(namespace string) GitTrackNamespaceLister {
	return gitTrackNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// GitTrackNamespaceLister helps list and get GitTracks.
type GitTrackNamespaceLister interface {
	// List lists all GitTracks in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.GitTrack, err error)
	// Get retrieves the GitTrack from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.GitTrack, error)
	GitTrackNamespaceListerExpansion
}

// gitTrackNamespaceLister implements the GitTrackNamespaceLister
// interface.
type gitTrackNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all GitTracks in the indexer for a given namespace.
func (s gitTrackNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.GitTrack, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.GitTrack))
	})
	return ret, err
}

// Get retrieves the GitTrack from the indexer for a given namespace and name.
func (s gitTrackNamespaceLister) Get(name string) (*v1alpha1.GitTrack, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("gittrack"), name)
	}
	return obj.(*v1alpha1.GitTrack), nil
}