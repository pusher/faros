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
	"sync"
	"time"

	"github.com/go-logr/logr"
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	farosflags "github.com/pusher/faros/pkg/flags"
	utils "github.com/pusher/faros/pkg/utils"
	farosclient "github.com/pusher/faros/pkg/utils/client"
	gitstore "github.com/pusher/git-store"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	rlogr "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new GitTrack Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this faros.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	recFn, opts := newReconciler(mgr)
	return add(mgr, recFn, opts)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) (reconcile.Reconciler, *reconcileGitTrackOpts) {
	// Create a restMapper (used by informer to look up resource kinds)
	restMapper, err := utils.NewRestMapper(mgr.GetConfig())
	if err != nil {
		panic(fmt.Errorf("unable to create rest mapper: %v", err))
	}

	gvrs, err := farosflags.ParseIgnoredResources()
	if err != nil {
		panic(fmt.Errorf("unable to parse ignored resources: %v", err))
	}

	applier, err := farosclient.NewApplier(mgr.GetConfig(), farosclient.Options{})
	if err != nil {
		panic(fmt.Errorf("unable to create applier: %v", err))
	}

	rec := &ReconcileGitTrack{
		Client:              mgr.GetClient(),
		scheme:              mgr.GetScheme(),
		store:               gitstore.NewRepoStore(farosflags.RepositoryDir),
		restMapper:          restMapper,
		recorder:            mgr.GetEventRecorderFor("gittrack-controller"),
		ignoredGVRs:         gvrs,
		lastUpdateTimes:     make(map[string]time.Time),
		mutex:               &sync.RWMutex{},
		applier:             applier,
		log:                 rlogr.Log.WithName("gittrack-controller"),
		namespace:           farosflags.Namespace,
		clusterGitTrackMode: farosflags.ClusterGitTrack,
	}
	opts := &reconcileGitTrackOpts{
		clusterGitTrackMode: farosflags.ClusterGitTrack,
	}
	return rec, opts
}

