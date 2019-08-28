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

package utils

import (
	"context"

	"github.com/onsi/gomega"
	gtypes "github.com/onsi/gomega/types"
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	farosclient "github.com/pusher/faros/pkg/utils/client"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// Matcher has Gomega Matchers that use the controller-runtime client
type Matcher struct {
	Client      client.Client
	FarosClient farosclient.Client
}

// Object is the combination of two interfaces as a helper for passing
// Kubernetes objects between methods
type Object interface {
	runtime.Object
	metav1.Object
}

// UpdateFunc modifies the object fetched from the API server before sending
// the update
type UpdateFunc func(Object) Object

// Apply creates or updates the object on the API server
func (m *Matcher) Apply(obj Object, opts *farosclient.ApplyOptions, extras ...interface{}) gomega.GomegaAssertion {
	err := m.FarosClient.Apply(context.TODO(), opts, obj)
	return gomega.Expect(err, extras)
}

// Create creates the object on the API server
func (m *Matcher) Create(obj Object, extras ...interface{}) gomega.GomegaAssertion {
	err := m.Client.Create(context.TODO(), obj)
	return gomega.Expect(err, extras)
}

// Delete deletes the object from the API server
func (m *Matcher) Delete(obj Object, extras ...interface{}) gomega.GomegaAssertion {
	err := m.Client.Delete(context.TODO(), obj)
	return gomega.Expect(err, extras)
}

// Update udpates the object on the API server
func (m *Matcher) Update(obj Object, intervals ...interface{}) gomega.GomegaAsyncAssertion {
	update := func() error {
		return m.Client.Update(context.TODO(), obj)
	}
	return gomega.Eventually(update, intervals...)
}

// UpdateWithFunc udpates the object on the API server by fetching the object
// and applying a mutating UpdateFunc before sending the update
func (m *Matcher) UpdateWithFunc(obj Object, fn UpdateFunc, intervals ...interface{}) gomega.GomegaAsyncAssertion {
	key := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
	update := func() error {
		err := m.Client.Get(context.TODO(), key, obj)
		if err != nil {
			return err
		}
		return m.Client.Update(context.TODO(), fn(obj))
	}
	return gomega.Eventually(update, intervals...)
}

// Get gets the object from the API server
func (m *Matcher) Get(obj Object, intervals ...interface{}) gomega.GomegaAsyncAssertion {
	key := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
	get := func() error {
		return m.Client.Get(context.TODO(), key, obj)
	}
	return gomega.Eventually(get, intervals...)
}

// Consistently continually gets the object from the API for comparison
func (m *Matcher) Consistently(obj Object, intervals ...interface{}) gomega.GomegaAsyncAssertion {
	return m.consistentlyObject(obj, intervals...)
}

// consistentlyObject gets an individual object from the API server
func (m *Matcher) consistentlyObject(obj Object, intervals ...interface{}) gomega.GomegaAsyncAssertion {
	key := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
	get := func() Object {
		err := m.Client.Get(context.TODO(), key, obj)
		if err != nil {
			panic(err)
		}
		return obj
	}
	return gomega.Consistently(get, intervals...)
}

// Eventually continually gets the object from the API for comparison
func (m *Matcher) Eventually(obj runtime.Object, intervals ...interface{}) gomega.GomegaAsyncAssertion {
	// If the object is a list, return a list
	if meta.IsListType(obj) {
		return m.eventuallyList(obj, intervals...)
	}
	if o, ok := obj.(Object); ok {
		return m.eventuallyObject(o, intervals...)
	}
	//Should not get here
	panic("Unknown object.")
}

// eventuallyObject gets an individual object from the API server
func (m *Matcher) eventuallyObject(obj Object, intervals ...interface{}) gomega.GomegaAsyncAssertion {
	key := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
	get := func() Object {
		err := m.Client.Get(context.TODO(), key, obj)
		if err != nil {
			panic(err)
		}
		return obj
	}
	return gomega.Eventually(get, intervals...)
}

// eventuallyList gets a list type  from the API server
func (m *Matcher) eventuallyList(obj runtime.Object, intervals ...interface{}) gomega.GomegaAsyncAssertion {
	list := func() runtime.Object {
		err := m.Client.List(context.TODO(), obj)
		if err != nil {
			panic(err)
		}
		return obj
	}
	return gomega.Eventually(list, intervals...)
}

// WithAnnotations returns the object's annotations
func WithAnnotations(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(obj Object) map[string]string {
		return obj.GetAnnotations()
	}, matcher)
}

