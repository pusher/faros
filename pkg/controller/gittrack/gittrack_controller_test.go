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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
var depKey = types.NamespacedName{Name: "foo-deployment", Namespace: "default"}
var mgr manager.Manager
var instance *farosv1alpha1.GitTrack
var requests chan reconcile.Request
var stop chan struct{}

const timeout = time.Second * 5

var _ = Describe("GitTrack Suite", func() {
	BeforeEach(func() {
		// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
		// channel when it is finished.
		var err error
		mgr, err = manager.New(cfg, manager.Options{})
		Expect(err).NotTo(HaveOccurred())
		c = mgr.GetClient()

		var recFn reconcile.Reconciler
		recFn, requests = SetupTestReconcile(newReconciler(mgr))
		Expect(add(mgr, recFn)).NotTo(HaveOccurred())
		stop = StartTestManager(mgr)
	})

	AfterEach(func() {
		close(stop)
	})

	Describe("On reconcile", func() {
		It("create an instance", func() {
			instance = &farosv1alpha1.GitTrack{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}
			// Create the GitTrack object and expect the Reconcile and Deployment to be created
			err := c.Create(context.TODO(), instance)
			Expect(err).NotTo(HaveOccurred())
			defer c.Delete(context.TODO(), instance)
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			deploy := &appsv1.Deployment{}
			Eventually(func() error { return c.Get(context.TODO(), depKey, deploy) }, timeout).
				Should(Succeed())

			// Delete the Deployment and expect Reconcile to be called for Deployment deletion
			Expect(c.Delete(context.TODO(), deploy)).NotTo(HaveOccurred())
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			Eventually(func() error { return c.Get(context.TODO(), depKey, deploy) }, timeout).
				Should(Succeed())

			// Manually delete Deployment since GC isn't enabled in the test control plane
			Expect(c.Delete(context.TODO(), deploy)).To(Succeed())
		})
	})
})
