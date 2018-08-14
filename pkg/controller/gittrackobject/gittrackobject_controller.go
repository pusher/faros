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
	"reflect"
	"syscall"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	"github.com/pusher/faros/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

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

	return &ReconcileGitTrackObject{
		Client:      mgr.GetClient(),
		scheme:      mgr.GetScheme(),
		eventStream: make(chan event.GenericEvent),
		stop:        stop,
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

	if gtoReconciler, ok := r.(Reconciler); ok {
		src := &source.Channel{
			Source: gtoReconciler.EventStream(),
		}
		src.InjectStopChannel(gtoReconciler.StopChan())
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
	stop        chan struct{}
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
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=faros.pusher.com,resources=gittrackobjects,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileGitTrackObject) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the GitTrackObject instance
	instance := &farosv1alpha1.GitTrackObject{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	child := &unstructured.Unstructured{}
	*child, err = utils.YAMLToUnstructured(instance.Spec.Data)
	if err != nil {
		log.Printf("unable to marshal data: %v", err)
		return reconcile.Result{}, fmt.Errorf("unable to unmarshal data: %v", err)
	}
	if err = controllerutil.SetControllerReference(instance, child, r.scheme); err != nil {
		log.Printf("unable to add owner reference: %v", err)
		return reconcile.Result{}, fmt.Errorf("unable to add owner reference: %v", err)
	}

	// Check if the Child already exists
	found := &unstructured.Unstructured{}
	found.SetKind(child.GetKind())
	found.SetAPIVersion(child.GetAPIVersion())

	err = r.Get(context.TODO(), types.NamespacedName{Name: child.GetName(), Namespace: child.GetNamespace()}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Printf("Creating child %s %s/%s\n", child.GetKind(), child.GetNamespace(), child.GetName())
		err = r.Create(context.TODO(), child)
		if err != nil {
			log.Printf("unable to create child: %v", err)
			return reconcile.Result{}, fmt.Errorf("unable to create child: %v", err)
		}
		// Now that we have created the object, the version on the API should be
		// the same as the child
		found = child
	} else if err != nil {
		log.Printf("unable to get child: %v", err)
		return reconcile.Result{}, fmt.Errorf("unable to get child: %v", err)
	}

	// Update the object if the spec differs from the version running
	var childSpec, foundSpec interface{}
	var ok bool
	if childSpec, ok = child.UnstructuredContent()["spec"]; !ok {
		return reconcile.Result{}, fmt.Errorf("child has no spec")
	}
	if foundSpec, ok = found.UnstructuredContent()["spec"]; !ok {
		return reconcile.Result{}, fmt.Errorf("found has no spec")
	}
	if !reflect.DeepEqual(childSpec, foundSpec) {
		found.UnstructuredContent()["spec"] = childSpec
		log.Printf("Updating child %s %s/%s\n", child.GetKind(), child.GetNamespace(), child.GetName())
		err = r.Update(context.TODO(), found)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}
