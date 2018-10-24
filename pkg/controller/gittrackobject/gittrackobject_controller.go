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
	"os"
	"os/signal"
	"syscall"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gittrackobjectutils "github.com/pusher/faros/pkg/controller/gittrackobject/utils"
	farosflags "github.com/pusher/faros/pkg/flags"
	"github.com/pusher/faros/pkg/utils"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new GitTrackObject Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this faros.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	// Set up informer stop channel
	stop := make(chan struct{})
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		close(stop)
	}()

	// Create a restMapper (used by informer to look up resource kinds)
	restMapper, err := utils.NewRestMapper(mgr.GetConfig())
	if err != nil {
		panic(fmt.Errorf("unable to create rest mapper: %v", err))
	}

	return &ReconcileGitTrackObject{
		Client:      mgr.GetClient(),
		scheme:      mgr.GetScheme(),
		eventStream: make(chan event.GenericEvent),
		cache:       mgr.GetCache(),
		informers:   make(map[string]toolscache.SharedIndexInformer),
		config:      mgr.GetConfig(),
		stop:        stop,
		restMapper:  restMapper,
		recorder:    mgr.GetRecorder("gittrackobject-controller"),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("gittrackobject-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to GitTrackObject
	err = c.Watch(&source.Kind{Type: &farosv1alpha1.GitTrackObject{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to ClusterGitTrackObject
	err = c.Watch(
		&source.Kind{Type: &farosv1alpha1.ClusterGitTrackObject{}},
		&handler.EnqueueRequestForObject{},
		utils.NewOwnerInNamespacePredicate(mgr.GetClient()),
	)
	if err != nil {
		return err
	}

	// Watch for events on the reconciler's eventStream channel
	if gtoReconciler, ok := r.(Reconciler); ok {
		src := &source.Channel{
			Source: gtoReconciler.EventStream(),
		}
		src.InjectStopChannel(gtoReconciler.StopChan())

		restMapper, err := utils.NewRestMapper(mgr.GetConfig())
		if err != nil {
			msg := fmt.Sprintf("unable to create new RESTMapper: %v", err)
			log.Printf(msg)
			return fmt.Errorf(msg)
		}

		// When an event is received, queue the event's owner for reconciliation
		err = c.Watch(src,
			&gittrackobjectutils.EnqueueRequestForOwner{
				NamespacedEnqueueRequestForOwner: &handler.EnqueueRequestForOwner{
					IsController: true,
					OwnerType:    &farosv1alpha1.GitTrackObject{},
				},
				NonNamespacedEnqueueRequestForOwner: &handler.EnqueueRequestForOwner{
					IsController: true,
					OwnerType:    &farosv1alpha1.ClusterGitTrackObject{},
				},
				RestMapper: restMapper,
			},
		)
		if err != nil {
			msg := fmt.Sprintf("unable to watch channel: %v", err)
			log.Printf(msg)
			return fmt.Errorf(msg)
		}
	}

	return nil
}

// Reconciler allows the test suite to mock the required methods
// for setting up the watch streams.
type Reconciler interface {
	EventStream() chan event.GenericEvent
	StopChan() chan struct{}
}

var _ reconcile.Reconciler = &ReconcileGitTrackObject{}

// ReconcileGitTrackObject reconciles a GitTrackObject object
type ReconcileGitTrackObject struct {
	client.Client
	scheme      *runtime.Scheme
	eventStream chan event.GenericEvent
	cache       cache.Cache
	informers   map[string]toolscache.SharedIndexInformer
	config      *rest.Config
	stop        chan struct{}
	restMapper  meta.RESTMapper
	recorder    record.EventRecorder
}

// EventStream returns a stream of generic event to trigger reconciles
func (r *ReconcileGitTrackObject) EventStream() chan event.GenericEvent {
	return r.eventStream
}

// StopChan returns the object stop channel
func (r *ReconcileGitTrackObject) StopChan() chan struct{} {
	return r.stop
}

// Reconcile reads that state of the cluster for a GitTrackObject object and makes changes based on the state read
// and what is in the GitTrackObject.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=faros.pusher.com,resources=gittrackobjects,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileGitTrackObject) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	var instance farosv1alpha1.GitTrackObjectInterface
	sOpts := newStatusOpts()
	mOpts := newMetricOpts()

	// Update the GitTrackObject status when we leave this function
	defer func() {
		err := r.updateStatus(instance, sOpts)
		mErr := r.updateMetrics(instance, mOpts)
		// Print out any errors that may have occurred
		for _, e := range []error{
			err,
			mErr,
			sOpts.inSyncError,
		} {
			if e != nil {
				log.Printf("%v", e)
			}
		}
	}()

	instance, err := r.getInstance(request)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Generate the child from the spec
	child := &unstructured.Unstructured{}
	*child, err = utils.YAMLToUnstructured(instance.GetSpec().Data)
	if err != nil {
		sOpts.inSyncReason = gittrackobjectutils.ErrorUnmarshallingData
		sOpts.inSyncError = fmt.Errorf("unable to unmarshal data: %v", err)
		r.sendEvent(instance, apiv1.EventTypeWarning, "UnmarshalFailed", "Couldn't unmarshal object from JSON/YAML")
		return reconcile.Result{}, sOpts.inSyncError
	}
	if child.GetName() == "" {
		sOpts.inSyncReason = gittrackobjectutils.ErrorGettingChild
		sOpts.inSyncError = fmt.Errorf("unable to get child: name cannot be empty")
		return reconcile.Result{}, nil
	}

	// Make sure to watch the child resource (does nothing if the resource is
	// already being watched
	err = r.watch(*child)
	if err != nil {
		sOpts.inSyncReason = gittrackobjectutils.ErrorWatchingChild
		sOpts.inSyncError = fmt.Errorf("unable to create watch: %v", err)
		return reconcile.Result{}, sOpts.inSyncError
	}

	err = controllerutil.SetControllerReference(instance, child, r.scheme)
	if err != nil {
		sOpts.inSyncReason = gittrackobjectutils.ErrorAddingOwnerReference
		sOpts.inSyncError = fmt.Errorf("unable to add owner reference: %v", err)
		return reconcile.Result{}, sOpts.inSyncError
	}

	// Construct holder for API copy of child
	found := &unstructured.Unstructured{}
	found.SetKind(child.GetKind())
	found.SetAPIVersion(child.GetAPIVersion())

	// Check if the Child already exists
	err = r.Get(context.TODO(), types.NamespacedName{Name: child.GetName(), Namespace: child.GetNamespace()}, found)
	if err != nil && errors.IsNotFound(err) {
		found = child.DeepCopy()
		err = utils.SetLastAppliedAnnotation(found, child)
		if err != nil {
			sOpts.inSyncReason = gittrackobjectutils.ErrorCreatingChild
			sOpts.inSyncError = fmt.Errorf("unable to set annotation: %v", err)
			return reconcile.Result{}, sOpts.inSyncError
		}

		// Not found, so create child
		log.Printf("Creating child %s %s/%s\n", child.GetKind(), child.GetNamespace(), child.GetName())
		r.sendEvent(instance, apiv1.EventTypeNormal, "CreateStarted", "Creating child %s %s/%s", child.GetKind(), child.GetNamespace(), child.GetName())
		err = r.Create(context.TODO(), found)
		if err != nil {
			sOpts.inSyncReason = gittrackobjectutils.ErrorCreatingChild
			sOpts.inSyncError = fmt.Errorf("unable to create child: %v", err)
			r.sendEvent(instance, apiv1.EventTypeWarning, "CreateFailed", "Failed to create child %s %s/%s", child.GetKind(), child.GetNamespace(), child.GetName())
			return reconcile.Result{}, sOpts.inSyncError
		}
		r.sendEvent(instance, apiv1.EventTypeNormal, "CreateSuccessful", "Successfully created child %s %s/%s", child.GetKind(), child.GetNamespace(), child.GetName())
		// Just created the object from the child, no need to check for update
		return reconcile.Result{}, nil
	} else if err != nil {
		sOpts.inSyncReason = gittrackobjectutils.ErrorGettingChild
		sOpts.inSyncError = fmt.Errorf("unable to get child: %v", err)
		return reconcile.Result{}, sOpts.inSyncError
	}

	updateStrategy, err := gittrackobjectutils.GetUpdateStrategy(child)
	if err != nil {
		sOpts.inSyncReason = gittrackobjectutils.ErrorUpdatingChild
		sOpts.inSyncError = fmt.Errorf("unable to get update strategy: %v", err)
		return reconcile.Result{}, sOpts.inSyncError
	}

	if updateStrategy == gittrackobjectutils.NeverUpdateStrategy {
		log.Printf("Update strategy for %s set to never, ignoring", found.GetName())
		// If we aren't updating the resource we should assume the resource is in sync
		mOpts.inSync = true
		return reconcile.Result{}, nil
	}

	// Update the object if the spec differs from the version running
	childUpdated, err := utils.UpdateChildResource(found, child)
	if err != nil {
		sOpts.inSyncReason = gittrackobjectutils.ErrorUpdatingChild
		sOpts.inSyncError = fmt.Errorf("unable to update child: %v", err)
		r.sendEvent(instance, apiv1.EventTypeWarning, "UpdateFailed", "Unable to update child %s %s/%s", child.GetKind(), child.GetNamespace(), child.GetName())
		return reconcile.Result{}, sOpts.inSyncError
	}
	if childUpdated {
		r.sendEvent(instance, apiv1.EventTypeNormal, "UpdateStarted", "Starting update of child %s %s/%s", child.GetKind(), child.GetNamespace(), child.GetName())
		if updateStrategy == gittrackobjectutils.RecreateUpdateStrategy {
			err = r.recreateChild(found, child)
		} else {
			err = r.updateChild(found, child)
		}
		if err != nil {
			sOpts.inSyncReason = gittrackobjectutils.ErrorUpdatingChild
			sOpts.inSyncError = err
			r.sendEvent(instance, apiv1.EventTypeWarning, "UpdateFailed", "Unable to update child %s %s/%s", child.GetKind(), child.GetNamespace(), child.GetName())
			return reconcile.Result{}, sOpts.inSyncError
		}
		r.sendEvent(instance, apiv1.EventTypeNormal, "UpdateSuccessful", "Successfully updated child %s %s/%s", child.GetKind(), child.GetNamespace(), child.GetName())
	}

	// If we got here everything is good so the object must be in-sync
	mOpts.inSync = true
	return reconcile.Result{}, nil
}

// recreateChild first deletes and then creates a child resource for a (Cluster)GitTrackObject
func (r *ReconcileGitTrackObject) recreateChild(found, child *unstructured.Unstructured) error {
	log.Printf("Deleting child %s %s/%s\n", found.GetKind(), found.GetNamespace(), found.GetName())
	err := r.Delete(context.TODO(), found)
	if err != nil {
		return fmt.Errorf("unable to delete child: %v", err)
	}

	found = child.DeepCopy()
	err = utils.SetLastAppliedAnnotation(found, child)
	if err != nil {
		return fmt.Errorf("unable to set annotation: %v", err)
	}

	log.Printf("Creating child %s %s/%s\n", child.GetKind(), child.GetNamespace(), child.GetName())
	err = r.Create(context.TODO(), child)
	if err != nil {
		return fmt.Errorf("unable to create child: %v", err)
	}

	return nil
}

// updateChild updates the given child resource of a (Cluster)GitTrackObject
func (r *ReconcileGitTrackObject) updateChild(found, child *unstructured.Unstructured) error {
	err := utils.SetLastAppliedAnnotation(found, child)
	if err != nil {
		return fmt.Errorf("error setting last applied annotation: %v", err)
	}
	// Update the child resource on the API
	log.Printf("Updating child %s %s/%s\n", child.GetKind(), child.GetNamespace(), child.GetName())
	err = r.Update(context.TODO(), found)
	if err != nil {
		return fmt.Errorf("unable to update child resource: %v", err)
	}
	return nil
}

// getInstance fetches the requested (Cluster)GitTrackObject from the API server
func (r *ReconcileGitTrackObject) getInstance(request reconcile.Request) (farosv1alpha1.GitTrackObjectInterface, error) {
	var instance farosv1alpha1.GitTrackObjectInterface
	if request.Namespace != "" {
		instance = &farosv1alpha1.GitTrackObject{}
	} else {
		instance = &farosv1alpha1.ClusterGitTrackObject{}
	}

	// Fetch the ClusterGitTrackObject instance
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		return nil, err
	}
	return instance, nil
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