// WithPodTemplateAnnotations returns the object's annotations
func WithPodTemplateAnnotations(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(obj *appsv1.Deployment) map[string]string {
		return obj.Spec.Template.GetAnnotations()
	}, matcher)
}

// WithPodTemplateLabels returns the object's labels
func WithPodTemplateLabels(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(obj *appsv1.Deployment) map[string]string {
		return obj.Spec.Template.GetLabels()
	}, matcher)
}

// WithFinalizers returns the object's annotations
func WithFinalizers(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(obj Object) []string {
		return obj.GetFinalizers()
	}, matcher)
}

// WithNamespaceFinalizers returns the namespace's finazliers
func WithNamespaceFinalizers(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(ns *corev1.Namespace) []corev1.FinalizerName {
		return ns.Spec.Finalizers
	}, matcher)
}

// WithOwnerReferences returns the object's owner references
func WithOwnerReferences(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(obj Object) []metav1.OwnerReference {
		return obj.GetOwnerReferences()
	}, matcher)
}

// WithResourceVersion returns the object's resource version
func WithResourceVersion(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(obj Object) string {
		return obj.GetResourceVersion()
	}, matcher)
}

// WithUID returns the object's UID
func WithUID(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(obj Object) types.UID {
		return obj.GetUID()
	}, matcher)
}

// WithUnstructuredObject returns the object's inner object
func WithUnstructuredObject(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(ev event.GenericEvent) unstructured.Unstructured {
		u, ok := ev.Object.(*unstructured.Unstructured)
		if !ok {
			panic("Non unstructured object")
		}
		return *u
	}, matcher)
}

// WithGitTrackObjectStatusConditions returns the GitTrackObject's status conditions
func WithGitTrackObjectStatusConditions(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(gto farosv1alpha1.GitTrackObjectInterface) []farosv1alpha1.GitTrackObjectCondition {
		return gto.GetStatus().Conditions
	}, matcher)
}

// WithGitTrackObjectConditionType returns the GitTrackObjectCondition's type
func WithGitTrackObjectConditionType(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(c farosv1alpha1.GitTrackObjectCondition) farosv1alpha1.GitTrackObjectConditionType {
		return c.Type
	}, matcher)
}

// WithGitTrackObjectConditionStatus returns the GitTrackObjectCondition's status
func WithGitTrackObjectConditionStatus(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(c farosv1alpha1.GitTrackObjectCondition) corev1.ConditionStatus {
		return c.Status
	}, matcher)
}

// WithGitTrackObjectConditionReason returns the GitTrackObjectCondition's reason
func WithGitTrackObjectConditionReason(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(c farosv1alpha1.GitTrackObjectCondition) string {
		return c.Reason
	}, matcher)
}

// WithGitTrackObjectConditionMessage returns the GitTrackObjectCondition's message
func WithGitTrackObjectConditionMessage(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(c farosv1alpha1.GitTrackObjectCondition) string {
		return c.Message
	}, matcher)
}

// WithItems returns the items of the list
func WithItems(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(obj runtime.Object) []runtime.Object {
		items, err := meta.ExtractList(obj)
		if err != nil {
			panic(err)
		}
		return items
	}, matcher)
}

// WithSubjects returns the ClusterRoleBinding's subjects
func WithSubjects(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(crb *rbacv1.ClusterRoleBinding) []rbacv1.Subject {
		return crb.Subjects
	}, matcher)
}

// WithInvolvedObjectKind returns the event's InvolvedObject's kind
func WithInvolvedObjectKind(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(ev *corev1.Event) string {
		return ev.InvolvedObject.Kind
	}, matcher)
}

// WithInvolvedObjectName returns the event's InvolvedObject's name
func WithInvolvedObjectName(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(ev *corev1.Event) string {
		return ev.InvolvedObject.Name
	}, matcher)
}

// WithInvolvedObjectNamespace returns the event's InvolvedObject's namespace
func WithInvolvedObjectNamespace(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(ev *corev1.Event) string {
		return ev.InvolvedObject.Namespace
	}, matcher)
}

// WithReason returns the event's reason
func WithReason(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(ev *corev1.Event) string {
		return ev.Reason
	}, matcher)
}

// WithEventType returns the event's type
func WithEventType(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(ev *corev1.Event) string {
		return ev.Type
	}, matcher)
}

// WithContainers returns the deployment's containers
func WithContainers(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(dep *appsv1.Deployment) []corev1.Container {
		return dep.Spec.Template.Spec.Containers
	}, matcher)
}

// WithImage returns the container's image
func WithImage(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(c corev1.Container) string {
		return c.Image
	}, matcher)
}
