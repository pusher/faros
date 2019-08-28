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
	farosflags "github.com/pusher/faros/pkg/flags"
	farosclient "github.com/pusher/faros/pkg/utils/client"
	testutils "github.com/pusher/faros/test/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ = Describe("Handler Suite", func() {
	var m testutils.Matcher
	var r *ReconcileGitTrack
	var mgr manager.Manager
	var stop chan struct{}

	const timeout = time.Second * 5
	const consistentlyTimeout = time.Second

	var setGitTrackReference = func(gt farosv1alpha1.GitTrackInterface, repo, reference string) {
		spec := gt.GetSpec()
		spec.Repository = repo
		spec.Reference = reference
		gt.SetSpec(spec)
	}

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

		c, err := client.New(mgr.GetConfig(), client.Options{})
		Expect(err).NotTo(HaveOccurred())

		m = testutils.Matcher{Client: c, FarosClient: applier}

		recFn := newReconciler(mgr)
		r = recFn.(*ReconcileGitTrack)

		stop = StartTestManager(mgr)
	})

	AfterEach(func() {
		// Stop Controller and informers before cleaning up
		close(stop)

		// Clean up all resources as GC is disabled in the control plane
		testutils.DeleteAll(cfg, timeout,
			&farosv1alpha1.GitTrackList{},
			&farosv1alpha1.ClusterGitTrackList{},
			&farosv1alpha1.GitTrackObjectList{},
			&farosv1alpha1.ClusterGitTrackObjectList{},
			&corev1.EventList{},
		)
	})

	Context("handleGitTrack", func() {
		var gt farosv1alpha1.GitTrackInterface
		var result handlerResult

		var AssertValidChildren = func() {
			var gto farosv1alpha1.GitTrackObjectInterface

			var AssertChild = func() {
				It("creates a GitTrackObject the child", func() {
					m.Get(gto, timeout).Should(Succeed())
				})

				It("adds an ownerreference to the child", func() {
					m.Eventually(gto, timeout).
						Should(testutils.WithOwnerReferences(ContainElement(testutils.GetGitTrackInterfaceOwnerRef(gt))))
				})

				It("should add a last applied annotation to the child", func() {
					m.Eventually(gto, timeout).
						Should(testutils.WithAnnotations(HaveKey(farosclient.LastAppliedAnnotation)))
				})
			}


			Context("for the deployment file", func() {
				BeforeEach(func() {
					gto = testutils.ExampleGitTrackObject.DeepCopy()
					gto.SetName("deployment-nginx")
				})

				AssertChild()
			})

			Context("for the service file", func() {
				BeforeEach(func() {
					gto = testutils.ExampleGitTrackObject.DeepCopy()
					gto.SetName("service-nginx")
				})

				AssertChild()
			})

			It("should return no errors", func() {
				Expect(result.parseError).ToNot(HaveOccurred())
				Expect(result.gitError).ToNot(HaveOccurred())
				Expect(result.gcError).ToNot(HaveOccurred())
				Expect(result.upToDateError).ToNot(HaveOccurred())
			})
		}

		Context("with a GitTrack", func() {
			BeforeEach(func() {
				gt = testutils.ExampleGitTrack.DeepCopy()
				setGitTrackReference(gt, repositoryURL, "a14443638218c782b84cae56a14f1090ee9e5c9c")

				r = r.withValues(
					"namespace", gt.GetNamespace(),
					"name", gt.GetName(),
				)

				// Create and fetch the instance to make sure caches are synced
				m.Create(gt).Should(Succeed())
				m.Get(gt, timeout).Should(Succeed())
			})

			JustBeforeEach(func() {
				result = r.handleGitTrack(gt)
			})

			Context("with valid children", func() {
				AssertValidChildren()
			})
		})

		Context("with a ClusterGitTrack", func() {
			BeforeEach(func() {
				gt = testutils.ExampleClusterGitTrack.DeepCopy()
				setGitTrackReference(gt, repositoryURL, "a14443638218c782b84cae56a14f1090ee9e5c9c")

				r = r.withValues(
					"namespace", gt.GetNamespace(),
					"name", gt.GetName(),
				)

				// Create and fetch the instance to make sure caches are synced
				m.Create(gt).Should(Succeed())
				m.Get(gt, timeout).Should(Succeed())
			})

			JustBeforeEach(func() {
				result = r.handleGitTrack(gt)
			})

			Context("with valid children", func() {
				AssertValidChildren()
			})
		})
	})
})
