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

var key = types.NamespacedName{Name: "example", Namespace: "default"}
var expectedRequest = reconcile.Request{NamespacedName: key}
var createInstance = func(gt *farosv1alpha1.GitTrack, ref string) {
	gt.Spec.Reference = ref
	err := c.Create(context.TODO(), gt)
	Expect(err).NotTo(HaveOccurred())
}

var gcInstance = func(key types.NamespacedName) {
	gt := &farosv1alpha1.GitTrack{}
	Eventually(func() error { return c.Get(context.TODO(), key, gt) }, timeout).Should(Succeed())
	err := c.Delete(context.TODO(), gt)
	Expect(err).NotTo(HaveOccurred())
	// GC isn't run in the control-plane so guess we'll have to clean up manually
	gtos := &farosv1alpha1.GitTrackObjectList{}
	err = c.List(context.TODO(), client.InNamespace(gt.Namespace), gtos)
	Expect(err).NotTo(HaveOccurred())
	for _, gto := range gtos.Items {
		err = c.Delete(context.TODO(), &gto)
		Expect(err).NotTo(HaveOccurred())
	}
}

var waitForInstanceCreated = func(key types.NamespacedName) {
	request := reconcile.Request{NamespacedName: key}
	// wait for reconcile for creating the GitTrack resource
	Eventually(requests, timeout).Should(Receive(Equal(request)))
	// wait for reconcile for updating the GitTrack resource's status
	Eventually(requests, timeout).Should(Receive(Equal(request)))
	obj := &farosv1alpha1.GitTrack{}
	Eventually(func() error {
		err := c.Get(context.TODO(), key, obj)
		if err != nil {
			return err
		}
		if len(obj.Status.Conditions) == 0 {
			return fmt.Errorf("Status not updated")
		}
		return nil
	}, timeout).Should(Succeed())
}

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
		instance = &farosv1alpha1.GitTrack{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "default",
			},
			Spec: farosv1alpha1.GitTrackSpec{Repository: repositoryURL}}
	})

	AfterEach(func() {
		gcInstance(key)
		close(stop)
	})

	Context("When a GitTrack resource is created", func() {
		Context("with a valid Spec", func() {
			BeforeEach(func() {
				createInstance(instance, "a14443638218c782b84cae56a14f1090ee9e5c9c")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("updates its status", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				two, zero := int64(2), int64(0)
				Expect(instance.Status.ObjectsDiscovered).To(Equal(two))
				Expect(instance.Status.ObjectsApplied).To(Equal(two))
				Expect(instance.Status.ObjectsIgnored).To(Equal(zero))
				Expect(instance.Status.ObjectsInSync).To(Equal(zero))
			})

			It("sets the status conditions", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				conditions := instance.Status.Conditions
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
			BeforeEach(func() {
				createInstance(instance, "9bf412f0e893c8c1624bb1c523cfeca8243534bc")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
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

		Context("with a cluster scoped resource", func() {
			BeforeEach(func() {
				createInstance(instance, "b17c0e0f45beca3f1c1e62a7f49fecb738c60d42")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("creates ClusterGitTrackObject", func() {
				nsCGto := &farosv1alpha1.ClusterGitTrackObject{}
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "namespace-test", Namespace: ""}, nsCGto)
				}, timeout).Should(Succeed())
			})
		})

		Context("with an invalid Reference", func() {
			BeforeEach(func() {
				createInstance(instance, "does-not-exist")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("updates the FilesFetched condition", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				// TODO: don't rely on ordering
				c := instance.Status.Conditions[1]
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
				instance.Spec.SubPath = "does-not-exist"
				createInstance(instance, "master")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("updates the FilesFetched condition", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				// TODO: don't rely on ordering
				c := instance.Status.Conditions[1]
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
				createInstance(instance, "28928ccaeb314b96293e18cc8889997f0f46b79b")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("creates the new resources", func() {
				before, after := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
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
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "ingress-example", Namespace: "default"}, after)
				}, timeout).Should(Succeed())
			})
		})

		Context("and resources are removed from the repository", func() {
			BeforeEach(func() {
				createInstance(instance, "4532b487a5aaf651839f5401371556aa16732a6e")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("deletes the removed resources", func() {
				before, after := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
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
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "configmap-deleted-config", Namespace: "default"}, after)
				}, timeout).ShouldNot(Succeed())
			})

			It("doesn't delete any other resources", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
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
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())

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
				createInstance(instance, "a14443638218c782b84cae56a14f1090ee9e5c9c")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("updates the updated resources", func() {
				before, after := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
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
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, after)
				}, timeout).Should(Succeed())
				Expect(after.Spec).ToNot(Equal(before.Spec))
			})

			It("doesn't modify any other resources", func() {
				before, after := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
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
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, after)
				}, timeout).Should(Succeed())
				Expect(after.Spec).To(Equal(before.Spec))
			})
		})

		Context("and the subPath has changed", func() {
		})

		Context("and the reference is invalid", func() {
			BeforeEach(func() {
				createInstance(instance, "a14443638218c782b84cae56a14f1090ee9e5c9c")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("does not update any of the resources", func() {
				deployBefore, deployAfter := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				serviceBefore, serviceAfter := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployBefore)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, serviceBefore)
				}, timeout).Should(Succeed())
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())

				instance.Spec.Reference = "does-not-exist"
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployAfter)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, serviceAfter)
				}, timeout).Should(Succeed())

				Expect(deployBefore).To(Equal(deployAfter))
				Expect(serviceBefore).To(Equal(serviceAfter))
			})

			It("updates the FilesFetched condition", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				instance.Spec.Reference = "does-not-exist"
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				// TODO: don't rely on ordering
				c := instance.Status.Conditions[1]
				Expect(c.Type).To(Equal(farosv1alpha1.FilesFetchedType))
				Expect(c.Status).To(Equal(v1.ConditionFalse))
				Expect(c.LastUpdateTime).NotTo(BeNil())
				Expect(c.LastTransitionTime).NotTo(BeNil())
				Expect(c.LastUpdateTime).To(Equal(c.LastTransitionTime))
				Expect(c.Reason).To(Equal(string(gittrackutils.ErrorFetchingFiles)))
				Expect(c.Message).To(Equal("failed to checkout 'does-not-exist': unable to parse ref does-not-exist: reference not found"))
			})
		})

		Context("and the subPath is invalid", func() {
			BeforeEach(func() {
				createInstance(instance, "a14443638218c782b84cae56a14f1090ee9e5c9c")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("does not update any of the resources", func() {
				deployBefore, deployAfter := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				serviceBefore, serviceAfter := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployBefore)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, serviceBefore)
				}, timeout).Should(Succeed())
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())

				instance.Spec.SubPath = "does-not-exist"
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployAfter)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, serviceAfter)
				}, timeout).Should(Succeed())

				Expect(deployBefore).To(Equal(deployAfter))
				Expect(serviceBefore).To(Equal(serviceAfter))
			})

			It("updates the FilesFetched condition", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				instance.Spec.SubPath = "does-not-exist"
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				// TODO: don't rely on ordering
				c := instance.Status.Conditions[1]
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

	Context("When a GitTrack resource is deleted", func() {
	})
})
