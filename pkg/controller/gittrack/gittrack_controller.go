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
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func initGitTrackObject(path string, file *gitstore.File) (*farosv1alpha1.GitTrackObject, error) {
	gto := &farosv1alpha1.GitTrackObject{}
	data := []byte(file.Contents())
	u, err := utils.YAMLToUnstructured(data)

	if err != nil {
		log.Printf("unable to parse '%s': %v\n", path, err)
		return gto, err
	}

	if u.GetNamespace() == "" {
		log.Printf("missing 'namespace' in '%s', ignoring\n", path)
		return gto, fmt.Errorf("missing 'namespace' in %s", path)
	}

	name := strings.ToLower(fmt.Sprintf("%s-%s", u.GetKind(), u.GetName()))
	gto.ObjectMeta = metav1.ObjectMeta{Name: name, Namespace: u.GetNamespace()}
	gto.Spec = farosv1alpha1.GitTrackObjectSpec{Name: u.GetName(), Kind: u.GetKind(), Data: data}

	return gto, nil
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

func success(c farosv1alpha1.GitTrackCondition, m string) farosv1alpha1.GitTrackCondition {
	now := metav1.Now()
	c.Status = v1.ConditionFalse
	c.LastUpdateTime = now
	c.LastTransitionTime = now
	if m != "" {
		c.Reason = m
		c.Message = m
	}
	return c
}

func failure(c farosv1alpha1.GitTrackCondition, m string) farosv1alpha1.GitTrackCondition {
	now := metav1.Now()
	c.Status = v1.ConditionTrue
	c.LastUpdateTime = now
	c.LastTransitionTime = now
	if m != "" {
		c.Reason = m
		c.Message = m
	}
	return c
}

// Reconcile reads the state of the cluster for a GitTrack object and makes changes based on the state read
// and what is in the GitTrack.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=faros.pusher.com,resources=gittracks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=faros.pusher.com,resources=gittrackobjects,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileGitTrack) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	status := &farosv1alpha1.GitTrackStatus{}
	parseErrorCondition := farosv1alpha1.GitTrackCondition{
		Type:   farosv1alpha1.ParseErrorType,
		Status: v1.ConditionUnknown,
	}
	gitErrorCondition := farosv1alpha1.GitTrackCondition{
		Type:   farosv1alpha1.GitErrorType,
		Status: v1.ConditionUnknown,
	}
	instance, err := r.fetchInstance(request)
	if err != nil {
		return reconcile.Result{}, err
	} else if instance == nil {
		return reconcile.Result{}, nil
	}

	files, err := r.getFilesForSpec(instance.Spec)
	if err != nil {
		status.Conditions = []farosv1alpha1.GitTrackCondition{
			success(parseErrorCondition, ""),
			failure(gitErrorCondition, fmt.Sprintf("%v", err)),
		}

		if !reflect.DeepEqual(instance.Status, status) {
			instance.Status = *(status)
			updateErr := r.Update(context.TODO(), instance)
			if updateErr != nil {
				log.Printf("Unable to update status: '%v'\n", updateErr)
			}
		}
		return reconcile.Result{}, err
	}

	// TODO: deal with files that contain multiple objects
	status.ObjectsDiscovered = int64(len(files))

	for path, file := range files {
		log.Printf("Processing '%s'\n", path)
		gto, err := initGitTrackObject(path, file)
		if err != nil {
			status.ObjectsIgnored++
			continue
		}

		if err = controllerutil.SetControllerReference(instance, gto, r.scheme); err != nil {
			// TODO: ignore or requeue?
			return reconcile.Result{}, err
		}

		found := &farosv1alpha1.GitTrackObject{}
		err = r.Get(context.TODO(), types.NamespacedName{Name: gto.Name, Namespace: gto.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			log.Printf("Creating GitTrackObject for '%s'\n", path)
			if err = r.Create(context.TODO(), gto); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to create GitTrackObject for '%s': %v", path, err)
			}
			status.ObjectsApplied++
			continue
		} else if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to get GitTrackObject for '%s': %v", path, err)
		}

		if !reflect.DeepEqual(gto.Spec, found.Spec) {
			found.Spec = gto.Spec
			log.Printf("Updating GitTrackObject %s/%s\n", gto.Namespace, gto.Name)
			if err = r.Update(context.TODO(), found); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to update GitTrackObject for '%s': %v", path, err)
			}
			status.ObjectsApplied++
		}
	}

	if status.ObjectsApplied == 0 {
		return reconcile.Result{}, nil
	}

	status.Conditions = []farosv1alpha1.GitTrackCondition{
		success(parseErrorCondition, ""),
		success(gitErrorCondition, ""),
	}

	if !reflect.DeepEqual(instance.Status, status) {
		instance.Status = *(status)
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to update status: %v", err)
		}
	}

	return reconcile.Result{}, nil
}
