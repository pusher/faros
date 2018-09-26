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
	"strings"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gittrackutils "github.com/pusher/faros/pkg/controller/gittrack/utils"
	utils "github.com/pusher/faros/pkg/utils"
	gitstore "github.com/pusher/git-store"
	flag "github.com/spf13/pflag"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var privateKeyPath = flag.String("private-key", "", "Path to default private key to use for Git checkouts")
var namespace = flag.String("namespace", "", "Only manage GitTrack resources in given namespace")
var nsPredicate utils.NamespacedPredicate

const ownedByLabel = "faros.pusher.com/owned-by"

// Add creates a new GitTrack Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this faros.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	clientSet := kubernetes.NewForConfigOrDie(mgr.GetConfig())

	// Create a restMapper (used by informer to look up resource kinds)
	restMapper, err := utils.NewRestMapper(mgr.GetConfig())
	if err != nil {
		panic(fmt.Errorf("unable to create rest mapper: %v", err))
	}

	return &ReconcileGitTrack{
		Client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		store:      gitstore.NewRepoStore(clientSet),
		restMapper: restMapper,
		recorder:   mgr.GetRecorder("gittrack-controller"),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("gittrack-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	nsPredicate = utils.NamespacedPredicate{Namespace: *namespace}

	// Watch for changes to GitTrack
	err = c.Watch(&source.Kind{Type: &farosv1alpha1.GitTrack{}}, &handler.EnqueueRequestForObject{}, nsPredicate)
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

	if nsPredicate.Namespace == "" {
		err = c.Watch(&source.Kind{Type: &farosv1alpha1.ClusterGitTrackObject{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &farosv1alpha1.GitTrack{},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileGitTrack{}

// ReconcileGitTrack reconciles a GitTrack object
type ReconcileGitTrack struct {
	client.Client
	scheme     *runtime.Scheme
	store      *gitstore.RepoStore
	restMapper meta.RESTMapper
	recorder   record.EventRecorder
}

// checkoutRepo checks out the repository at reference and returns a pointer to said repository
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

// getFiles checks out the Spec.Repository at Spec.Reference and returns a map of filename to
// gitstore.File pointers
func (r *ReconcileGitTrack) getFiles(gt *farosv1alpha1.GitTrack) (map[string]*gitstore.File, error) {
	r.recorder.Eventf(gt, apiv1.EventTypeNormal, "CheckoutStarted", "Checking out '%s' at '%s'", gt.Spec.Repository, gt.Spec.Reference)
	repo, err := r.checkoutRepo(gt.Spec.Repository, gt.Spec.Reference)
	if err != nil {
		r.recorder.Eventf(gt, apiv1.EventTypeWarning, "CheckoutFailed", "Failed to checkout '%s' at '%s'", gt.Spec.Repository, gt.Spec.Reference)
		return nil, err
	}

	files, err := repo.GetAllFiles(gt.Spec.SubPath)
	if err != nil {
		r.recorder.Eventf(gt, apiv1.EventTypeWarning, "CheckoutFailed", "Failed to get files for SubPath '%s'", gt.Spec.SubPath)
		return nil, fmt.Errorf("failed to get all files for subpath '%s': %v", gt.Spec.SubPath, err)
	} else if len(files) == 0 {
		r.recorder.Eventf(gt, apiv1.EventTypeWarning, "CheckoutFailed", "No files for SubPath '%s'", gt.Spec.SubPath)
		return nil, fmt.Errorf("no files for subpath '%s'", gt.Spec.SubPath)
	}

	return files, nil
}

// fetchInstance attempts to fetch the GitTrack resource by the name in the given Request
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

// listObjectsByName lists and filters GitTrackObjects by the `faros.pusher.com/owned-by` label,
// and returns a map of names to GitTrackObject mappings
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

// makeLabels returns a map of labels that are used for tracking ownership
// without having to list all of the GitTrackObjects every time (but rather
// filter by labels)
func makeLabels(g *farosv1alpha1.GitTrack) map[string]string {
	return map[string]string{ownedByLabel: g.Name}
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

func (r *ReconcileGitTrack) newGitTrackObjectInterface(name string, u *unstructured.Unstructured, labels map[string]string) (farosv1alpha1.GitTrackObjectInterface, error) {
	var instance farosv1alpha1.GitTrackObjectInterface
	_, namespaced, err := utils.GetAPIResource(r.restMapper, u.GetObjectKind().GroupVersionKind())
	if err != nil {
		return nil, fmt.Errorf("error getting API resource: %v", err)
	}
	if namespaced {
		instance = &farosv1alpha1.GitTrackObject{}
	} else {
		instance = &farosv1alpha1.ClusterGitTrackObject{}
	}
	instance.SetName(name)
	instance.SetNamespace(u.GetNamespace())
	instance.SetLabels(labels)

	data, err := u.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("error marshalling JSON: %v", err)
	}

	instance.SetSpec(farosv1alpha1.GitTrackObjectSpec{
		Name: u.GetName(),
		Kind: u.GetKind(),
		Data: data,
	})
	return instance, nil
}

// objectName constructs a name from an Unstructured object
func objectName(u *unstructured.Unstructured) string {
	return strings.ToLower(fmt.Sprintf("%s-%s", u.GetKind(), strings.Replace(u.GetName(), ":", "-", -1)))
}

// handleObject either creates or updates a GitTrackObject
func (r *ReconcileGitTrack) handleObject(u *unstructured.Unstructured, owner *farosv1alpha1.GitTrack) result {
	name := objectName(u)
	gto, err := r.newGitTrackObjectInterface(name, u, makeLabels(owner))
	if err != nil {
		return errorResult(name, err)
	}
	if err = controllerutil.SetControllerReference(owner, gto, r.scheme); err != nil {
		return errorResult(name, err)
	}
	found := gto.DeepCopyInterface()
	err = r.Get(context.TODO(), types.NamespacedName{Name: gto.GetName(), Namespace: gto.GetNamespace()}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Printf("Creating child for '%s'\n", name)
		r.recorder.Eventf(owner, apiv1.EventTypeNormal, "CreateStarted", "Creating child '%s'", name)
		if err = r.Create(context.TODO(), gto); err != nil {
			r.recorder.Eventf(owner, apiv1.EventTypeWarning, "CreateFailed", "Failed to create child '%s'", name)
			return errorResult(name, fmt.Errorf("failed to create child for '%s': %v", name, err))
		}
		r.recorder.Eventf(owner, apiv1.EventTypeNormal, "CreateSuccessful", "Created child '%s'", name)
		return successResult(name)
	} else if err != nil {
		return errorResult(name, fmt.Errorf("failed to get child for '%s': %v", name, err))
	}

	err = checkOwner(owner, found, r.scheme)
	if err != nil {
		return errorResult(name, fmt.Errorf("child '%s' is owned by another controller: %v", name, err))
	}

	childUpdated, err := r.updateChild(found, gto)
	if err != nil {
		return errorResult(name, fmt.Errorf("failed to update child resource: %v", err))
	}
	if childUpdated {
		r.recorder.Eventf(owner, apiv1.EventTypeNormal, "UpdateStarted", "Updating child '%s'", name)
		log.Printf("Updating child for '%s'\n", name)
		if err = r.Update(context.TODO(), found); err != nil {
			r.recorder.Eventf(owner, apiv1.EventTypeWarning, "UpdateFailed", "Failed to update child '%s'", name)
			return errorResult(name, fmt.Errorf("failed to update child for '%s': %v", name, err))
		}
		r.recorder.Eventf(owner, apiv1.EventTypeNormal, "UpdateSuccessful", "Updated child '%s'", name)
	}
	return successResult(name)
}

// UpdateChild compares the two GitTrackObjects and updates the foundGTO if the
// childGTO
func (r *ReconcileGitTrack) updateChild(foundGTO, childGTO farosv1alpha1.GitTrackObjectInterface) (bool, error) {
	// Convert the GitTrackObjects to unstructured
	found := &unstructured.Unstructured{}
	err := r.scheme.Convert(foundGTO, found, nil)
	if err != nil {
		return false, fmt.Errorf("failed to convert found child to Unstructured: %v", err)
	}
	child := &unstructured.Unstructured{}
	err = r.scheme.Convert(childGTO, child, nil)
	if err != nil {
		return false, fmt.Errorf("failed to convert child child to Unstructured: %v", err)
	}

	// Compare and update the resources
	childUpdated, err := utils.UpdateChildResource(found, child)
	if err != nil {
		return false, fmt.Errorf("error updating child resource: %v", err)
	}

	// Child wasn't updated, nothing to do now
	if !childUpdated {
		return false, nil
	}

	// Set the last applied annotation
	err = utils.SetLastAppliedAnnotation(found, child)
	if err != nil {
		return false, fmt.Errorf("error applying last applied annotation: %v", err)
	}

	// Convert Found back to a structured GTO
	err = r.scheme.Convert(found, foundGTO, nil)
	if err != nil {
		return false, fmt.Errorf("error converting object to structured: %v", err)
	}

	return true, nil
}

// deleteResources deletes any resources that are present in the given map
func (r *ReconcileGitTrack) deleteResources(leftovers map[string]farosv1alpha1.GitTrackObject) error {
	for name, gto := range leftovers {
		log.Printf("Deleting child '%s'\n", name)
		if err := r.Delete(context.TODO(), &gto); err != nil {
			return fmt.Errorf("failed to delete child for '%s': '%s'", name, err)
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

// checkOwner checks the owner reference of an object from the API to see if it
// is owned by the current GitTrack.
func checkOwner(owner *farosv1alpha1.GitTrack, child farosv1alpha1.GitTrackObjectInterface, s *runtime.Scheme) error {
	gvk, err := apiutil.GVKForObject(owner, s)
	if err != nil {
		return err
	}

	for _, ref := range child.GetOwnerReferences() {
		if ref.Kind == gvk.Kind && ref.UID != owner.UID {
			return fmt.Errorf("child object is owned by '%s'", ref.Name)
		}
	}
	return nil
}

// Reconcile reads the state of the cluster for a GitTrack object and makes changes based on the state read
// and what is in the GitTrack.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=faros.pusher.com,resources=gittracks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=faros.pusher.com,resources=gittrackobjects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=faros.pusher.com,resources=clustergittrackobjects,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileGitTrack) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	instance := &farosv1alpha1.GitTrack{}
	opts := newStatusOpts()

	if !nsPredicate.Match(request.Namespace) {
		log.Printf("GitTrack, received reconcile request for another namespace: %+v", request)
		// return reconcile.Result{}, nil
	}

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
	files, err := r.getFiles(instance)
	if err != nil {
		opts.gitError = err
		opts.gitReason = gittrackutils.ErrorFetchingFiles
		return reconcile.Result{}, err
	}
	// Git successful, set condition
	opts.gitReason = gittrackutils.GitFetchSuccess
	r.recorder.Eventf(instance, apiv1.EventTypeNormal, "CheckoutSuccessful", "Successfully checked out '%s' at '%s'", instance.Spec.Repository, instance.Spec.Reference)

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
		r.recorder.Eventf(instance, apiv1.EventTypeWarning, "CleanupFailed", "Failed to clean-up leftover resources")
		return reconcile.Result{}, fmt.Errorf("failed to clean-up tracked objects: %v", err)
	}
	opts.gcReason = gittrackutils.GCSuccess

	return reconcile.Result{}, nil
}
