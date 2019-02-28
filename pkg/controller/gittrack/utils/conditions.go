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
	// StatusUnknown is the default ConditionReason for all conditions
	StatusUnknown ConditionReason = "StatusUnknown"

	// ErrorFetchingFiles represents the condition reason when an error occurs
	// fetching files from the repository
	ErrorFetchingFiles ConditionReason = "ErrorFetchingFiles"

	// GitFetchSuccess represents the condition reason when no error occurs
	// fecthing files from the repository
	GitFetchSuccess ConditionReason = "GitFetchSuccess"

	// ErrorParsingFiles represents the condition reason when an error occurs
	// parsing files from the repository
	ErrorParsingFiles ConditionReason = "ErrorParsingFiles"

	// FileParseSuccess represents the condition reason when no error occurs
	// parsing files from the repository
	FileParseSuccess ConditionReason = "FileParseSuccess"

	// ErrorUpdatingChildren represents the condition reason when an error occurs
	// updating the child objects
	ErrorUpdatingChildren ConditionReason = "ErrorUpdatingChildren"

	// ChildrenUpdateSuccess represents the condition reason when no error occurs
	// updating the child objects
	ChildrenUpdateSuccess ConditionReason = "ChildUpdateSuccess"

	// ErrorDeletingChildren represents the condition reason when an error occurs
	// removing orphaned children
	ErrorDeletingChildren ConditionReason = "ErrorDeletingChildren"

	// GCSuccess represents the condition reason when no error occurs
	// removing orphaned children
	GCSuccess ConditionReason = "GCSuccess"
)

// ConditionReason represents a valid condition reason
type ConditionReason string

// NewGitTrackCondition creates a new GitTrack condition.
func NewGitTrackCondition(condType farosv1alpha1.GitTrackConditionType, status v1.ConditionStatus, reason ConditionReason, message string) *farosv1alpha1.GitTrackCondition {
	return &farosv1alpha1.GitTrackCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             string(reason),
		Message:            message,
	}
}

// GetGitTrackCondition returns the condition with the provided type.
func GetGitTrackCondition(status farosv1alpha1.GitTrackStatus, condType farosv1alpha1.GitTrackConditionType) *farosv1alpha1.GitTrackCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

// SetGitTrackCondition updates the GitTrack to include the provided condition. If the condition that
// we are about to add already exists and has the same status and reason then we are not going to update.
func SetGitTrackCondition(status *farosv1alpha1.GitTrackStatus, condition farosv1alpha1.GitTrackCondition) {
	currentCond := GetGitTrackCondition(*status, condition.Type)
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

// RemoveGitTrackCondition removes the GitTrack condition with the provided type.
func RemoveGitTrackCondition(status *farosv1alpha1.GitTrackStatus, condType farosv1alpha1.GitTrackConditionType) {
	status.Conditions = filterOutCondition(status.Conditions, condType)
}

// filterOutCondition returns a new slice of GitTrack conditions without conditions with the provided type.
func filterOutCondition(conditions []farosv1alpha1.GitTrackCondition, condType farosv1alpha1.GitTrackConditionType) []farosv1alpha1.GitTrackCondition {
	var newConditions []farosv1alpha1.GitTrackCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}
