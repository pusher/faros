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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gittrackobjectutils "github.com/pusher/faros/pkg/controller/gittrackobject/utils"
	farosflags "github.com/pusher/faros/pkg/flags"
	farosclient "github.com/pusher/faros/pkg/utils/client"
	testutils "github.com/pusher/faros/test/utils"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ = Describe("Handler Suite", func() {
	var m testutils.Matcher
	var r *ReconcileGitTrackObject
	var mgr manager.Manager

	var stop chan struct{}
	var stopInformers chan struct{}

	const timeout = time.Second * 5
	const consistentlyTimeout = time.Second

	BeforeEach(func() {
		// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
		// channel when it is finished.
		var err error
		cfg.RateLimiter = flowcontrol.NewFakeAlwaysRateLimiter()
		mgr, err = manager.New(cfg, manager.Options{
			Namespace:          farosflags.Namespace,
			MetricsBindAddress: "0", // Disable serving metrics while testing
		})
		Expect(err).NotTo(HaveOccurred())

		applier, err := farosclient.NewApplier(cfg, farosclient.Options{})
		Expect(err).NotTo(HaveOccurred())

		m = testutils.Matcher{Client: mgr.GetClient(), FarosClient: applier}

		recFn := newReconciler(mgr)
		r = recFn.(*ReconcileGitTrackObject)

		stopInformers = r.StopChan()
		stop = StartTestManager(mgr)
	})

	AfterEach(func() {
		// Stop Controller and informers before cleaning up
		close(stop)
		close(stopInformers)
		// Clean up all resources as GC is disabled in the control plane
		testutils.DeleteAll(cfg, timeout,
			&farosv1alpha1.GitTrackObjectList{},
			&farosv1alpha1.ClusterGitTrackObjectList{},
			&appsv1.DeploymentList{},
			&rbacv1.ClusterRoleBindingList{},
		)
	})

	Context("handleGitTrackObject", func() {
		Context("with a GitTrackObject", func() {
			var gto *farosv1alpha1.GitTrackObject
			var child *appsv1.Deployment
			var result handlerResult

			BeforeEach(func() {
				gto = testutils.ExampleGitTrackObject.DeepCopy()
				child = testutils.ExampleDeployment.DeepCopy()
				Expect(testutils.SetGitTrackObjectInterfaceSpec(gto, child)).To(Succeed())

				// Create and fetch the instance to make sure caches are synced
				m.Create(gto).Should(Succeed())
				m.Get(gto, timeout).Should(Succeed())
			})

			Context("when the child does not exist", func() {
				BeforeEach(func() {
					result = r.handleGitTrackObject(gto)
					Expect(result.inSyncError).To(BeNil())
				})

				It("should create the child resource", func() {
					m.Get(child, timeout).Should(Succeed())
				})

				It("should add an owner reference to the child", func() {
					m.Get(child, timeout).Should(Succeed())
					m.Eventually(child, timeout).
						Should(testutils.WithOwnerReferences(ContainElement(testutils.GetGitTrackObjectOwnerRef(gto))))
				})

				It("should add a last applied annotation to the child", func() {
					m.Get(child, timeout).Should(Succeed())
					m.Eventually(child, timeout).
						Should(testutils.WithAnnotations(HaveKey(farosclient.LastAppliedAnnotation)))
				})
			})

			Context("when the child already exists", func() {
				BeforeEach(func() {
					// Create and fetch the instance to make sure caches are synced
					m.Create(child).Should(Succeed())
					m.Get(child, timeout).Should(Succeed())

					result = r.handleGitTrackObject(gto)
					Expect(result.inSyncError).To(BeNil())
				})

				It("should add an owner reference to the child", func() {
					m.Eventually(child, timeout).
						Should(testutils.WithOwnerReferences(ContainElement(testutils.GetGitTrackObjectOwnerRef(gto))))
				})

				It("should add a last applied annotation to the child", func() {
					m.Eventually(child, timeout).
						Should(testutils.WithAnnotations(HaveKey(farosclient.LastAppliedAnnotation)))
				})
			})

			Context("when the child has the update strategy", func() {
				var originalVersion string
				var originalUID types.UID

				BeforeEach(func() {
					child.Spec.Template.SetAnnotations(map[string]string{"updated": "annotations"})
					child.SetOwnerReferences([]metav1.OwnerReference{testutils.GetGitTrackObjectOwnerRef(gto)})
					m.Apply(child, &farosclient.ApplyOptions{}).Should(Succeed())
					m.Get(child, timeout).Should(Succeed())

					originalVersion = child.GetResourceVersion()
					originalUID = child.GetUID()
				})

				Context("update", func() {
					BeforeEach(func() {
						specData := testutils.ExampleDeployment.DeepCopy()
						annotations := map[string]string{"faros.pusher.com/update-strategy": string(gittrackobjectutils.DefaultUpdateStrategy)}
						specData.SetAnnotations(annotations)
						Expect(testutils.SetGitTrackObjectInterfaceSpec(gto, specData)).To(Succeed())

						m.Update(gto, timeout).Should(Succeed())
						result = r.handleGitTrackObject(gto)
						Expect(result.inSyncError).To(BeNil())
					})

					It("should update the child", func() {
						m.Eventually(child, timeout).Should(testutils.WithResourceVersion(Not(Equal(originalVersion))))
					})

					It("should not replace the child", func() {
						m.Consistently(child, consistentlyTimeout).Should(testutils.WithUID(Equal(originalUID)))
					})
				})

				Context("never", func() {
					BeforeEach(func() {
						specData := testutils.ExampleDeployment.DeepCopy()
						annotations := map[string]string{"faros.pusher.com/update-strategy": string(gittrackobjectutils.NeverUpdateStrategy)}
						specData.SetAnnotations(annotations)
						Expect(testutils.SetGitTrackObjectInterfaceSpec(gto, specData)).To(Succeed())

						m.Update(gto, timeout).Should(Succeed())
						result = r.handleGitTrackObject(gto)
						Expect(result.inSyncError).To(BeNil())
					})

					It("should not update the child", func() {
						m.Consistently(child, consistentlyTimeout).Should(testutils.WithPodTemplateAnnotations(HaveKeyWithValue("updated", "annotations")))
					})

					It("should not replace the child", func() {
						m.Consistently(child, consistentlyTimeout).Should(testutils.WithUID(Equal(originalUID)))
					})
				})

				Context("recreate", func() {
					Context("without conflicts", func() {
						BeforeEach(func() {
							specData := testutils.ExampleDeployment.DeepCopy()
							annotations := map[string]string{"faros.pusher.com/update-strategy": string(gittrackobjectutils.RecreateUpdateStrategy)}
							specData.SetAnnotations(annotations)
							Expect(testutils.SetGitTrackObjectInterfaceSpec(gto, specData)).To(Succeed())

							m.Update(gto, timeout).Should(Succeed())
							result = r.handleGitTrackObject(gto)
							Expect(result.inSyncError).To(BeNil())
						})

						It("should update the child", func() {
							m.Eventually(child, timeout).Should(testutils.WithResourceVersion(Not(Equal(originalVersion))))
						})

						It("should not replace the child", func() {
							m.Consistently(child, consistentlyTimeout).Should(testutils.WithUID(Equal(originalUID)))
						})
					})
				})
			})
		})

		Context("with ClusterGitTrackObject", func() {
			var gto *farosv1alpha1.ClusterGitTrackObject
			var child *rbacv1.ClusterRoleBinding
			var result handlerResult

			BeforeEach(func() {
				gto = testutils.ExampleClusterGitTrackObject.DeepCopy()
				child = testutils.ExampleClusterRoleBinding.DeepCopy()
				Expect(testutils.SetGitTrackObjectInterfaceSpec(gto, child)).To(Succeed())

				// Create and fetch the instance to make sure caches are synced
				m.Create(gto).Should(Succeed())
				m.Get(gto, timeout).Should(Succeed())
			})

			Context("when the child does not exist", func() {
				BeforeEach(func() {
					result = r.handleGitTrackObject(gto)
					Expect(result.inSyncError).To(BeNil())
				})

				It("should create the child resource", func() {
					m.Get(child, timeout).Should(Succeed())
				})

				It("should add an owner reference to the child", func() {
					m.Get(child, timeout).Should(Succeed())
					m.Eventually(child, timeout).
						Should(testutils.WithOwnerReferences(ContainElement(testutils.GetClusterGitTrackObjectOwnerRef(gto))))
				})

				It("should add a last applied annotation to the child", func() {
					m.Get(child, timeout).Should(Succeed())
					m.Eventually(child, timeout).
						Should(testutils.WithAnnotations(HaveKey(farosclient.LastAppliedAnnotation)))
				})
			})

			Context("when the child already exists", func() {
				BeforeEach(func() {
					// Create and fetch the instance to make sure caches are synced
					m.Create(child).Should(Succeed())
					m.Get(child, timeout).Should(Succeed())

					result = r.handleGitTrackObject(gto)
					Expect(result.inSyncError).To(BeNil())
				})

				It("should add an owner reference to the child", func() {
					m.Eventually(child, timeout).
						Should(testutils.WithOwnerReferences(ContainElement(testutils.GetClusterGitTrackObjectOwnerRef(gto))))
				})

				It("should add a last applied annotation to the child", func() {
					m.Eventually(child, timeout).
						Should(testutils.WithAnnotations(HaveKey(farosclient.LastAppliedAnnotation)))
				})
			})

			Context("when the child has the update strategy", func() {
				var originalVersion string
				var originalUID types.UID

				BeforeEach(func() {
					child.Subjects = []rbacv1.Subject{}
					child.SetOwnerReferences([]metav1.OwnerReference{testutils.GetClusterGitTrackObjectOwnerRef(gto)})
					m.Apply(child, &farosclient.ApplyOptions{}).Should(Succeed())
					m.Eventually(child, timeout).Should(testutils.WithSubjects(BeEmpty()))

					originalVersion = child.GetResourceVersion()
					originalUID = child.GetUID()
				})

				Context("update", func() {
					BeforeEach(func() {
						specData := testutils.ExampleClusterRoleBinding.DeepCopy()
						annotations := map[string]string{"faros.pusher.com/update-strategy": string(gittrackobjectutils.DefaultUpdateStrategy)}
						specData.SetAnnotations(annotations)
						Expect(testutils.SetGitTrackObjectInterfaceSpec(gto, specData)).To(Succeed())

						m.Update(gto, timeout).Should(Succeed())
						result = r.handleGitTrackObject(gto)
						Expect(result.inSyncError).To(BeNil())
					})

					It("should update the child", func() {
						m.Eventually(child, timeout).Should(testutils.WithResourceVersion(Not(Equal(originalVersion))))
					})

					It("should not replace the child", func() {
						m.Consistently(child, consistentlyTimeout).Should(testutils.WithUID(Equal(originalUID)))
					})
				})

				Context("never", func() {
					BeforeEach(func() {
						specData := testutils.ExampleClusterRoleBinding.DeepCopy()
						annotations := map[string]string{"faros.pusher.com/update-strategy": string(gittrackobjectutils.NeverUpdateStrategy)}
						specData.SetAnnotations(annotations)
						Expect(testutils.SetGitTrackObjectInterfaceSpec(gto, specData)).To(Succeed())

						m.Update(gto, timeout).Should(Succeed())
						result = r.handleGitTrackObject(gto)
						Expect(result.inSyncError).To(BeNil())
					})

					It("should not update the child", func() {
						m.Consistently(child, consistentlyTimeout).Should(testutils.WithSubjects(BeEmpty()))
					})

					It("should not replace the child", func() {
						m.Consistently(child, consistentlyTimeout).Should(testutils.WithUID(Equal(originalUID)))
					})
				})

				Context("recreate", func() {
					Context("with conflicts", func() {
						BeforeEach(func() {
							specData := testutils.ExampleClusterRoleBinding.DeepCopy()
							// Create a conflict (this field is immutable)
							specData.RoleRef.Name = "changed"
							annotations := map[string]string{"faros.pusher.com/update-strategy": string(gittrackobjectutils.RecreateUpdateStrategy)}
							specData.SetAnnotations(annotations)
							Expect(testutils.SetGitTrackObjectInterfaceSpec(gto, specData)).To(Succeed())

							go func() {
								defer GinkgoRecover()
								// We are expecting a delete but we have no GC so have to do it manually
								m.Eventually(child, timeout).Should(testutils.WithFinalizers(ContainElement("foregroundDeletion")))
								child.SetFinalizers([]string{})
								m.Update(child).Should(Succeed())
								m.Get(child, timeout).ShouldNot(Succeed())
							}()

							m.Update(gto, timeout).Should(Succeed())
							result = r.handleGitTrackObject(gto)
							Expect(result.inSyncError).To(BeNil())
						})

						It("should replace the child", func() {
							m.Eventually(child, timeout).Should(testutils.WithUID(Not(Equal(originalUID))))
						})
					})
				})
			})
		})

	})
})