type reconcileGitTrackOpts struct {
	clusterGitTrackMode farosflags.ClusterGitTrackMode
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler, opts *reconcileGitTrackOpts) error {
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

	if opts.clusterGitTrackMode != farosflags.CGTMDisabled {
		// Watch for changes to ClusterGitTrack
		err = c.Watch(&source.Kind{Type: &farosv1alpha1.ClusterGitTrack{}}, &handler.EnqueueRequestForObject{})
		if err != nil {
			return err
		}

		err = c.Watch(&source.Kind{Type: &farosv1alpha1.ClusterGitTrackObject{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &farosv1alpha1.ClusterGitTrack{},
		})
		if err != nil {
			return err
		}
	}

	err = c.Watch(&source.Kind{Type: &farosv1alpha1.GitTrackObject{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &farosv1alpha1.GitTrack{},
	})
	if err != nil {
		return err
	}

	// TODO(dmo): disable this watch once we've made it so that gittracks cannot create clustergittrackobjects
	err = c.Watch(&source.Kind{Type: &farosv1alpha1.ClusterGitTrackObject{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &farosv1alpha1.GitTrack{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &farosv1alpha1.GitTrackObject{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &farosv1alpha1.ClusterGitTrack{},
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
	scheme          *runtime.Scheme
	store           *gitstore.RepoStore
	restMapper      meta.RESTMapper
	recorder        record.EventRecorder
	ignoredGVRs     map[schema.GroupVersionResource]interface{}
	lastUpdateTimes map[string]time.Time
	mutex           *sync.RWMutex
	applier         farosclient.Client
	log             logr.Logger

	namespace           string
	clusterGitTrackMode farosflags.ClusterGitTrackMode
}

func (r *ReconcileGitTrack) withValues(keysAndValues ...interface{}) *ReconcileGitTrack {
	reconciler := *r
	reconciler.log = r.log.WithValues(keysAndValues...)
	return &reconciler
}

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
	r.recorder.Eventf(gt, apiv1.EventTypeNormal, "CheckoutStarted", "Checking out '%s' at '%s'", gt.GetSpec().Repository, gt.GetSpec().Reference)
	gitCreds, err := r.fetchGitCredentials(gt)
	if err != nil {
		r.recorder.Eventf(gt, apiv1.EventTypeWarning, "CheckoutFailed", "Failed to checkout '%s' at '%s'", gt.GetSpec().Repository, gt.GetSpec().Reference)
		return nil, fmt.Errorf("unable to retrieve git credentials from secret: %v", err)
	}

	repo, err := r.checkoutRepo(gt.GetSpec().Repository, gt.GetSpec().Reference, gitCreds)
	if err != nil {
		r.recorder.Eventf(gt, apiv1.EventTypeWarning, "CheckoutFailed", "Failed to checkout '%s' at '%s'", gt.GetSpec().Repository, gt.GetSpec().Reference)
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
		r.recorder.Eventf(gt, apiv1.EventTypeWarning, "CheckoutFailed", "Failed to get files for SubPath '%s'", gt.GetSpec().SubPath)
		return nil, fmt.Errorf("failed to get all files for subpath '%s': %v", gt.GetSpec().SubPath, err)
	} else if len(files) == 0 {
		r.recorder.Eventf(gt, apiv1.EventTypeWarning, "CheckoutFailed", "No files for SubPath '%s'", gt.GetSpec().SubPath)
		return nil, fmt.Errorf("no files for subpath '%s'", gt.GetSpec().SubPath)
	}

	r.log.V(1).Info("Loaded files from repository", "file count", len(files))
	return files, nil
}

// fetchInstance attempts to fetch the GitTrack resource by the name in the given Request
func (r *ReconcileGitTrack) fetchInstance(req reconcile.Request) (farosv1alpha1.GitTrackInterface, error) {
	var instance farosv1alpha1.GitTrackInterface
	if req.Namespace != "" {
		instance = &farosv1alpha1.GitTrack{}
	} else {
		instance = &farosv1alpha1.ClusterGitTrack{}
	}
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
func (r *ReconcileGitTrack) listObjectsByName(owner farosv1alpha1.GitTrackInterface) (map[string]farosv1alpha1.GitTrackObjectInterface, error) {
	result := make(map[string]farosv1alpha1.GitTrackObjectInterface)

	gtos := &farosv1alpha1.GitTrackObjectList{}
	err := r.List(context.TODO(), gtos)
	if err != nil {
		return nil, err
	}
	for _, gto := range gtos.Items {
		if metav1.IsControlledBy(&gto, owner) {
			result[gto.GetNamespacedName()] = gto.DeepCopy()
		}
	}

	cgtos := &farosv1alpha1.ClusterGitTrackObjectList{}
	err = r.List(context.TODO(), cgtos)
	if err != nil {
		return nil, err
	}
	for _, cgto := range cgtos.Items {
		if metav1.IsControlledBy(&cgto, owner) {
			result[cgto.GetNamespacedName()] = cgto.DeepCopy()
		}
	}

	return result, nil
}

// objectResult represents the result of creating or updating a GitTrackObject
type objectResult struct {
	NamespacedName string
	Error          error
	Ignored        bool
	Reason         string
	InSync         bool
	TimeToDeploy   time.Duration
}

// errorResult is a convenience function for creating an error result
func errorResult(namespacedName string, err error) objectResult {
	return objectResult{NamespacedName: namespacedName, Error: err, Ignored: true}
}

// ignoreResult is a convenience function for creating an ignore objectResult
func ignoreResult(namespacedName string, reason string) objectResult {
	return objectResult{NamespacedName: namespacedName, Ignored: true, Reason: reason}
}

// successResult is a convenience function for creating a success objectResult
func successResult(namespacedName string, timeToDeploy time.Duration, inSync bool) objectResult {
	return objectResult{NamespacedName: namespacedName, TimeToDeploy: timeToDeploy, InSync: inSync}
}

func (r *ReconcileGitTrack) newGitTrackObjectInterface(name string, u *unstructured.Unstructured) (farosv1alpha1.GitTrackObjectInterface, error) {
	var instance farosv1alpha1.GitTrackObjectInterface
	_, namespaced, err := utils.GetAPIResource(r.restMapper, u.GetObjectKind().GroupVersionKind())
	if err != nil {
		return nil, fmt.Errorf("error getting API resource: %v", err)
	}
	if namespaced {
		instance = &farosv1alpha1.GitTrackObject{
			TypeMeta: farosv1alpha1.GitTrackObjectTypeMeta,
		}
	} else {
		instance = &farosv1alpha1.ClusterGitTrackObject{
			TypeMeta: farosv1alpha1.ClusterGitTrackObjectTypeMeta,
		}
	}
	instance.SetName(name)
	instance.SetNamespace(u.GetNamespace())

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
func (r *ReconcileGitTrack) handleObject(u *unstructured.Unstructured, owner farosv1alpha1.GitTrackInterface) objectResult {
	name := objectName(u)
	gto, err := r.newGitTrackObjectInterface(name, u)
	if err != nil {
		namespacedName := strings.TrimLeft(fmt.Sprintf("%s/%s", u.GetNamespace(), name), "/")
		return errorResult(namespacedName, err)
	}

	ignored, reason, err := r.ignoreObject(u, owner)
	if err != nil {
		return errorResult(gto.GetNamespacedName(), err)
	}
	if ignored {
		return ignoreResult(gto.GetNamespacedName(), reason)
	}

	r.mutex.RLock()
	timeToDeploy := time.Now().Sub(r.lastUpdateTimes[owner.GetSpec().Repository])
	r.mutex.RUnlock()

	if err = controllerutil.SetControllerReference(owner, gto, r.scheme); err != nil {
		return errorResult(gto.GetNamespacedName(), err)
	}
	found := gto.DeepCopyInterface()
	err = r.Get(context.TODO(), types.NamespacedName{Name: gto.GetName(), Namespace: gto.GetNamespace()}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.createChild(name, timeToDeploy, owner, found, gto)
	} else if err != nil {
		return errorResult(gto.GetNamespacedName(), fmt.Errorf("failed to get child for '%s': %v", name, err))
	}

	err = checkOwner(owner, found, r.scheme)
	if err != nil {
		r.recorder.Eventf(owner, apiv1.EventTypeWarning, "ControllerMismatch", "Child '%s' is owned by another controller: %v", name, err)
		return ignoreResult(gto.GetNamespacedName(), "child is owned by another controller")
	}

	inSync := childInSync(found)
	childUpdated, err := r.updateChild(found, gto)
	if err != nil {
		r.recorder.Eventf(owner, apiv1.EventTypeWarning, "UpdateFailed", "Failed to update child '%s'", name)
		return errorResult(gto.GetNamespacedName(), fmt.Errorf("failed to update child resource: %v", err))
	}
	if childUpdated {
		inSync = false
		r.log.V(0).Info("Child updated", "child name", name)
		r.recorder.Eventf(owner, apiv1.EventTypeNormal, "UpdateSuccessful", "Updated child '%s'", name)
	}
	return successResult(gto.GetNamespacedName(), timeToDeploy, inSync)
}

func childInSync(child farosv1alpha1.GitTrackObjectInterface) bool {
	for _, condition := range child.GetStatus().Conditions {
		if condition.Type == farosv1alpha1.ObjectInSyncType && condition.Status == apiv1.ConditionTrue {
			return true
		}
	}
	return false
}

func (r *ReconcileGitTrack) createChild(name string, timeToDeploy time.Duration, owner farosv1alpha1.GitTrackInterface, foundGTO, childGTO farosv1alpha1.GitTrackObjectInterface) objectResult {
	r.recorder.Eventf(owner, apiv1.EventTypeNormal, "CreateStarted", "Creating child '%s'", name)
	if err := r.applier.Apply(context.TODO(), &farosclient.ApplyOptions{}, childGTO); err != nil {
		r.recorder.Eventf(owner, apiv1.EventTypeWarning, "CreateFailed", "Failed to create child '%s'", name)
		return errorResult(childGTO.GetNamespacedName(), fmt.Errorf("failed to create child for '%s': %v", name, err))
	}
	r.recorder.Eventf(owner, apiv1.EventTypeNormal, "CreateSuccessful", "Created child '%s'", name)
	r.log.V(0).Info("Child created", "child name", name)
	return successResult(childGTO.GetNamespacedName(), timeToDeploy, false)
}

// UpdateChild compares the two GitTrackObjects and updates the foundGTO if the
// childGTO
func (r *ReconcileGitTrack) updateChild(foundGTO, childGTO farosv1alpha1.GitTrackObjectInterface) (bool, error) {
	originalResourceVersion := foundGTO.GetResourceVersion()
	err := r.applier.Apply(context.TODO(), &farosclient.ApplyOptions{}, childGTO)
	if err != nil {
		return false, fmt.Errorf("error updating child resource: %v", err)
	}

	// Not updated if the resource version hasn't changed
	if originalResourceVersion == childGTO.GetResourceVersion() {
		return false, nil
	}

	return true, nil
}

// deleteResources deletes any resources that are present in the given map
func (r *ReconcileGitTrack) deleteResources(leftovers map[string]farosv1alpha1.GitTrackObjectInterface) error {
	if len(leftovers) > 0 {
		r.log.V(0).Info("Found leftover resources to clean up", "leftover resources", string(len(leftovers)))
	}
	for name, obj := range leftovers {
		if err := r.Delete(context.TODO(), obj); err != nil {
			return fmt.Errorf("failed to delete child for '%s': '%s'", name, err)
		}
		r.log.V(0).Info("Child deleted", "child name", name)
	}
	return nil
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

// checkOwner checks the owner reference of an object from the API to see if it
// is owned by the current GitTrack.
func checkOwner(owner farosv1alpha1.GitTrackInterface, child farosv1alpha1.GitTrackObjectInterface, s *runtime.Scheme) error {
	gvk, err := apiutil.GVKForObject(owner, s)
	if err != nil {
		return err
	}

	for _, ref := range child.GetOwnerReferences() {
		if ref.Kind == gvk.Kind && ref.UID != owner.GetUID() {
			return fmt.Errorf("child object is owned by '%s'", ref.Name)
		}
	}
	return nil
}

// ignoreObject checks whether the unstructured object should be ignored
func (r *ReconcileGitTrack) ignoreObject(u *unstructured.Unstructured, owner farosv1alpha1.GitTrackInterface) (bool, string, error) {
	gvr, namespaced, err := utils.GetAPIResource(r.restMapper, u.GetObjectKind().GroupVersionKind())
	if err != nil {
		return false, "", err
	}

	// Ignore namespaced objects not in the namespace managed by the controller
	if namespaced && r.namespace != "" && r.namespace != u.GetNamespace() {
		r.log.V(1).Info("Object not in namespace", "object namespace", u.GetNamespace(), "managed namespace", r.namespace)
		return true, fmt.Sprintf("namespace `%s` is not managed by this Faros", u.GetNamespace()), nil
	}
	// Ignore GVKs in the ignoredGVKs set
	if _, ok := r.ignoredGVRs[gvr]; ok {
		r.log.V(1).Info("Object group version ignored globally", "group version resource", gvr.String())
		return true, fmt.Sprintf("resource `%s.%s/%s` ignored globally by flag", gvr.Resource, gvr.Group, gvr.Version), nil
	}

	ownerNamespace := owner.GetNamespace()
	_, ownerIsGittrack := owner.(*farosv1alpha1.GitTrack)
	_, ownerIsClusterGittrack := owner.(*farosv1alpha1.ClusterGitTrack)
	if namespaced {
		// prevent a gittrack in a namespace from handling objects which are in a different namespace
		if ownerIsGittrack && ownerNamespace != u.GetNamespace() {
			return true, fmt.Sprintf("namespace `%s` is not managed by this GitTrack", u.GetNamespace()), nil
		} else if ownerIsClusterGittrack && r.clusterGitTrackMode == farosflags.CGTMExcludeNamespaced {
			return true, "namespaced resources cannot be managed by ClusterGitTrack", nil
		}
	}

	if ownerIsClusterGittrack && r.clusterGitTrackMode == farosflags.CGTMDisabled {
		return true, "ClusterGitTrack handling disabled; ignoring", nil
	}

	return false, "", nil
}

// Reconcile reads the state of the cluster for a GitTrack object and makes changes based on the state read
// and what is in the GitTrack.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=faros.pusher.com,resources=gittracks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=faros.pusher.com,resources=gittrackobjects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=faros.pusher.com,resources=clustergittrackobjects,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileGitTrack) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	instance, err := r.fetchInstance(request)
	if err != nil || instance == nil {
		return reconcile.Result{}, err
	}

	reconciler := r.withValues(
		"namespace", instance.GetNamespace(),
		"name", instance.GetName(),
	)

	reconciler.log.V(1).Info("Reconcile started")
	defer reconciler.log.V(1).Info("Reconcile finished")

	result := reconciler.handleGitTrack(instance)
	var errs []error
	for _, err := range []error{result.parseError, result.gitError, result.gcError, result.upToDateError} {
		if err != nil {
			errs = append(errs, err)
		}
	}

	err = reconciler.updateStatus(instance, result.asStatusOpts())
	if err != nil {
		errs = append(errs, fmt.Errorf("error updating status: %v", err))
	}
	err = reconciler.updateMetrics(instance, result.asMetricOpts(instance.GetSpec().Repository))
	if err != nil {
		errs = append(errs, fmt.Errorf("error updating metrics: %v", err))
	}

	if len(errs) > 0 {
		for _, err := range errs {
			reconciler.log.Error(err, "error during reconcile")
		}
		return reconcile.Result{}, fmt.Errorf("errors encountered during reconcile")
	}

	return reconcile.Result{}, nil
}
