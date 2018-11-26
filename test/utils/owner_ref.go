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

package utils

import (
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetGitTrackObjectOwnerRef constructs an owner reference for the GTO given
func GetGitTrackObjectOwnerRef(gto *farosv1alpha1.GitTrackObject) metav1.OwnerReference {
	t := true
	return metav1.OwnerReference{
		APIVersion:         "faros.pusher.com/v1alpha1",
		Kind:               "GitTrackObject",
		Name:               gto.GetName(),
		UID:                gto.GetUID(),
		Controller:         &t,
		BlockOwnerDeletion: &t,
	}
}

// GetClusterGitTrackObjectOwnerRef constructs an owner reference for the GTO given
func GetClusterGitTrackObjectOwnerRef(gto *farosv1alpha1.ClusterGitTrackObject) metav1.OwnerReference {
	t := true
	return metav1.OwnerReference{
		APIVersion:         "faros.pusher.com/v1alpha1",
		Kind:               "ClusterGitTrackObject",
		Name:               gto.GetName(),
		UID:                gto.GetUID(),
		Controller:         &t,
		BlockOwnerDeletion: &t,
	}
}
