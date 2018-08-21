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
	"github.com/pusher/faros/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
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
		informers:   make(map[string]cache.SharedIndexInformer),
		config:      mgr.GetConfig(),
		stop:        stop,
		restMapper:  restMapper,
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

	// Watch for events on the reconciler's eventStream channel
	if gtoReconciler, ok := r.(Reconciler); ok {
		src := &source.Channel{
			Source: gtoReconciler.EventStream(),
		}
		src.InjectStopChannel(gtoReconciler.StopChan())
		// When an event is received, queue the event's owner for reconciliation
		err = c.Watch(src,
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &farosv1alpha1.GitTrackObject{},
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
	informers   map[string]cache.SharedIndexInformer
	config      *rest.Config
	stop        chan struct{}
	restMapper  meta.RESTMapper
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
	instance := &farosv1alpha1.GitTrackObject{}
	opts := newStatusOpts()

	// Update the GitTrackObject status when we leave this function
	defer func() {
		err := r.updateStatus(instance, opts)
		// Print out any errors that may have occurred
		for _, e := range []error{
			err,
			opts.inSyncError,
		} {
			if e != nil {
				log.Printf("%v", e)
			}
		}
	}()

	// Fetch the GitTrackObject instance
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.

			// Set instance to nil so we don't update the status
			instance = nil
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Generate the child from the spec
	child := &unstructured.Unstructured{}
	*child, err = utils.YAMLToUnstructured(instance.Spec.Data)
	if err != nil {
		opts.inSyncReason = gittrackobjectutils.ErrorUnmarshallingData
		opts.inSyncError = fmt.Errorf("unable to unmarshal data: %v", err)
		return reconcile.Result{}, opts.inSyncError
	}

	// Make sure to watch the child resource (does nothing if the resource is
	// already being watched
	err = r.watch(*child)
	if err != nil {
		opts.inSyncReason = gittrackobjectutils.ErrorWatchingChild
		opts.inSyncError = fmt.Errorf("unable to create watch: %v", err)
		return reconcile.Result{}, opts.inSyncError
	}

	// Add an owner reference to the child object
	err = controllerutil.SetControllerReference(instance, child, r.scheme)
	if err != nil {
		opts.inSyncReason = gittrackobjectutils.ErrorAddingOwnerReference
		opts.inSyncError = fmt.Errorf("unable to add owner reference: %v", err)
		return reconcile.Result{}, opts.inSyncError
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
			opts.inSyncReason = gittrackobjectutils.ErrorCreatingChild
			opts.inSyncError = fmt.Errorf("unable to set annotation: %v", err)
			return reconcile.Result{}, opts.inSyncError
		}

		// Not found, so create child
		log.Printf("Creating child %s %s/%s\n", child.GetKind(), child.GetNamespace(), child.GetName())
		err = r.Create(context.TODO(), found)
		if err != nil {
			opts.inSyncReason = gittrackobjectutils.ErrorCreatingChild
			opts.inSyncError = fmt.Errorf("unable to create child: %v", err)
			return reconcile.Result{}, opts.inSyncError
		}
		// Just created the object from the child, no need to check for update
		return reconcile.Result{}, nil
	} else if err != nil {
		opts.inSyncReason = gittrackobjectutils.ErrorGettingChild
		opts.inSyncError = fmt.Errorf("unable to get child: %v", err)
		return reconcile.Result{}, opts.inSyncError
	}

	// Update the object if the spec differs from the version running
	childUpdated, err := utils.UpdateChildResource(found, child)
	if err != nil {
		opts.inSyncReason = gittrackobjectutils.ErrorUpdatingChild
		opts.inSyncError = fmt.Errorf("unable to update child: %v", err)
		return reconcile.Result{}, opts.inSyncError
	}
	if childUpdated {
		err := utils.SetLastAppliedAnnotation(found, child)
		if err != nil {
			opts.inSyncReason = gittrackobjectutils.ErrorUpdatingChild
			opts.inSyncError = fmt.Errorf("error setting last applied annotation: %v", err)
			return reconcile.Result{}, opts.inSyncError
		}
		// Update the child resource on the API
		log.Printf("Updating child %s %s/%s\n", child.GetKind(), child.GetNamespace(), child.GetName())
		err = r.Update(context.TODO(), found)
		if err != nil {
			opts.inSyncReason = gittrackobjectutils.ErrorUpdatingChild
			opts.inSyncError = fmt.Errorf("unable to update child resource: %v", err)
			return reconcile.Result{}, opts.inSyncError
		}
	}

	return reconcile.Result{}, nil
}
