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
	farosflags "github.com/pusher/faros/pkg/flags"
	"github.com/pusher/faros/pkg/utils"
	farosclient "github.com/pusher/faros/pkg/utils/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// handlerResult is the single return object from handleGitTrackObject
// It contains all information required to update the status and metrics of
// the (Cluster)GitTrackObject passed to it
type handlerResult struct {
	inSyncError  error
	inSyncReason gittrackobjectutils.ConditionReason
}

// handleGitTrackObject handles the management of the child of the GitTrackObjectInterface
// and returns a handlerResult which contains information for updating the
// (Cluster)GitTrackObject's status and metrics
//
// It reads the child object from the instance and udpates the API if the object
// is out of sync
func (r *ReconcileGitTrackObject) handleGitTrackObject(gto farosv1alpha1.GitTrackObjectInterface) handlerResult {
	// Generate the child from the spec
	child, reason, err := r.getChildFromGitTrackObject(gto)
	if err != nil {
		return handlerResult{
			inSyncReason: reason,
			inSyncError:  fmt.Errorf("error reading child: %v", err),
		}
	}

	// Make sure to watch the child resource (does nothing if the resource is
	// already being watched)
	err = r.watch(*child)
	if err != nil {
		return handlerResult{
			inSyncReason: gittrackobjectutils.ErrorWatchingChild,
			inSyncError:  fmt.Errorf("unable to create watch: %v", err),
		}
	}

	// Add an owner reference to the child object
	err = controllerutil.SetControllerReference(gto, child, r.scheme)
	if err != nil {
		return handlerResult{
			inSyncReason: gittrackobjectutils.ErrorAddingOwnerReference,
			inSyncError:  fmt.Errorf("unable to add owner reference: %v", err),
		}
	}

	// Construct holder for API copy of child
	found := &unstructured.Unstructured{}
	found.SetKind(child.GetKind())
	found.SetAPIVersion(child.GetAPIVersion())

	err = r.Get(context.TODO(), types.NamespacedName{Name: child.GetName(), Namespace: child.GetNamespace()}, found)
	if err != nil && errors.IsNotFound(err) {
		reason, err = r.handleCreate(gto, child)
		if err != nil {
			return handlerResult{
				inSyncReason: reason,
				inSyncError:  fmt.Errorf("error creating child: %v", err),
			}
		}

		// Successfully created child
		return handlerResult{}
	} else if err != nil {
		return handlerResult{
			inSyncReason: gittrackobjectutils.ErrorGettingChild,
			inSyncError:  fmt.Errorf("unable to get child: %v", err),
		}
	}

	reason, err = r.handleUpdate(gto, found, child)
	if err != nil {
		return handlerResult{
			inSyncReason: reason,
			inSyncError:  fmt.Errorf("error updating child: %v", err),
		}
	}

	return handlerResult{}
}

// getChildFromGitTrackObject reads the Data from a GitTrackObjectSpec and
// converts it into and unstructured.unstructured runtime object
func (r *ReconcileGitTrackObject) getChildFromGitTrackObject(gto farosv1alpha1.GitTrackObjectInterface) (*unstructured.Unstructured, gittrackobjectutils.ConditionReason, error) {
	child, err := utils.YAMLToUnstructured(gto.GetSpec().Data)
	if err != nil {
		r.sendEvent(gto, corev1.EventTypeWarning, "UnmarshalFailed", "Couldn't unmarshal object from JSON/YAML")
		return nil, gittrackobjectutils.ErrorUnmarshallingData, fmt.Errorf("unable to unmarshal data: %v", err)
	}

	// If the child has no name then we can't use it
	if child.GetName() == "" {
		return nil, gittrackobjectutils.ErrorGettingChild, fmt.Errorf("unable to get child: name cannot be empty")
	}

	return &child, "", nil
}

// handleCreate takes an unstructured object sends it to the API to create it
func (r *ReconcileGitTrackObject) handleCreate(gto farosv1alpha1.GitTrackObjectInterface, child *unstructured.Unstructured) (gittrackobjectutils.ConditionReason, error) {
	// Log and send event that we are attempting to create the child resource
	log.Printf("Creating child %s %s/%s\n", child.GetKind(), child.GetNamespace(), child.GetName())
	r.sendEvent(gto, corev1.EventTypeNormal, "CreateStarted", "Creating child %s %s/%s", child.GetKind(), child.GetNamespace(), child.GetName())

	err := r.applier.Apply(context.TODO(), &farosclient.ApplyOptions{}, child)
	if err != nil {
		r.sendEvent(gto, corev1.EventTypeWarning, "CreateFailed", "Failed to create child %s %s/%s", child.GetKind(), child.GetNamespace(), child.GetName())
		return gittrackobjectutils.ErrorCreatingChild, fmt.Errorf("unable to create child: %v", err)
	}

	// Successfully created the child object
	r.sendEvent(gto, corev1.EventTypeNormal, "CreateSuccessful", "Successfully created child %s %s/%s", child.GetKind(), child.GetNamespace(), child.GetName())
	return "", nil
}

func (r *ReconcileGitTrackObject) handleUpdate(gto farosv1alpha1.GitTrackObjectInterface, found, child *unstructured.Unstructured) (gittrackobjectutils.ConditionReason, error) {
	updateStrategy, err := gittrackobjectutils.GetUpdateStrategy(child)
	if err != nil {
		return gittrackobjectutils.ErrorUpdatingChild, fmt.Errorf("unable to get update strategy: %v", err)
	}

	switch updateStrategy {
	case gittrackobjectutils.RecreateUpdateStrategy:
		return r.handleRecreateUpdateStrategy(gto, found, child)
	case gittrackobjectutils.NeverUpdateStrategy:
		return r.handleNeverUpdateStrategy(gto, found)
	default:
		return r.handleDefaultUpdateStrategy(gto, found, child)
	}
}

