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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/manager"
  gitstore "github.com/pusher/git-store"
)

var _ = Describe("GitTrack Suite", func() {
	Describe("checkoutRepo", func() {
		var r *ReconcileGitTrack
		var mgr manager.Manager

		var checkoutErr error
		var repo *gitstore.Repo
		var url string
		var reference string
		var credentials *gitCredentials

		BeforeEach(func() {
			// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
			// channel when it is finished.
			var err error
			cfg.RateLimiter = flowcontrol.NewFakeAlwaysRateLimiter()
			mgr, err = manager.New(cfg, manager.Options{
				MetricsBindAddress: "0", // Disable serving metrics while testing
			})
			Expect(err).NotTo(HaveOccurred())

			recFn := newReconciler(mgr)
			r = recFn.(*ReconcileGitTrack)

			repo = nil
			checkoutErr = nil
			url = repositoryURL
			reference = "master"
			credentials = nil
		})

		JustBeforeEach(func() {
			repo, checkoutErr = r.checkoutRepo(url, reference, credentials)
		})

		Context("when no credentials are provided or required", func() {
			It("checks out the repository", func() {
				Expect(repo).ToNot(BeNil())
			})

			It("does not return an error", func() {
				Expect(checkoutErr).ToNot(HaveOccurred())
			})
		})
  })
})
