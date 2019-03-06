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

package test

import (
	"context"

	"github.com/onsi/gomega"
	gtypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Matcher has Gomega Matchers that use the controller-runtime client
type Matcher struct {
	Client client.Client
}

// Object is the combination of two interfaces as a helper for passing
// Kubernetes objects between methods
type Object interface {
	runtime.Object
	metav1.Object
	schema.ObjectKind
}

// Create creates the object on the API server
func (m *Matcher) Create(obj Object, extras ...interface{}) gomega.GomegaAssertion {
	err := m.Client.Create(context.TODO(), obj)
	return gomega.Expect(err, extras)
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

// WithKind returns the objects TypeMeta
func WithKind(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(obj Object) string {
		_, kind := obj.GroupVersionKind().ToAPIVersionAndKind()
		return kind
	}, matcher)
}

// WithAPIVersion returns the objects TypeMeta
func WithAPIVersion(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(obj Object) string {
		apiVersion, _ := obj.GroupVersionKind().ToAPIVersionAndKind()
		return apiVersion
	}, matcher)
}

// WithResourceVersion returns the object's ResourceVersion
func WithResourceVersion(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(obj Object) string {
		return obj.GetResourceVersion()
	}, matcher)
}

// WithCreationTimestamp returns the object's CreationTimestamp
func WithCreationTimestamp(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(obj Object) metav1.Time {
		return obj.GetCreationTimestamp()
	}, matcher)
}

// WithSelfLink returns the object's SelfLink
func WithSelfLink(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(obj Object) string {
		return obj.GetSelfLink()
	}, matcher)
}

// WithUID returns the object's UID
func WithUID(matcher gtypes.GomegaMatcher) gtypes.GomegaMatcher {
	return gomega.WithTransform(func(obj Object) types.UID {
		return obj.GetUID()
	}, matcher)
}

// WithContainers returns the deployments Containers
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
