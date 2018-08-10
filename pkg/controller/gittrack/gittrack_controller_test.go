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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "example", Namespace: "default"}}
var gtKey = types.NamespacedName{Name: "example", Namespace: "default"}
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

	Describe("When a GitTrack resource is created", func() {
		BeforeEach(func() {
			instance = &farosv1alpha1.GitTrack{ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "default"}, Spec: farosv1alpha1.GitTrackSpec{Repository: "file://" + repositoryPath + "/fixtures", Reference: "master"}}
			// Create the GitTrack object and expect the Reconcile and Deployment to be created
			err := c.Create(context.TODO(), instance)
			Expect(err).NotTo(HaveOccurred())
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
		})

		AfterEach(func() {
			err := c.Delete(context.TODO(), instance)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should update its status", func() {
			gt := &farosv1alpha1.GitTrack{}
			Eventually(func() error { return c.Get(context.TODO(), gtKey, gt) }, timeout).Should(Succeed())
			status := farosv1alpha1.GitTrackStatus{ObjectsDiscovered: 2, ObjectsApplied: 2, ObjectsIgnored: 0, ObjectsInSync: 0}
			Expect(gt.Status).To(Equal(status))
		})

		PIt("should update its conditions", func() {
		})

		PIt("should create GitTrackObjects", func() {
		})
	})
})
