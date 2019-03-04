package gittrack

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
)

var _ = Describe("GitTrack Suite", func() {
	Describe("createRepoRefFromCreds", func() {
		Context("When the credentialType is SSH", func() {
			repo, _ := createRepoRefFromCreds("ssh@tempuri.org", &gitCredentials{
				secret:         []byte("mySecret"),
				credentialType: farosv1alpha1.GitCredentialTypeSSH,
			})

			It("sets the private key", func() {
				Expect(repo.URL).To(Equal("ssh@tempuri.org"))
				Expect(repo.PrivateKey).To(Equal([]byte("mySecret")))
			})
		})

		Context("When the credentialType is HTTP basic auth", func() {
			Context("When the secret contains a username", func() {
				repo, _ := createRepoRefFromCreds("https://tempuri.org", &gitCredentials{
					secret:         []byte("username:password"),
					credentialType: farosv1alpha1.GitCredentialTypeHTTPBasicAuth,
				})

				It("sets the username and password", func() {
					Expect(repo.URL).To(Equal("https://tempuri.org"))
					Expect(repo.User).To(Equal("username"))
					Expect(repo.Pass).To(Equal("password"))
				})
			})

			Context("When the secret contains no username", func() {
				repo, _ := createRepoRefFromCreds("https://tempuri.org", &gitCredentials{
					secret:         []byte("password"),
					credentialType: farosv1alpha1.GitCredentialTypeHTTPBasicAuth,
				})

				It("sets the  password", func() {
					Expect(repo.URL).To(Equal("https://tempuri.org"))
					Expect(repo.User).To(BeEmpty())
					Expect(repo.Pass).To(Equal("password"))
				})
			})
		})
	})
})
