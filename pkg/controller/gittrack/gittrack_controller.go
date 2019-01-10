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
	"strings"
	"sync"
	"time"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gittrackutils "github.com/pusher/faros/pkg/controller/gittrack/utils"
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

	return &ReconcileGitTrack{
		Client:          mgr.GetClient(),
		scheme:          mgr.GetScheme(),
		store:           gitstore.NewRepoStore(),
		restMapper:      restMapper,
		recorder:        mgr.GetRecorder("gittrack-controller"),
		ignoredGVRs:     gvrs,
		lastUpdateTimes: make(map[string]time.Time),
		mutex:           sync.RWMutex{},
		applier:         applier,
	}
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

	err = c.Watch(&source.Kind{Type: &farosv1alpha1.ClusterGitTrackObject{}}, &handler.EnqueueRequestForOwner{
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
	scheme          *runtime.Scheme
	store           *gitstore.RepoStore
	restMapper      meta.RESTMapper
	recorder        record.EventRecorder
	ignoredGVRs     map[schema.GroupVersionResource]interface{}
	lastUpdateTimes map[string]time.Time
	mutex           sync.RWMutex
	applier         farosclient.Client
}

// checkoutRepo checks out the repository at reference and returns a pointer to said repository
func (r *ReconcileGitTrack) checkoutRepo(url string, ref string, privateKey []byte) (*gitstore.Repo, error) {
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

	lastUpdated, err := repo.LastUpdated()
	if err != nil {
		return &gitstore.Repo{}, fmt.Errorf("failed to get last updated timestamp: %v", err)
	}

	r.mutex.Lock()
	r.lastUpdateTimes[url] = lastUpdated
	r.mutex.Unlock()

	return repo, nil
}

// fetchPrivateyKey extracts a given key's data from a given deployKey secret reference
func (r *ReconcileGitTrack) fetchPrivateKey(namespace string, deployKey farosv1alpha1.GitTrackDeployKey) ([]byte, error) {
	// Check if the deployKey is empty, do nothing if it is
	emptyKey := farosv1alpha1.GitTrackDeployKey{}
	if deployKey == emptyKey {
		return []byte{}, nil
	}
	// Check the deployKey fields are both non-empty
	if deployKey.SecretName == "" || deployKey.Key == "" {
		return []byte{}, fmt.Errorf("if using a deploy key, both SecretName and Key must be set")
	}

	// Fetch the secret from the API
	secret := &apiv1.Secret{}
	err := r.Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      deployKey.SecretName,
	}, secret)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to look up secret %s: %v", deployKey.SecretName, err)
	}

	// Extract the data from the secret
	var ok bool
	var privateKey []byte
	if privateKey, ok = secret.Data[deployKey.Key]; !ok {
		return []byte{}, fmt.Errorf("invalid deploy key reference. Secret %s does not have key %s", deployKey.SecretName, deployKey.Key)
	}
	return privateKey, nil
}

