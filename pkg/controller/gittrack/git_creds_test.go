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
)

var _ = Describe("GitTrack Suite", func() {
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
