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
	farosclient "github.com/pusher/faros/pkg/utils/client"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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

	// Create a client using the restMapper
	restClient, err := client.New(mgr.GetConfig(), client.Options{Mapper: restMapper})
	if err != nil {
		panic(fmt.Errorf("unable to create rest client: %v", err))
	}

	// Create a cache using the restMapper
	restCache, _ := cache.New(mgr.GetConfig(), cache.Options{Mapper: restMapper})
	if err != nil {
		panic(fmt.Errorf("unable to create rest cache: %v", err))
	}

	applier, err := farosclient.NewApplier(mgr.GetConfig(), farosclient.Options{})
	if err != nil {
		panic(fmt.Errorf("unable to create applier: %v", err))
	}

	dryRunVerifier, err := utils.NewDryRunVerifier(mgr.GetConfig())
	if err != nil {
		panic(fmt.Errorf("unable to create dry run verifier: %v", err))
	}

	return &ReconcileGitTrackObject{
		Client:         restClient,
		scheme:         mgr.GetScheme(),
		eventStream:    make(chan event.GenericEvent),
		cache:          restCache,
		informers:      make(map[string]cache.Informer),
		config:         mgr.GetConfig(),
		stop:           stop,
		recorder:       mgr.GetEventRecorderFor("gittrackobject-controller"),
		applier:        applier,
		dryRunVerifier: dryRunVerifier,
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
			},
			utils.NewOwnersOwnerInNamespacePredicate(mgr.GetClient()),
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
	informers   map[string]cache.Informer
	config      *rest.Config
	stop        chan struct{}
	recorder    record.EventRecorder

	applier        farosclient.Client
	dryRunVerifier *utils.DryRunVerifier
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
	// Fetch the GTO requested
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

	// Create new opts structs for updating status and metrics
	result := r.handleGitTrackObject(instance)
	r.updateStatus(instance, &statusOpts{inSyncError: result.inSyncError, inSyncReason: result.inSyncReason})
	inSync := result.inSyncError == nil
	r.updateMetrics(instance, &metricsOpts{inSync: inSync})

	if result.inSyncError != nil {
		log.Printf("error syncing child %s: %v", instance.GetNamespacedName(), result.inSyncError)
	}

	return reconcile.Result{}, result.inSyncError
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
