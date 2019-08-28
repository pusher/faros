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
	"time"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gittrackutils "github.com/pusher/faros/pkg/controller/gittrack/utils"
)

// handlerResult is the single return object from handleGitTrack
// It contains all information required to update the status and metrics of
// the (Cluster)GitTrack passed to it
type handlerResult struct {
	applied        int64
	discovered     int64
	ignored        int64
	inSync         int64
	parseError     error
	parseReason    gittrackutils.ConditionReason
	gitError       error
	gitReason      gittrackutils.ConditionReason
	gcError        error
	gcReason       gittrackutils.ConditionReason
	upToDateError  error
	upToDateReason gittrackutils.ConditionReason
	ignoredFiles   map[string]string
	timeToDeploy   []time.Duration
}

func (r *ReconcileGitTrack) handleGitTrack(gt farosv1alpha1.GitTrackInterface) handlerResult {
	return handlerResult{}
}