// handleDefaultUpdateStrategy compares the existing and desired state of the
// child resource and updates the object in-place if required
func (r *ReconcileGitTrackObject) handleDefaultUpdateStrategy(gto farosv1alpha1.GitTrackObjectInterface, found, child *unstructured.Unstructured) (gittrackobjectutils.ConditionReason, error) {
	childUpdated, err := r.updateChild(found, child)
	if err != nil {
		r.sendEvent(gto, corev1.EventTypeWarning, "UpdateFailed", "Unable to update child %s %s/%s", child.GetKind(), child.GetNamespace(), child.GetName())
		return gittrackobjectutils.ErrorUpdatingChild, fmt.Errorf("unable to update child: %v", err)
	}
	if !childUpdated {
		return "", nil
	}

	// Update was successful
	r.sendEvent(gto, corev1.EventTypeNormal, "UpdateSuccessful", "Successfully updated child %s %s/%s", child.GetKind(), child.GetNamespace(), child.GetName())
	return "", nil
}

// handleNeverUpdateStrategy compares the existing object to the existing object
// with the correct owner references applied and updates if necessary
func (r *ReconcileGitTrackObject) handleNeverUpdateStrategy(gto farosv1alpha1.GitTrackObjectInterface, found *unstructured.Unstructured) (gittrackobjectutils.ConditionReason, error) {
	child := found.DeepCopy()
	err := controllerutil.SetControllerReference(gto, child, r.scheme)
	if err != nil {
		return gittrackobjectutils.ErrorAddingOwnerReference, fmt.Errorf("unable to add owner reference: %v", err)
	}
	return r.handleDefaultUpdateStrategy(gto, found, child)
}

// handleRecreateUpdateStrategy compares the existing and desired state of the
// resources and then deletes and recreates the child object if an update is
// required
func (r *ReconcileGitTrackObject) handleRecreateUpdateStrategy(gto farosv1alpha1.GitTrackObjectInterface, found, child *unstructured.Unstructured) (gittrackobjectutils.ConditionReason, error) {
	childUpdated, err := r.recreateChild(found, child)
	if err != nil {
		r.sendEvent(gto, corev1.EventTypeWarning, "UpdateFailed", "Unable to update child %s %s/%s", child.GetKind(), child.GetNamespace(), child.GetName())
		return gittrackobjectutils.ErrorUpdatingChild, fmt.Errorf("unable to update child: %v", err)
	}
	if !childUpdated {
		return "", nil
	}

	// Update was successful
	r.sendEvent(gto, corev1.EventTypeNormal, "UpdateSuccessful", "Successfully updated child %s %s/%s", child.GetKind(), child.GetNamespace(), child.GetName())
	return "", nil
}

// recreateChild first deletes and then creates a child resource for a (Cluster)GitTrackObject
func (r *ReconcileGitTrackObject) recreateChild(found, child *unstructured.Unstructured) (bool, error) {
	// Recreating the child does not make sense with dry run (dry run delete does
	// not mean we can dry run create) and so do not attempt dry run here.
	return r.applyChild(found, child, true)
}

// updateChild updates the given child resource of a (Cluster)GitTrackObject
func (r *ReconcileGitTrackObject) updateChild(found, child *unstructured.Unstructured) (bool, error) {
	// HasSupport returns an error if dry run not supported
	if err := r.dryRunVerifier.HasSupport(child.GroupVersionKind()); err == nil {
		return r.applyChildWithDryRun(found, child, false)
	}
	// Dry run not supported so apply without DryRun
	return r.applyChild(found, child, false)
}

// applyChildWithDryRun first applies the child with DryRun and then updates the resource if there is change to persist
func (r *ReconcileGitTrackObject) applyChildWithDryRun(found, child *unstructured.Unstructured, force bool) (bool, error) {
	// Take a copy of the original child so that if the dry run shows a diff,
	// we Apply the original state of the child object
	originalChild := child.DeepCopy()

	dryRunTrue := true
	err := r.applier.Apply(context.TODO(), &farosclient.ApplyOptions{ForceDeletion: &force, ServerDryRun: &dryRunTrue}, child)
	if err != nil {
		return false, fmt.Errorf("unable to update child resource: %v", err)
	}

	// Not updated if the child now equals the server version
	if reflect.DeepEqual(child, found) {
		return false, nil
	}

	// The DryRun showed a change is required so now update without DryRun
	err = r.applier.Apply(context.TODO(), &farosclient.ApplyOptions{ForceDeletion: &force}, originalChild)
	if err != nil {
		return false, fmt.Errorf("unable to update child resource: %v", err)
	}
	return true, nil
}

// applyChild uses the applier to update the child
func (r *ReconcileGitTrackObject) applyChild(found, child *unstructured.Unstructured, force bool) (bool, error) {
	originalResourceVersion := found.GetResourceVersion()
	err := r.applier.Apply(context.TODO(), &farosclient.ApplyOptions{ForceDeletion: &force}, child)
	if err != nil {
		return false, fmt.Errorf("unable to update child resource: %v", err)
	}

	// Not updated if the resource version hasn't changed
	if originalResourceVersion == child.GetResourceVersion() {
		return false, nil
	}
	return true, nil
}

// sendEvent wraps event recording to make sure the namespace is set correctly
// on all events
func (r *ReconcileGitTrackObject) sendEvent(gto farosv1alpha1.GitTrackObjectInterface, eventType, reason, messageFmt string, args ...interface{}) {
	instance := gto.DeepCopyInterface()
	if instance.GetNamespace() == "" {
		instance.SetNamespace(farosflags.Namespace)
	}

	r.recorder.Eventf(instance, eventType, reason, messageFmt, args...)
}
