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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gittrackobjectutils "github.com/pusher/faros/pkg/controller/gittrackobject/utils"
	farosflags "github.com/pusher/faros/pkg/flags"
	testutils "github.com/pusher/faros/test/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ = Describe("Status Suite", func() {
	var m testutils.Matcher
	var r *ReconcileGitTrackObject
	var mgr manager.Manager

	var stop chan struct{}
	var stopInformers chan struct{}

	const timeout = time.Second * 5

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

		m = testutils.Matcher{Client: mgr.GetClient()}

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
			&farosv1alpha1.GitTrackList{},
			&farosv1alpha1.GitTrackObjectList{},
			&farosv1alpha1.ClusterGitTrackObjectList{},
			&appsv1.DeploymentList{},
			&rbacv1.ClusterRoleBindingList{},
			&corev1.EventList{},
		)
	})

	Context("updateStatus", func() {
		var opts *statusOpts

		BeforeEach(func() {
			opts = &statusOpts{}
		})

		Context("with a GitTrackObject", func() {
			var gto *farosv1alpha1.GitTrackObject

			BeforeEach(func() {
				gto = testutils.ExampleGitTrackObject.DeepCopy()

				r = r.withValues(
					"Namespace", gto.GetNamespace(),
					"ChildName", gto.GetSpec().Name,
					"ChildKind", gto.GetSpec().Kind,
				)

				m.Create(gto).Should(Succeed())
				m.Get(gto, timeout).Should(Succeed())
			})

			Context("with no inSync Error", func() {
				BeforeEach(func() {
					r.updateStatus(gto, opts)
				})

				It("should set the inSync condition", func() {
					m.Eventually(gto).Should(
						testutils.WithGitTrackObjectStatusConditions(
							ContainElement(
								SatisfyAll(
									testutils.WithGitTrackObjectConditionType(Equal(farosv1alpha1.ObjectInSyncType)),
									testutils.WithGitTrackObjectConditionStatus(Equal(corev1.ConditionTrue)),
									testutils.WithGitTrackObjectConditionReason(Equal(string(gittrackobjectutils.ChildAppliedSuccess))),
									testutils.WithGitTrackObjectConditionMessage(Equal("")),
								),
							),
						),
					)
				})
			})

			Context("with an inSync Error", func() {
				BeforeEach(func() {
					opts.inSyncReason = gittrackobjectutils.ErrorCreatingChild
					opts.inSyncError = fmt.Errorf("New test error")
					r.updateStatus(gto, opts)
				})

				It("should set the inSync condition", func() {
					m.Eventually(gto).Should(
						testutils.WithGitTrackObjectStatusConditions(
							ContainElement(
								SatisfyAll(
									testutils.WithGitTrackObjectConditionType(Equal(farosv1alpha1.ObjectInSyncType)),
									testutils.WithGitTrackObjectConditionStatus(Equal(corev1.ConditionFalse)),
									testutils.WithGitTrackObjectConditionReason(Equal(string(gittrackobjectutils.ErrorCreatingChild))),
									testutils.WithGitTrackObjectConditionMessage(Equal(opts.inSyncError.Error())),
								),
							),
						),
					)
				})
			})
		})

		Context("with a ClusterGitTrackObject", func() {
			var gto *farosv1alpha1.ClusterGitTrackObject

			BeforeEach(func() {
				gto = testutils.ExampleClusterGitTrackObject.DeepCopy()

				r = r.withValues(
					"Namespace", gto.GetNamespace(),
					"ChildName", gto.GetSpec().Name,
					"ChildKind", gto.GetSpec().Kind,
				)

				m.Create(gto).Should(Succeed())
				m.Get(gto, timeout).Should(Succeed())
			})

			Context("with no inSync Error", func() {
				BeforeEach(func() {
					r.updateStatus(gto, opts)
				})

				It("should set the inSync condition", func() {
					m.Eventually(gto).Should(
						testutils.WithGitTrackObjectStatusConditions(
							ContainElement(
								SatisfyAll(
									testutils.WithGitTrackObjectConditionType(Equal(farosv1alpha1.ObjectInSyncType)),
									testutils.WithGitTrackObjectConditionStatus(Equal(corev1.ConditionTrue)),
									testutils.WithGitTrackObjectConditionReason(Equal(string(gittrackobjectutils.ChildAppliedSuccess))),
									testutils.WithGitTrackObjectConditionMessage(Equal("")),
								),
							),
						),
					)
				})
			})

			Context("with an inSync Error", func() {
				BeforeEach(func() {
					opts.inSyncReason = gittrackobjectutils.ErrorCreatingChild
					opts.inSyncError = fmt.Errorf("New test error")
					r.updateStatus(gto, opts)
				})

				It("should set the inSync condition", func() {
					m.Eventually(gto).Should(
						testutils.WithGitTrackObjectStatusConditions(
							ContainElement(
								SatisfyAll(
									testutils.WithGitTrackObjectConditionType(Equal(farosv1alpha1.ObjectInSyncType)),
									testutils.WithGitTrackObjectConditionStatus(Equal(corev1.ConditionFalse)),
									testutils.WithGitTrackObjectConditionReason(Equal(string(gittrackobjectutils.ErrorCreatingChild))),
									testutils.WithGitTrackObjectConditionMessage(Equal(opts.inSyncError.Error())),
								),
							),
						),
					)
				})
			})
		})
	})
})
