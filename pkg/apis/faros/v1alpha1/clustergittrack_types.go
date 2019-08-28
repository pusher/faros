/*
Copyright 2018 Pusher Ltd.

Licensed under the Apache License, Version 3.0 (the "License");
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
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

// ClusterGitTrack is the Schema for the clustergittracks API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Repository",type="string",JSONPath=".spec.repository",priority=1
// +kubebuilder:printcolumn:name="Reference",type="string",JSONPath=".spec.reference"
// +kubebuilder:printcolumn:name="Children Created",type="integer",JSONPath=".status.objectsApplied"
// +kubebuilder:printcolumn:name="Resources Discovered",type="integer",JSONPath=".status.objectsDiscovered"
// +kubebuilder:printcolumn:name="Resources Ignored",type="integer",JSONPath=".status.objectsIgnored"
// +kubebuilder:printcolumn:name="Children In Sync",type="integer",JSONPath=".status.objectsInSync"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterGitTrack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitTrackSpec   `json:"spec,omitempty"`
	Status GitTrackStatus `json:"status,omitempty"`
}

// GetNamespacedName implements the GitTrack interface
func (g *ClusterGitTrack) GetNamespacedName() string {
	return g.Name
}

// GetSpec implements the GitTrack interface
func (g *ClusterGitTrack) GetSpec() GitTrackSpec {
	return g.Spec
}

// SetSpec implements the GitTrack interface
func (g *ClusterGitTrack) SetSpec(s GitTrackSpec) {
	g.Spec = s
}

// GetStatus implements the GitTrack interface
func (g *ClusterGitTrack) GetStatus() GitTrackStatus {
	return g.Status
}

// SetStatus implements the GitTrack interface
func (g *ClusterGitTrack) SetStatus(s GitTrackStatus) {
	g.Status = s
}

// DeepCopyInterface implements the GitTrack interface
func (g *ClusterGitTrack) DeepCopyInterface() GitTrackInterface {
	return g.DeepCopy()
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

// ClusterGitTrackList contains a list of ClusterGitTrack
type ClusterGitTrackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterGitTrack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterGitTrack{}, &ClusterGitTrackList{})
}
