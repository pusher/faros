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

package gittrackobject

import (
	"context"
	"fmt"
	"log"
	"reflect"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gittrackobjectutils "github.com/pusher/faros/pkg/controller/gittrackobject/utils"
	v1 "k8s.io/api/core/v1"
)

type statusOpts struct {
	inSyncError  error
	inSyncReason gittrackobjectutils.ConditionReason
}

func (s *statusOpts) isEmpty() bool {
	empty := &statusOpts{}
	return *s == *empty
}

// updateGitTrackObjectStatus updates the GitTrackObject's status field if
// any condition has changed.
func updateGitTrackObjectStatus(gto farosv1alpha1.GitTrackObjectInterface, opts *statusOpts) bool {
	status := gto.GetStatus()
	setCondition(&status, farosv1alpha1.ObjectInSyncType, opts.inSyncError, opts.inSyncReason)

	if !reflect.DeepEqual(gto.GetStatus(), status) {
		gto.SetStatus(status)
		return true
	}
	return false
}

func setCondition(status *farosv1alpha1.GitTrackObjectStatus, condType farosv1alpha1.GitTrackObjectConditionType, condErr error, reason gittrackobjectutils.ConditionReason) {
	if condErr != nil {
		// Error for condition , set condition appropriately
		cond := gittrackobjectutils.NewGitTrackObjectCondition(
			condType,
			v1.ConditionFalse,
			reason,
			condErr.Error(),
		)
		gittrackobjectutils.SetGitTrackObjectCondition(status, *cond)
		return
	}

	// No error for condition, set condition appropriately
	cond := gittrackobjectutils.NewGitTrackObjectCondition(
		condType,
		v1.ConditionTrue,
		reason,
		"",
	)
	gittrackobjectutils.SetGitTrackObjectCondition(status, *cond)
}

// updateStatus calculates a new status for the GitTrackObject and then updates
// the resource on the API if the status differs from before.
func (r *ReconcileGitTrackObject) updateStatus(original farosv1alpha1.GitTrackObjectInterface, opts *statusOpts) error {
	// Default inSyncReason if opts are empty
	if opts.isEmpty() {
		opts.inSyncReason = gittrackobjectutils.ChildAppliedSuccess
	}
	gto := original.DeepCopyInterface()
	gtoUpdated := updateGitTrackObjectStatus(gto, opts)
	if gtoUpdated {
		log.Printf("Updating %s/%s status", gto.GetNamespace(), gto.GetName())
		err := r.Update(context.TODO(), gto)
		if err != nil {
			return fmt.Errorf("unable to update status: %v", err)
		}
	}

	return nil
}
