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
	"io/ioutil"
	"log"
	"reflect"
	"strings"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gittrackutils "github.com/pusher/faros/pkg/controller/gittrack/utils"
	utils "github.com/pusher/faros/pkg/utils"
	gitstore "github.com/pusher/git-store"
	flag "github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var privateKeyPath = flag.String("private-key", "", "Path to default private key to use for Git checkouts")

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
	privateKey := []byte{}
	var err error
	if *privateKeyPath != "" {
		privateKey, err = ioutil.ReadFile(*privateKeyPath)
		if err != nil {
			return &gitstore.Repo{}, fmt.Errorf("failed to load private key: %v", err)
		}
	}

	log.Printf("Getting repository '%s'\n", url)
	repo, err := r.store.Get(&gitstore.RepoRef{URL: url, PrivateKey: privateKey})
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

func (r *ReconcileGitTrack) getFilesForSpec(s farosv1alpha1.GitTrackSpec) (map[string]*gitstore.File, error) {
	repo, err := r.checkoutRepo(s.Repository, s.Reference)
	if err != nil {
		return nil, err
	}

	files, err := repo.GetAllFiles(s.SubPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get all files for subpath '%s': %v", s.SubPath, err)
	} else if len(files) == 0 {
		return nil, fmt.Errorf("no files for subpath '%s'", s.SubPath)
	}

	return files, nil
}

func (r *ReconcileGitTrack) fetchInstance(req reconcile.Request) (*farosv1alpha1.GitTrack, error) {
	instance := &farosv1alpha1.GitTrack{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return nil, nil
		}
		// Error reading the object - requeue the request.
		return nil, err
	}
	return instance, nil
}

func (r *ReconcileGitTrack) listObjectsByName(owner *farosv1alpha1.GitTrack) (map[string]farosv1alpha1.GitTrackObject, error) {
	result := make(map[string]farosv1alpha1.GitTrackObject)
	gtos := &farosv1alpha1.GitTrackObjectList{}
	opts := client.InNamespace(owner.Namespace).MatchingLabels(makeLabels(owner))
	err := r.List(context.TODO(), opts, gtos)
	if err != nil {
		return nil, err
	}
	for _, gto := range gtos.Items {
		result[gto.Name] = gto
	}
	return result, nil
}

func makeLabels(g *farosv1alpha1.GitTrack) map[string]string {
	return map[string]string{"faros.pusher.com/owned-by": g.Name}
}

// result represents the result of creating or updating a GitTrackObject
type result struct {
	Name    string
	Error   error
	Ignored bool
}

// errorResult is a convenience function for creating an error result
func errorResult(name string, err error) result {
	return result{Name: name, Error: err, Ignored: true}
}

// successResult is a convenience function for creating a success result
func successResult(name string) result {
	return result{Name: name}
}

// newGitTrackObject initializes a GitTrackObject from a name and an Unstructured
func newGitTrackObject(name string, u *unstructured.Unstructured, labels map[string]string) *farosv1alpha1.GitTrackObject {
	data, _ := u.MarshalJSON()
	return &farosv1alpha1.GitTrackObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: u.GetNamespace(),
			Labels:    labels,
		},
		Spec: farosv1alpha1.GitTrackObjectSpec{
			Name: u.GetName(),
			Kind: u.GetKind(),
			Data: data,
		},
	}
}

// objectName constructs a name from an Unstructured object
func objectName(u *unstructured.Unstructured) string {
	return strings.ToLower(fmt.Sprintf("%s-%s", u.GetKind(), u.GetName()))
}

// handleObject either creates or updates a GitTrackObject
func (r *ReconcileGitTrack) handleObject(u *unstructured.Unstructured, owner *farosv1alpha1.GitTrack) result {
	name := objectName(u)
	gto := newGitTrackObject(name, u, makeLabels(owner))
	if err := controllerutil.SetControllerReference(owner, gto, r.scheme); err != nil {
		return errorResult(name, err)
	}
	found := &farosv1alpha1.GitTrackObject{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: gto.Name, Namespace: gto.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Printf("Creating GitTrackObject for '%s'\n", name)
		if err = r.Create(context.TODO(), gto); err != nil {
			return errorResult(name, fmt.Errorf("failed to create GitTrackObject for '%s': %v", name, err))
		}
		return successResult(name)
	} else if err != nil {
		return errorResult(name, fmt.Errorf("failed to get GitTrackObject for '%s': %v", name, err))
	}

	if !reflect.DeepEqual(gto.Spec, found.Spec) {
		found.Spec = gto.Spec
		log.Printf("Updating GitTrackObject for '%s'\n", name)
		if err = r.Update(context.TODO(), found); err != nil {
			return errorResult(name, fmt.Errorf("failed to update GitTrackObject for '%s': %v", name, err))
		}
	}
	return successResult(name)
}

