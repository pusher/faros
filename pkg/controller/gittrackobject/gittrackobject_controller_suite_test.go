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
	"log"
	"path/filepath"
	"testing"

	"github.com/kubernetes-sigs/kubebuilder/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pusher/faros/pkg/apis"
	farosflags "github.com/pusher/faros/pkg/flags"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var cfg *rest.Config

func TestGitTrackObjectController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "GitTrackObject Suite", []Reporter{test.NewlineReporter{}})
}

var t *envtest.Environment

var _ = BeforeSuite(func() {
	t = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "config", "crds")},
	}
	apis.AddToScheme(scheme.Scheme)

	farosflags.Namespace = "default"

	var err error
	if cfg, err = t.Start(); err != nil {
		log.Fatal(err)
	}
})

var _ = AfterSuite(func() {
	t.Stop()
})

type testReconciler struct {
	*ReconcileGitTrackObject
	requests chan reconcile.Request
}

func (t *testReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	result, err := t.ReconcileGitTrackObject.Reconcile(req)
	t.requests <- req
	return result, err
}

// SetupTestReconcile returns a reconcile.Reconcile implementation that delegates to inner and
// writes the request to requests after Reconcile is finished.
func SetupTestReconcile(inner reconcile.Reconciler) (reconcile.Reconciler, chan reconcile.Request) {
	requests := make(chan reconcile.Request)
	reconciler := inner.(*ReconcileGitTrackObject)
	return &testReconciler{
		ReconcileGitTrackObject: reconciler,
		requests:                requests,
	}, requests
}

// StartTestManager adds recFn
func StartTestManager(mgr manager.Manager) chan struct{} {
	stop := make(chan struct{})
	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(stop)).NotTo(HaveOccurred())
	}()
	return stop
}

// testEventRecorder is used to inspect the input to the Reconciler's event
// recorder during tests
type testEventRecorder struct {
	record.EventRecorder
	events chan TestEvent
}

// Eventf implements the record.EventRecorder interface
// Sends TestEvents to the testEventRecorder's events channel for tests to
// read from
func (t *testEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	obj, ok := object.(v1.Object)
	Expect(ok).To(BeTrue())

	// Not every test will listen for events so send them in a go routine so
	// we don't block
	go func() {
		t.events <- TestEvent{Namespace: obj.GetNamespace()}
	}()

	t.EventRecorder.Eventf(object, eventtype, reason, messageFmt, args...)
}

// TestEvent holds information about the EventRecorder input from the reconciler
type TestEvent struct {
	Namespace string
}

// SetupTestEventRecorder injects the testEventRecorder into the reconciler
func SetupTestEventRecorder(inner reconcile.Reconciler) (reconcile.Reconciler, chan TestEvent) {
	events := make(chan TestEvent)
	reconciler := inner.(*ReconcileGitTrackObject)
	reconciler.recorder = &testEventRecorder{
		EventRecorder: reconciler.recorder,
		events:        events,
	}
	return reconciler, events
}
