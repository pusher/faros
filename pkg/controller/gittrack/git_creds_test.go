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
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gitstore "github.com/pusher/git-store"
	testutils "github.com/pusher/faros/test/utils"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ = Describe("GitTrack Suite", func() {
	Describe("fetchGitCredentials", func() {
		var r *ReconcileGitTrack
		var mgr manager.Manager
		var stop chan struct{}

		var gt farosv1alpha1.GitTrackInterface
		var creds *gitCredentials
		var credsErr error

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

			stop = StartTestManager(mgr)

			creds = nil
			credsErr = nil
		})

		AfterEach(func() {
			// Stop Controller and informers before cleaning up
			close(stop)
		})

		var AssertNoDeployKey = func() {
			Context("which has no DeployKey", func() {
				It("should return no error", func() {
					Expect(credsErr).To(BeNil())
				})

				It("should return nil credentials", func() {
					Expect(creds).To(BeNil())
				})
			})
		}

		JustBeforeEach(func() {
			creds, credsErr = r.fetchGitCredentials(gt)
		})

		Context("With a GitTrack", func() {
			BeforeEach(func() {
				gt = testutils.ExampleGitTrack.DeepCopy()
			})

			AssertNoDeployKey()
		})

		Context("With a ClusterGitTrack", func() {
			BeforeEach(func() {
				gt = testutils.ExampleClusterGitTrack.DeepCopy()
			})

			AssertNoDeployKey()
		})
	})

	Describe("createRepoRefFromCreds", func() {
		Context("When the credentialType is SSH", func() {
			repo, _ := createRepoRefFromCreds("ssh@tempuri.org", &gitCredentials{
				secret:         []byte("mySecret"),
				credentialType: farosv1alpha1.GitCredentialTypeSSH,
			})

			It("sets the private key", func() {
				expected := gitstore.RepoRef{
					URL:        "ssh@tempuri.org",
					PrivateKey: []byte("mySecret"),
				}
				Expect(expected).To(BeEquivalentTo(*repo))
			})
		})

		Context("When the credentialType is HTTP basic auth", func() {
			Context("When the secret contains a username", func() {
				repo, _ := createRepoRefFromCreds("https://tempuri.org", &gitCredentials{
					secret:         []byte("username:password"),
					credentialType: farosv1alpha1.GitCredentialTypeHTTPBasicAuth,
				})

				It("sets the username and password", func() {
					expected := gitstore.RepoRef{
						URL:  "https://tempuri.org",
						User: "username",
						Pass: "password",
					}
					Expect(expected).To(BeEquivalentTo(*repo))
				})
			})

			Context("When the secret contains no colon", func() {
				repo, err := createRepoRefFromCreds("https://tempuri.org", &gitCredentials{
					secret:         []byte("password"),
					credentialType: farosv1alpha1.GitCredentialTypeHTTPBasicAuth,
				})

				It("returns an error", func() {
					Expect(repo).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("You must specify the secret as <username>:<password> for credential type HTTPBasicAuth"))
				})
			})
		})

		Context("When the credentials are nil", func() {
			repo, _ := createRepoRefFromCreds("https://tempuri.org", nil)

			It("returns a repoRef with the URL set", func() {
				Expect(repo.URL).To(Equal("https://tempuri.org"))
			})
		})
	})
})