// getFiles checks out the Spec.Repository at Spec.Reference and returns a map of filename to
// gitstore.File pointers
func (r *ReconcileGitTrack) getFiles(gt *farosv1alpha1.GitTrack) (map[string]*gitstore.File, error) {
	r.recorder.Eventf(gt, apiv1.EventTypeNormal, "CheckoutStarted", "Checking out '%s' at '%s'", gt.Spec.Repository, gt.Spec.Reference)
	privateKey, err := r.fetchPrivateKey(gt.Namespace, gt.Spec.DeployKey)
	if err != nil {
		r.recorder.Eventf(gt, apiv1.EventTypeWarning, "CheckoutFailed", "Failed to checkout '%s' at '%s'", gt.Spec.Repository, gt.Spec.Reference)
		return nil, fmt.Errorf("unable to retrieve private key: %v", err)
	}

	repo, err := r.checkoutRepo(gt.Spec.Repository, gt.Spec.Reference, privateKey)
	if err != nil {
		r.recorder.Eventf(gt, apiv1.EventTypeWarning, "CheckoutFailed", "Failed to checkout '%s' at '%s'", gt.Spec.Repository, gt.Spec.Reference)
		return nil, err
	}

	subPath := gt.Spec.SubPath
	if !strings.HasSuffix(subPath, "/") {
		subPath += "/"
	}

	globbedSubPath := strings.TrimPrefix(subPath, "/") + "{**/*,*}.{yaml,yml,json}"
	files, err := repo.GetAllFiles(globbedSubPath, true)
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
func (r *ReconcileGitTrack) listObjectsByName(owner *farosv1alpha1.GitTrack) (map[string]farosv1alpha1.GitTrackObjectInterface, error) {
	result := make(map[string]farosv1alpha1.GitTrackObjectInterface)

	gtos := &farosv1alpha1.GitTrackObjectList{}
	err := r.List(context.TODO(), &client.ListOptions{}, gtos)
	if err != nil {
		return nil, err
	}
	for _, gto := range gtos.Items {
		if metav1.IsControlledBy(&gto, owner) {
			result[gto.GetNamespacedName()] = gto.DeepCopy()
		}
	}

	cgtos := &farosv1alpha1.ClusterGitTrackObjectList{}
	err = r.List(context.TODO(), &client.ListOptions{}, cgtos)
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

// result represents the result of creating or updating a GitTrackObject
type result struct {
	NamespacedName string
	Error          error
	Ignored        bool
	InSync         bool
	TimeToDeploy   time.Duration
}

// errorResult is a convenience function for creating an error result
func errorResult(namespacedName string, err error) result {
	return result{NamespacedName: namespacedName, Error: err, Ignored: true}
}

// ignoreResult is a convenience function for creating an ignore result
func ignoreResult(namespacedName string) result {
	return result{NamespacedName: namespacedName, Ignored: true}
}

// successResult is a convenience function for creating a success result
func successResult(namespacedName string, timeToDeploy time.Duration, inSync bool) result {
	return result{NamespacedName: namespacedName, TimeToDeploy: timeToDeploy, InSync: inSync}
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
func (r *ReconcileGitTrack) handleObject(u *unstructured.Unstructured, owner *farosv1alpha1.GitTrack) result {
	name := objectName(u)
	gto, err := r.newGitTrackObjectInterface(name, u)
	if err != nil {
		namespacedName := strings.TrimLeft(fmt.Sprintf("%s/%s", u.GetNamespace(), name), "/")
		return errorResult(namespacedName, err)
	}

	ignored, err := r.ignoreObject(u)
	if err != nil {
		return errorResult(gto.GetNamespacedName(), err)
	}
	if ignored {
		return ignoreResult(gto.GetNamespacedName())
	}

	r.mutex.RLock()
	timeToDeploy := time.Now().Sub(r.lastUpdateTimes[owner.Spec.Repository])
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
		return ignoreResult(gto.GetNamespacedName())
	}

	inSync := childInSync(found)
	childUpdated, err := r.updateChild(found, gto)
	if err != nil {
		r.recorder.Eventf(owner, apiv1.EventTypeWarning, "UpdateFailed", "Failed to update child '%s'", name)
		return errorResult(gto.GetNamespacedName(), fmt.Errorf("failed to update child resource: %v", err))
	}
	if childUpdated {
		inSync = false
		log.Printf("Updated child for '%s'\n", name)
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

func (r *ReconcileGitTrack) createChild(name string, timeToDeploy time.Duration, owner *farosv1alpha1.GitTrack, foundGTO, childGTO farosv1alpha1.GitTrackObjectInterface) result {
	log.Printf("Creating child for '%s'\n", name)
	r.recorder.Eventf(owner, apiv1.EventTypeNormal, "CreateStarted", "Creating child '%s'", name)
	if err := r.applier.Apply(context.TODO(), &farosclient.ApplyOptions{}, childGTO); err != nil {
		r.recorder.Eventf(owner, apiv1.EventTypeWarning, "CreateFailed", "Failed to create child '%s'", name)
		return errorResult(childGTO.GetNamespacedName(), fmt.Errorf("failed to create child for '%s': %v", name, err))
	}
	r.recorder.Eventf(owner, apiv1.EventTypeNormal, "CreateSuccessful", "Created child '%s'", name)
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
	for name, obj := range leftovers {
		log.Printf("Deleting child '%s'\n", name)
		if err := r.Delete(context.TODO(), obj); err != nil {
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

// ignoreObject checks whether the unstructured object should be ignored
func (r *ReconcileGitTrack) ignoreObject(u *unstructured.Unstructured) (bool, error) {
	gvr, namespaced, err := utils.GetAPIResource(r.restMapper, u.GetObjectKind().GroupVersionKind())
	if err != nil {
		return false, err
	}

	// Ignore namespaced objects not in the namespace managed by the controller
	if namespaced && farosflags.Namespace != "" && farosflags.Namespace != u.GetNamespace() {
		return true, nil
	}
	// Ignore GVKs in the ignoredGVKs set
	if _, ok := r.ignoredGVRs[gvr]; ok {
		return true, nil
	}
	return false, nil
}

// Reconcile reads the state of the cluster for a GitTrack object and makes changes based on the state read
// and what is in the GitTrack.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=faros.pusher.com,resources=gittracks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=faros.pusher.com,resources=gittrackobjects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=faros.pusher.com,resources=clustergittrackobjects,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileGitTrack) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	instance := &farosv1alpha1.GitTrack{}
	sOpts := newStatusOpts()
	mOpts := newMetricOpts(sOpts)

	// Update the GitTrackObject status when we leave this function
	defer func() {
		err := r.updateStatus(instance, sOpts)
		mErr := r.updateMetrics(instance, mOpts)
		// Print out any errors that may have occurred
		for _, e := range []error{
			err,
			mErr,
			sOpts.gitError,
			sOpts.parseError,
			sOpts.gcError,
			sOpts.upToDateError,
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

	// Set the repository for metrics
	mOpts.repository = instance.Spec.Repository

	// Get a map of the files that are in the Spec
	files, err := r.getFiles(instance)
	if err != nil {
		sOpts.gitError = err
		sOpts.gitReason = gittrackutils.ErrorFetchingFiles
		return reconcile.Result{}, err
	}
	// Git successful, set condition
	sOpts.gitReason = gittrackutils.GitFetchSuccess
	r.recorder.Eventf(instance, apiv1.EventTypeNormal, "CheckoutSuccessful", "Successfully checked out '%s' at '%s'", instance.Spec.Repository, instance.Spec.Reference)

	// Attempt to parse k8s objects from files
	objects, errors := objectsFrom(files)
	if len(errors) > 0 {
		sOpts.parseError = fmt.Errorf(strings.Join(errors, ",\n"))
		sOpts.parseReason = gittrackutils.ErrorParsingFiles
	} else {
		sOpts.parseReason = gittrackutils.FileParseSuccess
	}

	// Update status with the number of objects discovered
	sOpts.discovered = int64(len(objects))
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
			sOpts.ignored++
		} else {
			sOpts.applied++
		}
		mOpts.timeToDeploy = append(mOpts.timeToDeploy, res.TimeToDeploy)
		if res.InSync {
			sOpts.inSync++
		}
		delete(objectsByName, res.NamespacedName)
		if res.Error != nil {
			handlerErrors = append(handlerErrors, res.Error.Error())
		}
	}

	// If there were errors updating the child objects, set the ChildrenUpToDate
	// condition appropriately
	if len(handlerErrors) > 0 {
		sOpts.upToDateError = fmt.Errorf(strings.Join(handlerErrors, ",\n"))
		sOpts.upToDateReason = gittrackutils.ErrorUpdatingChildren
	} else {
		sOpts.upToDateReason = gittrackutils.ChildrenUpdateSuccess
	}

	// Cleanup potentially leftover resources
	if err = r.deleteResources(objectsByName); err != nil {
		sOpts.gcError = err
		sOpts.gcReason = gittrackutils.ErrorDeletingChildren
		r.recorder.Eventf(instance, apiv1.EventTypeWarning, "CleanupFailed", "Failed to clean-up leftover resources")
		return reconcile.Result{}, fmt.Errorf("failed to clean-up tracked objects: %v", err)
	}
	sOpts.gcReason = gittrackutils.GCSuccess

	return reconcile.Result{}, nil
}
