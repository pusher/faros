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
	"k8s.io/api/core/v1"
)

type statusOpts struct {
	inSyncError  error
	inSyncReason gittrackobjectutils.ConditionReason
}

func newStatusOpts() *statusOpts {
	return &statusOpts{
		inSyncReason: gittrackobjectutils.ChildAppliedSuccess,
	}
}

// updateGitTrackObjectStatus updates the GitTrackObject's status field if
// any condition has changed.
func updateGitTrackObjectStatus(gto *farosv1alpha1.GitTrackObject, opts *statusOpts) bool {
	if gto == nil {
		return false
	}
	status := gto.GetStatus()

	setCondition(&status, farosv1alpha1.ObjectInSyncType, opts.inSyncError, opts.inSyncReason)

	if !reflect.DeepEqual(gto.Status, status) {
		gto.Status = status
		return true
	}
	return false
}

// updateClusterGitTrackObjectStatus updates the ClsuterGitTrackObject's status
// field if any condition has changed.
func updateClusterGitTrackObjectStatus(gto *farosv1alpha1.ClusterGitTrackObject, opts *statusOpts) bool {
	if gto == nil {
		return false
	}
	status := gto.Status

	setCondition(&status, farosv1alpha1.ObjectInSyncType, opts.inSyncError, opts.inSyncReason)

	if !reflect.DeepEqual(gto.Status, status) {
		gto.Status = status
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
	// Update the GitTrackObject's status
	switch t := original.(type) {
	case *farosv1alpha1.GitTrackObject:
		g := original.(*farosv1alpha1.GitTrackObject).DeepCopy()
		gtoUpdated := updateGitTrackObjectStatus(g, opts)
		// If the status was modified, update the GitTrackObject on the API
		if gtoUpdated {
			log.Printf("Updating GitTrackObject %s status", g.Name)
			err := r.Update(context.TODO(), g)
			if err != nil {
				return fmt.Errorf("unable to update GitTrackObject: %v", err)
			}
		}
	case *farosv1alpha1.ClusterGitTrackObject:
		g := original.(*farosv1alpha1.ClusterGitTrackObject).DeepCopy()
		gtoUpdated := updateClusterGitTrackObjectStatus(g, opts)
		// If the status was modified, update the GitTrackObject on the API
		if gtoUpdated {
			log.Printf("Updating ClusterGitTrackObject %s status", g.Name)
			err := r.Update(context.TODO(), g)
			if err != nil {
				return fmt.Errorf("unable to update ClusterGitTrackObject: %v", err)
			}
		}
	case nil:
		// If the object is nil, don't do anything
		return nil
	default:
		return fmt.Errorf("unknown type: %s", t)
	}
	return nil
}
