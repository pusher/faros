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
	"fmt"
	"strings"
	"time"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gittrackutils "github.com/pusher/faros/pkg/controller/gittrack/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

// asStatusOpts converts the handlerResult into a statusOpts to be passed to updateStatus
func (h handlerResult) asStatusOpts() *statusOpts {
	return &statusOpts{
		applied:        h.applied,
		discovered:     h.discovered,
		ignored:        h.ignored,
		inSync:         h.inSync,
		parseError:     h.parseError,
		parseReason:    h.parseReason,
		gitError:       h.gitError,
		gitReason:      h.gitReason,
		gcError:        h.gcError,
		gcReason:       h.gcReason,
		upToDateError:  h.upToDateError,
		upToDateReason: h.upToDateReason,
		ignoredFiles:   h.ignoredFiles,
	}
}

// asMetricOpts converts handlerResult to a metricsOpts to be passed to updateMetrics
func (h handlerResult) asMetricOpts(repository string) *metricsOpts {
	return &metricsOpts{
		status:       h.asStatusOpts(),
		timeToDeploy: h.timeToDeploy,
		repository:   repository,
	}
}

func (r *ReconcileGitTrack) handleGitTrack(gt farosv1alpha1.GitTrackInterface) handlerResult {
	var result handlerResult

	// Get a map of the files that are in the Spec
	files, err := r.getFiles(gt)
	if err != nil {
		result.gitError = err
		result.gitReason = gittrackutils.ErrorFetchingFiles
		return result
	}
	// Git successful, set condition
	result.gitReason = gittrackutils.GitFetchSuccess
	r.recorder.Eventf(gt, corev1.EventTypeNormal, "CheckoutSuccessful", "Successfully checked out '%s' at '%s'", gt.GetSpec().Repository, gt.GetSpec().Reference)

	// Attempt to parse k8s objects from files
	objects, fileErrors := objectsFrom(files)
	result.ignoredFiles = fileErrors
	result.ignored += int64(len(fileErrors))
	if len(fileErrors) > 0 {
		var errs []string
		for file, reason := range fileErrors {
			errs = append(errs, fmt.Sprintf("%s: %s", file, reason))
		}
		result.parseError = fmt.Errorf(strings.Join(errs, ",\n"))
		result.parseReason = gittrackutils.ErrorParsingFiles
	} else {
		result.parseReason = gittrackutils.FileParseSuccess
	}

	// Update status with the number of objects discovered
	result.discovered = int64(len(objects))

	// Get a list of the GitTrackObjects that currently exist, by name
	objectsByName, err := r.listObjectsByName(gt)
	if err != nil {
		return result
	}
	// Process the objects and feed back the results
	resultsChan := make(chan objectResult, len(objects))
	for _, obj := range objects {
		go func(obj *unstructured.Unstructured) {
			resultsChan <- r.handleObject(obj, gt)
		}(obj)
	}

	handlerErrors := []string{}
	// Iterate through results and update status accordingly
	for range objects {
		res := <-resultsChan
		if res.Ignored {
			result.ignoredFiles[res.NamespacedName] = res.Reason
			result.ignored++
		} else {
			result.applied++
		}
		result.timeToDeploy = append(result.timeToDeploy, res.TimeToDeploy)
		if res.InSync {
			result.inSync++
		}
		delete(objectsByName, res.NamespacedName)
		if res.Error != nil {
			handlerErrors = append(handlerErrors, res.Error.Error())
		}
	}

	// If there were errors updating the child objects, set the ChildrenUpToDate
	// condition appropriately
	if len(handlerErrors) > 0 {
		result.upToDateError = fmt.Errorf(strings.Join(handlerErrors, ",\n"))
		result.upToDateReason = gittrackutils.ErrorUpdatingChildren
	} else {
		result.upToDateReason = gittrackutils.ChildrenUpdateSuccess
	}

	// Cleanup potentially leftover resources
	if err = r.deleteResources(objectsByName); err != nil {
		result.gcError = err
		result.gcReason = gittrackutils.ErrorDeletingChildren
		r.recorder.Eventf(gt, corev1.EventTypeWarning, "CleanupFailed", "Failed to clean-up leftover resources")
		return result
	}
	result.gcReason = gittrackutils.GCSuccess

	return result
}
