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
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
)

// handlerResult is the single return object from handleGitTrackObject
// It contains all information required to update the status and metrics of
// the (Cluster)GitTrackObject passed to it
type handlerResult struct {
	inSyncError  error
	inSyncReason string
}

// handleGitTrackObject handles the management of the child of the GitTrackObjectInterface
// and returns a handlerResult which contains information for updating the
// (Cluster)GitTrackObject's status and metrics
//
// It reads the child object from the instance and udpates the API if the object
// is out of sync
func (r *ReconcileGitTrackObject) handleGitTrackObject(gto farosv1alpha1.GitTrackObjectInterface) handlerResult {
	return handlerResult{}
}
