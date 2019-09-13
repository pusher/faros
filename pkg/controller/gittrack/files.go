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
	"strings"
	"time"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	farosflags "github.com/pusher/faros/pkg/flags"
	utils "github.com/pusher/faros/pkg/utils"
	gitstore "github.com/pusher/git-store"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// checkoutRepo checks out the repository at reference and returns a pointer to said repository
func (r *ReconcileGitTrack) checkoutRepo(url string, ref string, gitCreds *gitCredentials) (*gitstore.Repo, error) {
	r.log.V(1).Info("Getting repository", "url", url)
	repoRef, err := createRepoRefFromCreds(url, gitCreds)
	if err != nil {
		return &gitstore.Repo{}, err
	}
	rc, done, err := r.store.GetAsync(repoRef)
	if err != nil {
		return &gitstore.Repo{}, fmt.Errorf("failed to get repository '%s': %v'", url, err)
	}
	repo := &gitstore.Repo{}

	select {
	case <-done:
		if rc.Error != nil {
			return repo, fmt.Errorf("failed to get repository '%s': %v'", url, rc.Error)
		}
		repo = rc.Repo
	case <-time.After(farosflags.FetchTimeout):
		return repo, fmt.Errorf("timed out getting repository '%s'", url)
	}

	r.log.V(1).Info("Checking out reference", "reference", ref)
	ctx, cancel := context.WithTimeout(context.Background(), farosflags.FetchTimeout)
	defer cancel()
	err = repo.CheckoutContext(ctx, ref)
	if err != nil {
		return &gitstore.Repo{}, fmt.Errorf("failed to checkout '%s': %v", ref, err)
	}

	lastUpdated, err := repo.LastUpdated()
	if err != nil {
		return &gitstore.Repo{}, fmt.Errorf("failed to get last updated timestamp: %v", err)
	}

	r.mutex.Lock()
	r.lastUpdateTimes[url] = lastUpdated
	r.mutex.Unlock()

	return repo, nil
}

// getFiles checks out the Spec.Repository at Spec.Reference and returns a map of filename to
// gitstore.File pointers
func (r *ReconcileGitTrack) getFiles(gt farosv1alpha1.GitTrackInterface) (map[string]*gitstore.File, error) {
	r.recorder.Eventf(gt, corev1.EventTypeNormal, "CheckoutStarted", "Checking out '%s' at '%s'", gt.GetSpec().Repository, gt.GetSpec().Reference)
	gitCreds, err := r.fetchGitCredentials(gt)
	if err != nil {
		r.recorder.Eventf(gt, corev1.EventTypeWarning, "CheckoutFailed", "Failed to checkout '%s' at '%s'", gt.GetSpec().Repository, gt.GetSpec().Reference)
		return nil, fmt.Errorf("unable to retrieve git credentials from secret: %v", err)
	}

	repo, err := r.checkoutRepo(gt.GetSpec().Repository, gt.GetSpec().Reference, gitCreds)
	if err != nil {
		r.recorder.Eventf(gt, corev1.EventTypeWarning, "CheckoutFailed", "Failed to checkout '%s' at '%s'", gt.GetSpec().Repository, gt.GetSpec().Reference)
		return nil, err
	}

	subPath := gt.GetSpec().SubPath
	if !strings.HasSuffix(subPath, "/") {
		subPath += "/"
	}

	r.log.V(1).Info("Loading files from subpath", "subpath", subPath)
	globbedSubPath := strings.TrimPrefix(subPath, "/") + "{**/*,*}.{yaml,yml,json}"
	files, err := repo.GetAllFiles(globbedSubPath, true)
	if err != nil {
		r.recorder.Eventf(gt, corev1.EventTypeWarning, "CheckoutFailed", "Failed to get files for SubPath '%s'", gt.GetSpec().SubPath)
		return nil, fmt.Errorf("failed to get all files for subpath '%s': %v", gt.GetSpec().SubPath, err)
	} else if len(files) == 0 {
		r.recorder.Eventf(gt, corev1.EventTypeWarning, "CheckoutFailed", "No files for SubPath '%s'", gt.GetSpec().SubPath)
		return nil, fmt.Errorf("no files for subpath '%s'", gt.GetSpec().SubPath)
	}

	r.log.V(1).Info("Loaded files from repository", "file count", len(files))
	return files, nil
}

// objectsFrom iterates through all the files given and attempts to create Unstructured objects
func objectsFrom(files map[string]*gitstore.File) ([]*unstructured.Unstructured, map[string]string) {
	objects := []*unstructured.Unstructured{}
	fileErrors := make(map[string]string)
	for path, file := range files {
		// TODO (@JoelSpeed): What happens if there are multiple resources in one file,
		// but one of them is invalid? Can we still get the rest?
		us, err := utils.YAMLToUnstructuredSlice([]byte(file.Contents()))
		if err != nil {
			fileErrors[path] = fmt.Sprintf("unable to parse '%s': %v\n", path, err)
			continue
		}
		objects = append(objects, us...)
	}
	return objects, fileErrors
}
