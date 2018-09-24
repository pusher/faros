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
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GitTrackSpec defines the desired state of GitTrack
type GitTrackSpec struct {
	// Reference contains the git reference this GitTrack tracks
	Reference string `json:"reference"`

	// Repository is the git repository URI to clone from
	Repository string `json:"repository"`

	// +kubebuilder:validation:Pattern=^[a-zA-Z0-9/\-.]*$
	// SubPath is the subpath within the repository underneath which files are considered
	SubPath string `json:"subPath,omitempty"`

	// DeployKey holds a reference to an SSH key needed to access the repository
	DeployKey GitTrackDeployKey `json:"deployKey,omitempty"`
}

// GitTrackDeployKey holds a reference to an SSH key needed to access the repository
type GitTrackDeployKey struct {
	// SecretName is the name of the Secret object containins the key
	SecretName string `json:"secretName"`

	// Key is the key within the Secret object that contains the deploy key
	Key string `json:"key"`
}

// GitTrackStatus defines the observed state of GitTrack
type GitTrackStatus struct {
	// ObjectsDiscovered is the number of k8s objects found in the repository path
	ObjectsDiscovered int64 `json:"objectsDiscovered"`

	// ObjectsApplied is the number of k8s objects for which a GitTrackObjects was created
	ObjectsApplied int64 `json:"objectsApplied"`

	// ObjectsIgnored is the number of k8s objects found in the repository path for which no GitTrackObject was created
	ObjectsIgnored int64 `json:"ObjectsIgnored"`

	// ObjectsInSync is the number of GitTrackObjects that were successfully applied to the cluster
	ObjectsInSync int64 `json:"objectsInSync"`

	// Conditions are the conditions on this GitTrack
	Conditions []GitTrackCondition `json:"conditions,omitempty"`
}

// GitTrackConditionType is the type of a GitTrackCondition
type GitTrackConditionType string

const (
	// FilesParsedType referes to whether all files parsed successfully
	FilesParsedType GitTrackConditionType = "FilesParsed"

	// FilesFetchedType refers to whether all files where fetched from git
	// successfully
	FilesFetchedType GitTrackConditionType = "FilesFetched"

	// ChildrenUpToDateType referes to whether all children were created/updated
	// successfully
	ChildrenUpToDateType GitTrackConditionType = "ChildrenUpToDate"

	// ChildrenGarbageCollectedType referes to whether all children that were meant to
	// be GC'd have been GC'
	ChildrenGarbageCollectedType GitTrackConditionType = "ChildrenGarbageCollected"
)

// GitTrackCondition is a status condition for a GitTrack
type GitTrackCondition struct {
	// Type of this condition
	Type GitTrackConditionType `json:"type"`

	// Status of this condition
	Status v1.ConditionStatus `json:"status"`

	// LastUpdateTime of this condition
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	// LastTransitionTime of this condition
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason for the current status of this condition
	Reason string `json:"reason,omitempty"`

	// Message associated with this condition
	Message string `json:"message,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitTrack is the Schema for the gittracks API
// +k8s:openapi-gen=true
type GitTrack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitTrackSpec   `json:"spec,omitempty"`
	Status GitTrackStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitTrackList contains a list of GitTrack
type GitTrackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitTrack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitTrack{}, &GitTrackList{})
}
