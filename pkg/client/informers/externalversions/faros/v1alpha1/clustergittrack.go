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
	time "time"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	clientset "github.com/pusher/faros/pkg/client/clientset"
	internalinterfaces "github.com/pusher/faros/pkg/client/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/pusher/faros/pkg/client/listers/faros/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ClusterGitTrackInformer provides access to a shared informer and lister for
// ClusterGitTracks.
type ClusterGitTrackInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.ClusterGitTrackLister
}

type clusterGitTrackInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewClusterGitTrackInformer constructs a new informer for ClusterGitTrack type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewClusterGitTrackInformer(client clientset.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredClusterGitTrackInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredClusterGitTrackInformer constructs a new informer for ClusterGitTrack type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredClusterGitTrackInformer(client clientset.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.FarosV1alpha1().ClusterGitTracks().List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.FarosV1alpha1().ClusterGitTracks().Watch(options)
			},
		},
		&farosv1alpha1.ClusterGitTrack{},
		resyncPeriod,
		indexers,
	)
}

func (f *clusterGitTrackInformer) defaultInformer(client clientset.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredClusterGitTrackInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *clusterGitTrackInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&farosv1alpha1.ClusterGitTrack{}, f.defaultInformer)
}

func (f *clusterGitTrackInformer) Lister() v1alpha1.ClusterGitTrackLister {
	return v1alpha1.NewClusterGitTrackLister(f.Informer().GetIndexer())
}
