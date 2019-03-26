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
	farosflags "github.com/pusher/faros/pkg/flags"
	testutils "github.com/pusher/faros/test/utils"
	appsv1 "k8s.io/api/apps/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ = Describe("Watch Suite", func() {
	var c client.Client
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
		c = mgr.GetClient()
		m = testutils.Matcher{Client: c}

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
			&appsv1.DeploymentList{},
		)
	})

	Context("watch", func() {
		var u unstructured.Unstructured
		BeforeEach(func() {
			// Create unstructured Deployment
			content, err := runtime.NewTestUnstructuredConverter(apiequality.Semantic).ToUnstructured(testutils.ExampleDeployment.DeepCopy())
			Expect(err).NotTo(HaveOccurred())
			u.SetUnstructuredContent(content)

			m.Create(&u).Should(Succeed())
			m.Get(&u, timeout).Should(Succeed())

			// Call watch with the unstructued deployment
			Expect(r.watch(u)).NotTo(HaveOccurred())
		})

		It("should create an informer", func() {
			key := fmt.Sprintf("%s:%s", u.GetNamespace(), u.GroupVersionKind().String())
			Expect(r.informers).To(HaveKey(key))
		})

		Context("when a watched kind is modified", func() {
			BeforeEach(func() {
				u.SetAnnotations(map[string]string{"new": "annotations"})
				m.Update(&u).Should(Succeed())
			})

			It("should send an event to the event stream", func() {
				Eventually(r.eventStream, timeout).Should(Receive(testutils.WithUnstructuredObject(Equal(u))))
			})
		})

		Context("when called a second time with the same object", func() {
			var originalInformers map[string]cache.SharedIndexInformer
			BeforeEach(func() {
				originalInformers = r.informers

				// Call watch with the unstructued deployment
				Expect(r.watch(u)).NotTo(HaveOccurred())
			})

			It("should not change the existing informers", func() {
				Expect(r.informers).To(Equal(originalInformers))
			})
		})
	})
})
