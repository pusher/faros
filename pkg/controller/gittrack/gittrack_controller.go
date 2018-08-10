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
	"log"
	"reflect"
	"strings"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	utils "github.com/pusher/faros/pkg/utils"
	gitstore "github.com/pusher/git-store"
	// corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	// "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new GitTrack Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this faros.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	clientSet := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	return &ReconcileGitTrack{Client: mgr.GetClient(), scheme: mgr.GetScheme(), store: gitstore.NewRepoStore(clientSet)}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("gittrack-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to GitTrack
	err = c.Watch(&source.Kind{Type: &farosv1alpha1.GitTrack{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &farosv1alpha1.GitTrackObject{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &farosv1alpha1.GitTrack{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileGitTrack{}

// ReconcileGitTrack reconciles a GitTrack object
type ReconcileGitTrack struct {
	client.Client
	scheme *runtime.Scheme
	store  *gitstore.RepoStore
}

func (r *ReconcileGitTrack) checkoutRepo(url string, ref string) (*gitstore.Repo, error) {
	log.Printf("Getting repository '%s'\n", url)
	repo, err := r.store.Get(&gitstore.RepoRef{URL: url})
	if err != nil {
		return &gitstore.Repo{}, fmt.Errorf("failed to get repository '%s': %v'", url, err)
	}

	log.Printf("Checking out '%s'\n", ref)
	err = repo.Checkout(ref)
	if err != nil {
		return &gitstore.Repo{}, fmt.Errorf("failed to checkout '%s': %v", ref, err)
	}

	return repo, nil
}

func initGitTrackObject(path string, file *gitstore.File) (*farosv1alpha1.GitTrackObject, error) {
	gto := &farosv1alpha1.GitTrackObject{}
	data := []byte(file.Contents())
	u, err := utils.YAMLToUnstructured(data)

	if err != nil {
		log.Printf("unable to parse '%s': %v\n", path, err)
		return gto, err
	}

	if u.GetNamespace() == "" {
		log.Printf("missing 'namespace' for '%s', ignoring\n", path)
		return gto, err
	}

	name := strings.ToLower(fmt.Sprintf("%s-%s", u.GetKind(), u.GetName()))
	gto.ObjectMeta = metav1.ObjectMeta{Name: name, Namespace: u.GetNamespace()}
	gto.Spec = farosv1alpha1.GitTrackObjectSpec{Name: u.GetName(), Kind: u.GetKind(), Data: data}
	return gto, nil
}

// Reconcile reads the state of the cluster for a GitTrack object and makes changes based on the state read
// and what is in the GitTrack.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=faros.pusher.com,resources=gittracks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=faros.pusher.com,resources=gittrackobjects,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileGitTrack) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the GitTrack instance
	instance := &farosv1alpha1.GitTrack{}
	status := &farosv1alpha1.GitTrackStatus{}
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

	repo, err := r.checkoutRepo(instance.Spec.Repository, instance.Spec.Reference)
	if err != nil {
		return reconcile.Result{}, err
	}

	files, err := repo.GetAllFiles(instance.Spec.SubPath)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get all files for subpath '%s': %v", instance.Spec.SubPath, err)
	}

	// TODO: deal with files that contain multiple objects
	status.ObjectsDiscovered = int64(len(files))

	for path, file := range files {
		gto, err := initGitTrackObject(path, file)
		if err != nil {
			log.Printf("%v\n", err)
			status.ObjectsIgnored++
		}

		found := &farosv1alpha1.GitTrackObject{}
		err = r.Get(context.TODO(), types.NamespacedName{Name: gto.ObjectMeta.Name, Namespace: gto.ObjectMeta.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			log.Printf("Creating GitTrackObject for '%s'\n", path)
			err = r.Create(context.TODO(), gto)
			if err != nil {
				log.Printf("%v\n", err)
				return reconcile.Result{}, fmt.Errorf("failed to create GitTrackObject for '%s': %v", path, err)
			}
			status.ObjectsApplied++
		} else if err != nil {
			log.Printf("%v\n", err)
			return reconcile.Result{}, fmt.Errorf("failed to get GitTrackObject for '%s': %v", path, err)
		} else {
			if !reflect.DeepEqual(gto.Spec, found.Spec) {
				found.Spec = gto.Spec
				log.Printf("Updating GitTrackObject %s/%s\n", gto.Namespace, gto.Name)
				err = r.Update(context.TODO(), found)
				if err != nil {
					log.Printf("%v\n", err)
					return reconcile.Result{}, fmt.Errorf("failed to update GitTrackObject for '%s': %v", path, err)
				}
				status.ObjectsApplied++
			}
		}
	}
	if !reflect.DeepEqual(instance.Status, status) {
		instance.Status = *(status)
		err = r.Update(context.TODO(), instance)
		if err != nil {
			// TODO: should we return an error here or nah?
			log.Printf("%v\n", err)
		}
	}
	return reconcile.Result{}, nil
}
