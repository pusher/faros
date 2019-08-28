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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// API related string constants for Group, Version and Kinds within
// v1alpha1.faros.pusher.com
const (
	Group   = "faros.pusher.com"
	Version = "v1alpha1"

	GitTrackKind              = "GitTrack"
	GitTrackObjectKind        = "GitTrackObject"
	ClusterGitTrackKind       = "ClusterGitTrack"
	ClusterGitTrackObjectKind = "ClusterGitTrackObject"
)

// GroupVersion and TypeMeta for v1alpha1.faros.pusher.com
var (
	GroupVersion = schema.GroupVersion{
		Group:   Group,
		Version: Version,
	}

	GitTrackTypeMeta = metav1.TypeMeta{
		APIVersion: GroupVersion.String(),
		Kind:       GitTrackKind,
	}
	GitTrackObjectTypeMeta = metav1.TypeMeta{
		APIVersion: GroupVersion.String(),
		Kind:       GitTrackObjectKind,
	}
	ClusterGitTrackTypeMeta = metav1.TypeMeta{
		APIVersion: GroupVersion.String(),
		Kind:       ClusterGitTrackKind,
	}
	ClusterGitTrackObjectTypeMeta = metav1.TypeMeta{
		APIVersion: GroupVersion.String(),
		Kind:       ClusterGitTrackObjectKind,
	}
)