// deleteResources deletes any resources that are present in the given map
func (r *ReconcileGitTrack) deleteResources(leftovers map[string]farosv1alpha1.GitTrackObject) error {
	for name, gto := range leftovers {
		log.Printf("Deleting GitTrackObject '%s'\n", name)
		if err := r.Delete(context.TODO(), &gto); err != nil {
			return fmt.Errorf("failed to delete GitTrackObject for '%s': '%s'", name, err)
		}
	}
	return nil
}

// objectsFrom iterates through all the files given and attempts to create Unstructured objects
func objectsFrom(files map[string]*gitstore.File) ([]*unstructured.Unstructured, []string) {
	objects := []*unstructured.Unstructured{}
	errors := []string{}
	for path, file := range files {
		us, err := utils.YAMLToUnstructuredSlice([]byte(file.Contents()))
		if err != nil {
			errors = append(errors, fmt.Sprintf("unable to parse '%s': %v\n", path, err))
			continue
		}
		objects = append(objects, us...)
	}
	return objects, errors
}

// Reconcile reads the state of the cluster for a GitTrack object and makes changes based on the state read
// and what is in the GitTrack.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=faros.pusher.com,resources=gittracks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=faros.pusher.com,resources=gittrackobjects,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileGitTrack) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	instance := &farosv1alpha1.GitTrack{}
	opts := newStatusOpts()

	// Update the GitTrackObject status when we leave this function
	defer func() {
		err := r.updateStatus(instance, opts)
		// Print out any errors that may have occurred
		for _, e := range []error{
			err,
			opts.gitError,
			opts.parseError,
			opts.gcError,
			opts.upToDateError,
		} {
			if e != nil {
				log.Printf("%v", e)
			}
		}
	}()

	var err error
	instance, err = r.fetchInstance(request)
	if err != nil || instance == nil {
		return reconcile.Result{}, err
	}

	// Get a map of the files that are in the Spec
	files, err := r.getFilesForSpec(instance.Spec)
	if err != nil {
		opts.gitError = err
		opts.gitReason = gittrackutils.ErrorFetchingFiles
		return reconcile.Result{}, err
	}
	// Git successful, set condition
	opts.gitReason = gittrackutils.GitFetchSuccess

	// Attempt to parse k8s objects from files
	objects, errors := objectsFrom(files)
	if len(errors) > 0 {
		opts.parseError = fmt.Errorf(strings.Join(errors, ",\n"))
		opts.parseReason = gittrackutils.ErrorParsingFiles
	} else {
		opts.parseReason = gittrackutils.FileParseSuccess
	}

	// Update status with the number of objects discovered
	opts.discovered = int64(len(objects))
	// Get a list of the GitTrackObjects that currently exist, by name
	objectsByName, err := r.listObjectsByName(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	// Process the objects and feed back the results
	resultsChan := make(chan result, len(objects))
	for _, obj := range objects {
		go func(obj *unstructured.Unstructured) {
			resultsChan <- r.handleObject(obj, instance)
		}(obj)
	}

	handlerErrors := []string{}
	// Iterate through results and update status accordingly
	for range objects {
		res := <-resultsChan
		if res.Ignored {
			opts.ignored++
		} else {
			opts.applied++
		}
		delete(objectsByName, res.Name)
		if res.Error != nil {
			handlerErrors = append(handlerErrors, res.Error.Error())
		}
	}

	// If there were errors updating the child objects, set the ChildrenUpToDate
	// condition appropriately
	if len(handlerErrors) > 0 {
		opts.upToDateError = fmt.Errorf(strings.Join(handlerErrors, ",\n"))
		opts.upToDateReason = gittrackutils.ErrorUpdatingChildren
	} else {
		opts.upToDateReason = gittrackutils.ChildrenUpdateSuccess
	}

	// Cleanup potentially leftover resources
	if err = r.deleteResources(objectsByName); err != nil {
		opts.gcError = err
		opts.gcReason = gittrackutils.ErrorDeletingChildren
		return reconcile.Result{}, fmt.Errorf("failed to cleanup tracked objects: %v", err)
	}
	opts.gcReason = gittrackutils.GCSuccess

	return reconcile.Result{}, nil
}
