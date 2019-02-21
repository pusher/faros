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

package client

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pusher/faros/pkg/utils/client/test"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ = Describe("Applier Suite", func() {
	var c client.Client
	var a Client
	var o *ApplyOptions
	var m test.Matcher

	var deployment *appsv1.Deployment
	var mgrStopped *sync.WaitGroup
	var stopMgr chan struct{}

	const timeout = time.Second * 5
	const consistentlyTimeout = time.Second

	BeforeEach(func() {
		mgr, err := manager.New(cfg, manager.Options{})
		Expect(err).NotTo(HaveOccurred())
		c = mgr.GetClient()
		m = test.Matcher{Client: c}

		a, err = NewApplier(mgr.GetConfig(), Options{})
		Expect(err).NotTo(HaveOccurred())
		o = &ApplyOptions{}

		stopMgr, mgrStopped = StartTestManager(mgr)

		deployment = test.ExampleDeployment.DeepCopy()
	})

	AfterEach(func() {
		close(stopMgr)
		mgrStopped.Wait()

		test.DeleteAll(cfg, timeout,
			&appsv1.DeploymentList{},
		)
	})

	Describe("when the deployment does not exist", func() {
		BeforeEach(func() {
			Expect(a.Apply(context.TODO(), o, deployment)).NotTo(HaveOccurred())
		})

		It("creates the deployment", func() {
			m.Get(deployment, timeout).Should(Succeed())
		})

		It("should default the deployment object", func() {
			Expect(deployment).ShouldNot(test.WithUID(BeEmpty()))
			Expect(deployment).ShouldNot(test.WithResourceVersion(BeEmpty()))
			Expect(deployment).ShouldNot(test.WithCreationTimestamp(Equal(metav1.Time{})))
			Expect(deployment).ShouldNot(test.WithSelfLink(BeEmpty()))
		})

		Context("with ServerDryRun true", func() {
			BeforeEach(func() {
				serverDryRun := true
				o.ServerDryRun = &serverDryRun
				Expect(a.Apply(context.TODO(), o, deployment)).NotTo(HaveOccurred())
			})

			It("not to create the deployment", func() {
				m.Get(deployment, timeout).ShouldNot(Succeed())
			})

			It("should default the deployment object", func() {
				Expect(deployment).ShouldNot(test.WithUID(BeEmpty()))
				Expect(deployment).ShouldNot(test.WithResourceVersion(BeEmpty()))
				Expect(deployment).ShouldNot(test.WithCreationTimestamp(Equal(metav1.Time{})))
				Expect(deployment).ShouldNot(test.WithSelfLink(BeEmpty()))
			})
		})
	})
})
