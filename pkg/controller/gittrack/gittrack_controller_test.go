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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	"golang.org/x/net/context"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client
var mgr manager.Manager
var instance *farosv1alpha1.GitTrack
var requests chan reconcile.Request
var stop chan struct{}

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "example", Namespace: "default"}}
var gtKey = types.NamespacedName{Name: "example", Namespace: "default"}

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

	Context("When a GitTrack resource is created", func() {
		Context("with a valid Spec", func() {
			BeforeEach(func() {
				instance = &farosv1alpha1.GitTrack{ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "default"}, Spec: farosv1alpha1.GitTrackSpec{Repository: fmt.Sprintf("file://%s/fixtures", repositoryPath), Reference: "a14443638218c782b84cae56a14f1090ee9e5c9c"}}
				// Create the GitTrack object and expect the Reconcile and Deployment to be created
				err := c.Create(context.TODO(), instance)
				Expect(err).NotTo(HaveOccurred())
				// Wait for reconcile for creating the GitTrack resource
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for creating the GitTrackObject resources
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			})

			AfterEach(func() {
				Eventually(func() error { return c.Get(context.TODO(), gtKey, instance) }, timeout).Should(Succeed())
				err := c.Delete(context.TODO(), instance)
				Expect(err).NotTo(HaveOccurred())
				// GC isn't run in the control-plane so guess we'll have to clean up manually
				gtos := &farosv1alpha1.GitTrackObjectList{}
				err = c.List(context.TODO(), client.InNamespace(instance.Namespace), gtos)
				Expect(err).NotTo(HaveOccurred())
				for _, gto := range gtos.Items {
					err = c.Delete(context.TODO(), &gto)
					Expect(err).NotTo(HaveOccurred())
				}
			})

			It("updates its status", func() {
				gt := &farosv1alpha1.GitTrack{}
				Eventually(func() error { return c.Get(context.TODO(), gtKey, gt) }, timeout).Should(Succeed())
				two, zero := int64(2), int64(0)
				Expect(gt.Status.ObjectsDiscovered).To(Equal(two))
				Expect(gt.Status.ObjectsApplied).To(Equal(two))
				Expect(gt.Status.ObjectsIgnored).To(Equal(zero))
				Expect(gt.Status.ObjectsInSync).To(Equal(zero))
			})

			It("sets the status conditions", func() {
				gt := &farosv1alpha1.GitTrack{}
				Eventually(func() error { return c.Get(context.TODO(), gtKey, gt) }, timeout).Should(Succeed())
				conditions := gt.Status.Conditions
				Expect(len(conditions)).To(Equal(2))
				parseErrorCondition := conditions[0]
				gitErrorCondition := conditions[1]
				Expect(parseErrorCondition.Type).To(Equal(farosv1alpha1.ParseErrorType))
				Expect(gitErrorCondition.Type).To(Equal(farosv1alpha1.GitErrorType))
			})

			It("creates GitTrackObjects", func() {
				deployGto := &farosv1alpha1.GitTrackObject{}
				serviceGto := &farosv1alpha1.GitTrackObject{}
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployGto)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, serviceGto)
				}, timeout).Should(Succeed())
			})

			It("sets ownerReferences for created GitTrackObjects", func() {
				deployGto := &farosv1alpha1.GitTrackObject{}
				serviceGto := &farosv1alpha1.GitTrackObject{}
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployGto)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, serviceGto)
				}, timeout).Should(Succeed())
				Expect(len(deployGto.OwnerReferences)).To(Equal(1))
				Expect(len(serviceGto.OwnerReferences)).To(Equal(1))
			})
		})

		Context("with an invalid Reference", func() {
			BeforeEach(func() {
				instance = &farosv1alpha1.GitTrack{ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "default"}, Spec: farosv1alpha1.GitTrackSpec{Repository: fmt.Sprintf("file://%s/fixtures", repositoryPath), Reference: "does-not-exist"}}
				err := c.Create(context.TODO(), instance)
				Expect(err).NotTo(HaveOccurred())
				// Wait for reconcile for creating the GitTrack resource
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for updating status
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			})

			AfterEach(func() {
				err := c.Delete(context.TODO(), instance)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates the GitError condition", func() {
				gt := &farosv1alpha1.GitTrack{}
				Eventually(func() error { return c.Get(context.TODO(), gtKey, gt) }, timeout).Should(Succeed())
				// TODO: don't rely on ordering
				c := gt.Status.Conditions[1]
				Expect(c.Type).To(Equal(farosv1alpha1.GitErrorType))
				Expect(c.Status).To(Equal(v1.ConditionTrue))
				Expect(c.LastUpdateTime).NotTo(BeNil())
				Expect(c.LastTransitionTime).NotTo(BeNil())
				Expect(c.LastUpdateTime).To(Equal(c.LastTransitionTime))
				Expect(c.Reason).To(Equal("failed to checkout 'does-not-exist': unable to parse ref does-not-exist: reference not found"))
				Expect(c.Message).To(Equal("failed to checkout 'does-not-exist': unable to parse ref does-not-exist: reference not found"))
			})
		})

		Context("with an invalid SubPath", func() {
			BeforeEach(func() {
				instance = &farosv1alpha1.GitTrack{ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "default"}, Spec: farosv1alpha1.GitTrackSpec{Repository: fmt.Sprintf("file://%s/fixtures", repositoryPath), Reference: "master", SubPath: "does-not-exist"}}
				err := c.Create(context.TODO(), instance)
				Expect(err).NotTo(HaveOccurred())
				// Wait for reconcile for creating the GitTrack resource
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for updating status
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			})

			AfterEach(func() {
				err := c.Delete(context.TODO(), instance)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates the GitError condition", func() {
				gt := &farosv1alpha1.GitTrack{}
				Eventually(func() error { return c.Get(context.TODO(), gtKey, gt) }, timeout).Should(Succeed())
				// TODO: don't rely on ordering
				c := gt.Status.Conditions[1]
				Expect(c.Type).To(Equal(farosv1alpha1.GitErrorType))
				Expect(c.Status).To(Equal(v1.ConditionTrue))
				Expect(c.LastUpdateTime).NotTo(BeNil())
				Expect(c.LastTransitionTime).NotTo(BeNil())
				Expect(c.LastUpdateTime).To(Equal(c.LastTransitionTime))
				Expect(c.Reason).To(Equal("no files for subpath 'does-not-exist'"))
				Expect(c.Message).To(Equal("no files for subpath 'does-not-exist'"))
			})
		})
	})

	Context("When a GitTrack resource is updated", func() {
		Context("and resources are added to the repository", func() {
			PIt("creates the new resources", func() {
			})

			PIt("doesn't update any of the existing resources", func() {
			})
		})

		Context("and resources are deleted from the repository", func() {
			PIt("deletes the removed resources", func() {
			})

			PIt("doesn't delete any other resources", func() {
			})
		})

		Context("and resources in the repository are updated", func() {
			BeforeEach(func() {
				instance = &farosv1alpha1.GitTrack{ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "default"}, Spec: farosv1alpha1.GitTrackSpec{Repository: fmt.Sprintf("file://%s/fixtures", repositoryPath), Reference: "a14443638218c782b84cae56a14f1090ee9e5c9c"}}
				err := c.Create(context.TODO(), instance)
				Expect(err).NotTo(HaveOccurred())
				// wait for reconcile for creating the GitTrack resource
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// wait for reconcile for creating the GitTrackObject resources
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// wait for reconcile for updating status
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			})

			AfterEach(func() {
				err := c.Delete(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
			})

			PIt("updates the updated resources", func() {
			})

			PIt("doesn't modify any other resources", func() {
			})
		})

		Context("and the subPath has changed", func() {
		})

		Context("and the reference is invalid", func() {
		})

		Context("and the subPath is invalid", func() {
		})
	})

	Context("When a GitTrack resource is deleted", func() {
	})
})
