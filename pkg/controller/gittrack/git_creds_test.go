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
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gitstore "github.com/pusher/git-store"
	testutils "github.com/pusher/faros/test/utils"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	farosclient "github.com/pusher/faros/pkg/utils/client"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitTrack Suite", func() {
	Describe("fetchGitCredentials", func() {
		var m testutils.Matcher
		var r *ReconcileGitTrack
		var mgr manager.Manager
		var stop chan struct{}

		var gt farosv1alpha1.GitTrackInterface
		var secret *corev1.Secret
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

			applier, err := farosclient.NewApplier(cfg, farosclient.Options{})
			Expect(err).NotTo(HaveOccurred())

			c, err := client.New(mgr.GetConfig(), client.Options{})
			Expect(err).NotTo(HaveOccurred())

			m = testutils.Matcher{Client: c, FarosClient: applier}

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

		var setDeployKey = func(gt farosv1alpha1.GitTrackInterface, dk farosv1alpha1.GitTrackDeployKey) {
			spec := gt.GetSpec()
			spec.DeployKey = dk
			gt.SetSpec(spec)
		}

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

		var AssertWithDeployKey = func() {
			Context("which has a DeployKey" , func() {
				var dk farosv1alpha1.GitTrackDeployKey

				var keysMustBeSetErr = errors.New("both SecretName and Key must be set when using DeployKey")
				var secretNotFoundErr = errors.New("failed to look up secret nonExistentSecret: Secret \"nonExistentSecret\" not found")

				BeforeEach(func() {
					By("Creating a secret with a private key")
					secret = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "foosecret",
							Namespace: "default",
						},
						Data: map[string][]byte{
							"privatekey": []byte("PrivateKey"),
						},
					}
					m.Create(secret).Should(Succeed())

					dk = farosv1alpha1.GitTrackDeployKey{
						SecretName: "foosecret",
						SecretNamespace: "default",
						Key:        "privatekey",
					}
					setDeployKey(gt, dk)
				})

				AfterEach(func() {
					m.Delete(secret).Should(Succeed())
				})

				Context("if the secret exists", func() {
					It("retrieves the private key from the secret", func() {
						Expect(creds).ToNot(BeNil())
						Expect(creds.secret).To(Equal(secret.Data["privatekey"]))
					})
				})

				Context("if the secret does not exist", func() {
					BeforeEach(func() {
						dk.SecretName = "nonExistentSecret"
						setDeployKey(gt, dk)
					})

					It("returns an error", func() {
						Expect(credsErr).ToNot(BeNil())
						Expect(credsErr).To(Equal(secretNotFoundErr))
					})

					It("returns nil credentials", func() {
						Expect(creds).To(BeNil())
					})
				})

				Context("if a key is not set within the DeployKey", func () {
					BeforeEach(func() {
						dk.Key = ""
						setDeployKey(gt, dk)
					})

					Context("but the secret name is", func() {
						It("returns an error", func() {
							Expect(credsErr).ToNot(BeNil())
							Expect(credsErr).To(Equal(keysMustBeSetErr))
						})

						It("returns nil credentials", func() {
							Expect(creds).To(BeNil())
						})
					})
				})

				Context("if the secret name is not set within the DeployKey", func () {
					BeforeEach(func() {
						dk.SecretName = ""
						setDeployKey(gt, dk)
					})

					Context("but the key name is", func() {
						It("returns an error", func() {
							Expect(credsErr).ToNot(BeNil())
							Expect(credsErr).To(Equal(keysMustBeSetErr))
						})

						It("returns nil credentials", func() {
							Expect(creds).To(BeNil())
						})
					})
				})

				Context("if the secret namespace is not set", func() {
					BeforeEach(func() {
						dk.SecretNamespace = ""
						setDeployKey(gt, dk)
					})

					Context("with a GitTrack", func() {
						It("fetches the secret from the GitTrack's Namespace", func() {
							if _, ok := gt.(*farosv1alpha1.GitTrack); !ok {
								Skip("test behaviour is for GitTracks only")
							}

							Expect(creds).ToNot(BeNil())
							Expect(creds.secret).To(Equal(secret.Data["privatekey"]))
						})

						It("should return no error", func() {
							if _, ok := gt.(*farosv1alpha1.GitTrack); !ok {
								Skip("test behaviour is for GitTracks only")
							}

							Expect(credsErr).To(BeNil())
						})
					})

					Context("With a ClusterGitTrack", func() {
						It("returns an error", func() {
							if _, ok := gt.(*farosv1alpha1.ClusterGitTrack); !ok {
								Skip("test behaviour is for ClusterGitTracks only")
							}

							Expect(credsErr).To(Equal(errors.New("no SecretNamespace set for DeployKey")))
						})

						It("returns nil credentials", func() {
							if _, ok := gt.(*farosv1alpha1.ClusterGitTrack); !ok {
								Skip("test behaviour is for ClusterGitTracks only")
							}

							Expect(creds).To(BeNil())
						})
					})
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
			AssertWithDeployKey()
		})

		Context("With a ClusterGitTrack", func() {
			BeforeEach(func() {
				gt = testutils.ExampleClusterGitTrack.DeepCopy()
			})

			AssertNoDeployKey()
			AssertWithDeployKey()
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
					Expect(err.Error()).To(Equal("you must specify the secret as <username>:<password> for credential type HTTPBasicAuth"))
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
