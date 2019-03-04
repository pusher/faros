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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ChildAppliedSuccess represents the condition reason when no error occurs
	// applying the child object
	ChildAppliedSuccess ConditionReason = "ChildAppliedSuccess"

	// ErrorAddingOwnerReference represents the condition reason when the child's
	// Owner reference cannot be set
	ErrorAddingOwnerReference ConditionReason = "ErrorAddingOwnerReference"

	// ErrorUnmarshallingData represents the condition reason when the object's
	// data cannot be unmarshalled
	ErrorUnmarshallingData ConditionReason = "ErrorUnmarshallingData"

	// ErrorCreatingChild represents the condition reason when the controller
	// hits an error trying to create the child
	ErrorCreatingChild ConditionReason = "ErrorCreatingChild"

	// ErrorGettingChild represents the condition reason when the controller
	// hits an error trying to get the child
	ErrorGettingChild ConditionReason = "ErrorGettingChild"

	// ErrorUpdatingChild represents the condition reason when the controller
	// hits an error trying to update the child
	ErrorUpdatingChild ConditionReason = "ErrorUpdatingChild"

	// ErrorWatchingChild represents the condition reason when the controller
	// cannot create an informer for the child's kind
	ErrorWatchingChild ConditionReason = "ErrorWatchingChild"
)

// ConditionReason represents a valid condition reason
type ConditionReason string

// NewGitTrackObjectCondition creates a new GitTrackObject condition.
func NewGitTrackObjectCondition(condType farosv1alpha1.GitTrackObjectConditionType, status v1.ConditionStatus, reason ConditionReason, message string) *farosv1alpha1.GitTrackObjectCondition {
	return &farosv1alpha1.GitTrackObjectCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             string(reason),
		Message:            message,
	}
}

// GetGitTrackObjectCondition returns the condition with the provided type.
func GetGitTrackObjectCondition(status farosv1alpha1.GitTrackObjectStatus, condType farosv1alpha1.GitTrackObjectConditionType) *farosv1alpha1.GitTrackObjectCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

// SetGitTrackObjectCondition updates the GitTrackObject to include the provided condition. If the condition that
// we are about to add already exists and has the same status and reason then we are not going to update.
func SetGitTrackObjectCondition(status *farosv1alpha1.GitTrackObjectStatus, condition farosv1alpha1.GitTrackObjectCondition) {
	currentCond := GetGitTrackObjectCondition(*status, condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason {
		return
	}
	// Do not update lastTransitionTime if the status of the condition doesn't change.
	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}
	newConditions := filterOutCondition(status.Conditions, condition.Type)
	status.Conditions = append(newConditions, condition)
}

// RemoveGitTrackObjectCondition removes the GitTrackObject condition with the provided type.
func RemoveGitTrackObjectCondition(status *farosv1alpha1.GitTrackObjectStatus, condType farosv1alpha1.GitTrackObjectConditionType) {
	status.Conditions = filterOutCondition(status.Conditions, condType)
}

// filterOutCondition returns a new slice of GitTrackObject conditions without conditions with the provided type.
func filterOutCondition(conditions []farosv1alpha1.GitTrackObjectCondition, condType farosv1alpha1.GitTrackObjectConditionType) []farosv1alpha1.GitTrackObjectCondition {
	var newConditions []farosv1alpha1.GitTrackObjectCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}
