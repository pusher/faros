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
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GitTrackObjectSpec defines the desired state of GitTrackObject
type GitTrackObjectSpec struct {
	// Name of the tracked object
	Name string `json:"name"`

	// Kind of the tracked object
	Kind string `json:"kind"`

	// Data representation of the tracked object
	Data []byte `json:"data"`
}

// GitTrackObjectStatus defines the observed state of GitTrackObject
type GitTrackObjectStatus struct {
	// Conditions of this object
	Conditions []GitTrackObjectCondition `json:"conditions"`
}

// GitTrackObjectConditionType is the type of a GitTrackObjectCondition
type GitTrackObjectConditionType string

const (
	// ObjectInSyncType whether the tracked object is in sync or not
	ObjectInSyncType GitTrackObjectConditionType = "ObjectInSync"
)

// GitTrackObjectCondition is a status condition for a GitTrackObject
type GitTrackObjectCondition struct {
	// Type of this condition
	Type GitTrackObjectConditionType `json:"type"`

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

// GitTrackObject is the Schema for the gittrackobjects API
// +k8s:openapi-gen=true
type GitTrackObject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitTrackObjectSpec   `json:"spec,omitempty"`
	Status GitTrackObjectStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitTrackObjectList contains a list of GitTrackObject
type GitTrackObjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitTrackObject `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitTrackObject{}, &GitTrackObjectList{})
}
