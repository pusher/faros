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

package gittrack

import (
	"context"
	"fmt"
	"log"
	"reflect"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gittrackutils "github.com/pusher/faros/pkg/controller/gittrack/utils"
	"k8s.io/api/core/v1"
)

type statusOpts struct {
	applied     int64
	discovered  int64
	ignored     int64
	inSync      int64
	parseError  error
	parseReason gittrackutils.ConditionReason
	gitError    error
	gitReason   gittrackutils.ConditionReason
}

func newStatusOpts() *statusOpts {
	return &statusOpts{
		parseReason: gittrackutils.StatusUnknown,
		gitReason:   gittrackutils.StatusUnknown,
	}
}

func updateGitTrackStatus(gt *farosv1alpha1.GitTrack, opts *statusOpts) (updated bool) {
	if gt == nil {
		return
	}

	status := farosv1alpha1.GitTrackStatus{}

	status.ObjectsApplied = opts.applied
	status.ObjectsDiscovered = opts.discovered
	status.ObjectsIgnored = opts.ignored
	status.ObjectsInSync = opts.inSync
	setCondition(&status, farosv1alpha1.FilesParsedType, opts.parseError, opts.parseReason)
	setCondition(&status, farosv1alpha1.FilesFetchedType, opts.gitError, opts.gitReason)

	if !reflect.DeepEqual(gt.Status, status) {
		gt.Status = status
		updated = true
	}
	return
}

func setCondition(status *farosv1alpha1.GitTrackStatus, condType farosv1alpha1.GitTrackConditionType, condErr error, reason gittrackutils.ConditionReason) {
	if condErr != nil {
		// Error for condition , set condition appropriately
		cond := gittrackutils.NewGitTrackCondition(
			condType,
			v1.ConditionFalse,
			reason,
			condErr.Error(),
		)
		gittrackutils.SetGitTrackCondition(status, *cond)
		return
	}

	// No error for condition, set condition appropriately
	cond := gittrackutils.NewGitTrackCondition(
		condType,
		v1.ConditionTrue,
		reason,
		"",
	)
	gittrackutils.SetGitTrackCondition(status, *cond)
}

// updateStatus calculates a new status for the GitTrack and then updates
// the resource on the API if the status differs from before.
func (r *ReconcileGitTrack) updateStatus(original *farosv1alpha1.GitTrack, opts *statusOpts) error {
	// Update the GitTrack's status
	gt := original.DeepCopy()
	gtUpdated := updateGitTrackStatus(gt, opts)

	// If the status was modified, update the GitTrack on the API
	if gtUpdated {
		log.Printf("Updating GitTrack %s status", gt.Name)
		err := r.Update(context.TODO(), gt)
		if err != nil {
			return fmt.Errorf("unable to update GitTrack: %v", err)
		}
	}
	return nil
}
