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
	gittrackutils "github.com/pusher/faros/pkg/controller/gittrack/utils"
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
				Expect(len(conditions)).To(Equal(4))
				parseErrorCondition := conditions[0]
				gitErrorCondition := conditions[1]
				gcErrorCondition := conditions[2]
				upToDateCondiiton := conditions[3]
				Expect(parseErrorCondition.Type).To(Equal(farosv1alpha1.FilesParsedType))
				Expect(gitErrorCondition.Type).To(Equal(farosv1alpha1.FilesFetchedType))
				Expect(gcErrorCondition.Type).To(Equal(farosv1alpha1.ChildrenGarbageCollectedType))
				Expect(upToDateCondiiton.Type).To(Equal(farosv1alpha1.ChildrenUpToDateType))
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

			PIt("adds a label for tracking owned GitTrackObjects", func() {
				deployGto := &farosv1alpha1.GitTrackObject{}
				serviceGto := &farosv1alpha1.GitTrackObject{}
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployGto)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, serviceGto)
				}, timeout).Should(Succeed())
			})
		})

		Context("with multi-document YAML", func() {
			// 61940ab417e30cf81d3bf488b6e03cfe881e9557
			BeforeEach(func() {
				instance = &farosv1alpha1.GitTrack{ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "default"}, Spec: farosv1alpha1.GitTrackSpec{Repository: fmt.Sprintf("file://%s/fixtures", repositoryPath), Reference: "9bf412f0e893c8c1624bb1c523cfeca8243534bc"}}
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

			It("creates GitTrackObjects", func() {
				dsGto, cmGto := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "daemonset-fluentd", Namespace: "default"}, dsGto)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "configmap-fluentd-config", Namespace: "default"}, cmGto)
				}, timeout).Should(Succeed())
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

			It("updates the FilesFetched condition", func() {
				gt := &farosv1alpha1.GitTrack{}
				Eventually(func() error { return c.Get(context.TODO(), gtKey, gt) }, timeout).Should(Succeed())
				// TODO: don't rely on ordering
				c := gt.Status.Conditions[1]
				Expect(c.Type).To(Equal(farosv1alpha1.FilesFetchedType))
				Expect(c.Status).To(Equal(v1.ConditionFalse))
				Expect(c.LastUpdateTime).NotTo(BeNil())
				Expect(c.LastTransitionTime).NotTo(BeNil())
				Expect(c.LastUpdateTime).To(Equal(c.LastTransitionTime))
				Expect(c.Reason).To(Equal(string(gittrackutils.ErrorFetchingFiles)))
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

			It("updates the FilesFetched condition", func() {
				gt := &farosv1alpha1.GitTrack{}
				Eventually(func() error { return c.Get(context.TODO(), gtKey, gt) }, timeout).Should(Succeed())
				// TODO: don't rely on ordering
				c := gt.Status.Conditions[1]
				Expect(c.Type).To(Equal(farosv1alpha1.FilesFetchedType))
				Expect(c.Status).To(Equal(v1.ConditionFalse))
				Expect(c.LastUpdateTime).NotTo(BeNil())
				Expect(c.LastTransitionTime).NotTo(BeNil())
				Expect(c.LastUpdateTime).To(Equal(c.LastTransitionTime))
				Expect(c.Reason).To(Equal(string(gittrackutils.ErrorFetchingFiles)))
				Expect(c.Message).To(Equal("no files for subpath 'does-not-exist'"))
			})
		})
	})

	Context("When a GitTrack resource is updated", func() {
		Context("and resources are added to the repository", func() {
			BeforeEach(func() {
				instance = &farosv1alpha1.GitTrack{ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "default"}, Spec: farosv1alpha1.GitTrackSpec{Repository: fmt.Sprintf("file://%s/fixtures", repositoryPath), Reference: "28928ccaeb314b96293e18cc8889997f0f46b79b"}}
				err := c.Create(context.TODO(), instance)
				Expect(err).NotTo(HaveOccurred())
				// wait for reconcile for creating the GitTrack resource
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// wait for reconcile for creating the GitTrackObject resources
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

			It("creates the new resources", func() {
				before, after := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				Eventually(func() error { return c.Get(context.TODO(), gtKey, instance) }, timeout).Should(Succeed())
				Expect(instance.Spec.Reference).To(Equal("28928ccaeb314b96293e18cc8889997f0f46b79b"))

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "ingress-example", Namespace: "default"}, before)
				}, timeout).ShouldNot(Succeed())

				instance.Spec.Reference = "09d24c51c191b4caacd35cda23bd44c86f16edc6"
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error { return c.Get(context.TODO(), gtKey, instance) }, timeout).Should(Succeed())

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "ingress-example", Namespace: "default"}, after)
				}, timeout).Should(Succeed())
			})
		})

		Context("and resources are removed from the repository", func() {
			BeforeEach(func() {
				instance = &farosv1alpha1.GitTrack{ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: "default"}, Spec: farosv1alpha1.GitTrackSpec{Repository: fmt.Sprintf("file://%s/fixtures", repositoryPath), Reference: "4532b487a5aaf651839f5401371556aa16732a6e"}}
				err := c.Create(context.TODO(), instance)
				Expect(err).NotTo(HaveOccurred())
				// wait for reconcile for creating the GitTrack resource
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// wait for reconcile for creating the GitTrackObject resources
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

			It("deletes the removed resources", func() {
				before, after := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				Eventually(func() error { return c.Get(context.TODO(), gtKey, instance) }, timeout).Should(Succeed())
				Expect(instance.Spec.Reference).To(Equal("4532b487a5aaf651839f5401371556aa16732a6e"))

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "configmap-deleted-config", Namespace: "default"}, before)
				}, timeout).Should(Succeed())

				instance.Spec.Reference = "28928ccaeb314b96293e18cc8889997f0f46b79b"
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error { return c.Get(context.TODO(), gtKey, instance) }, timeout).Should(Succeed())

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "configmap-deleted-config", Namespace: "default"}, after)
				}, timeout).ShouldNot(Succeed())
			})

			It("doesn't delete any other resources", func() {
				Eventually(func() error { return c.Get(context.TODO(), gtKey, instance) }, timeout).Should(Succeed())
				Expect(instance.Spec.Reference).To(Equal("4532b487a5aaf651839f5401371556aa16732a6e"))

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "configmap-deleted-config", Namespace: "default"}, &farosv1alpha1.GitTrackObject{})
				}, timeout).Should(Succeed())

				instance.Spec.Reference = "28928ccaeb314b96293e18cc8889997f0f46b79b"
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error { return c.Get(context.TODO(), gtKey, instance) }, timeout).Should(Succeed())

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "configmap-deleted-config", Namespace: "default"}, &farosv1alpha1.GitTrackObject{})
				}, timeout).ShouldNot(Succeed())

				gtos := &farosv1alpha1.GitTrackObjectList{}
				err = c.List(context.TODO(), client.InNamespace(instance.Namespace), gtos)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(gtos.Items)).To(Equal(2))
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

			It("updates the updated resources", func() {
				before, after := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				Eventually(func() error { return c.Get(context.TODO(), gtKey, instance) }, timeout).Should(Succeed())
				Expect(instance.Spec.Reference).To(Equal("a14443638218c782b84cae56a14f1090ee9e5c9c"))

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, before)
				}, timeout).Should(Succeed())

				instance.Spec.Reference = "448b39a21d285fcb5aa4b718b27a3e13ffc649b3"
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error { return c.Get(context.TODO(), gtKey, instance) }, timeout).Should(Succeed())

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, after)
				}, timeout).Should(Succeed())
				Expect(after.Spec).ToNot(Equal(before.Spec))
			})

			It("doesn't modify any other resources", func() {
				before, after := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				Eventually(func() error { return c.Get(context.TODO(), gtKey, instance) }, timeout).Should(Succeed())
				Expect(instance.Spec.Reference).To(Equal("a14443638218c782b84cae56a14f1090ee9e5c9c"))

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, before)
				}, timeout).Should(Succeed())

				instance.Spec.Reference = "448b39a21d285fcb5aa4b718b27a3e13ffc649b3"
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error { return c.Get(context.TODO(), gtKey, instance) }, timeout).Should(Succeed())

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, after)
				}, timeout).Should(Succeed())
				Expect(after.Spec).To(Equal(before.Spec))
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
