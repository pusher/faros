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
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	farosflags "github.com/pusher/faros/pkg/flags"
	farosclient "github.com/pusher/faros/pkg/utils/client"
	testutils "github.com/pusher/faros/test/utils"
	gitstore "github.com/pusher/git-store"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ = Describe("GitTrack Suite", func() {
	var r *ReconcileGitTrack
	var mgr manager.Manager
	var m testutils.Matcher
	var subPath string
	var reference string
	var url string
	var stop chan struct{}
	var gt farosv1alpha1.GitTrackInterface

	const timeout = 5 * time.Second

	var setupSpec = func(gt farosv1alpha1.GitTrackInterface, reference, url, subpath string) {
		spec := gt.GetSpec()
		spec.Repository = url
		spec.Reference = reference
		spec.SubPath = subpath
		gt.SetSpec(spec)
	}

	BeforeEach(func() {
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

		recFn, _ := newReconciler(mgr)
		r = recFn.(*ReconcileGitTrack)

		stop = StartTestManager(mgr)

		// Reset defaults before each test
		url = repositoryURL
		reference = "936b7ee3df1dbd61b1fc691b742fa5d5d3c0dced"
		subPath = ""
		gt = nil
	})

	JustBeforeEach(func() {
		// Not all tests depend on a {Cluster,}GitTrack
		if gt != nil {
			setupSpec(gt, reference, url, subPath)
		}
	})

	AfterEach(func() {
		// Stop Controller and informers before cleaning up
		close(stop)

		testutils.DeleteAll(cfg, timeout, &corev1.EventList{})
	})

	Describe("checkoutRepo", func() {
		var checkoutErr error
		var repo *gitstore.Repo
		var credentials *gitCredentials

		JustBeforeEach(func() {
			repo, checkoutErr = r.checkoutRepo(url, reference, credentials)
		})

		Context("when no credentials are required", func() {
			It("returns a repository", func() {
				Expect(repo).ToNot(BeNil())
			})

			It("does not return an error", func() {
				Expect(checkoutErr).ToNot(HaveOccurred())
			})

			It("sets the last updated time", func() {
				ts, _ := time.Parse(time.RFC3339, "2019-04-03T10:11:54+01:00")
				Expect(r.lastUpdateTimes).To(HaveKeyWithValue(url, BeTemporally("==", ts)))
			})
		})

		PContext("when given invalid credentials", func() {
			BeforeEach(func() {
				url = "git@github.com:"
			})

			It("returns an error", func() {
				Expect(checkoutErr).ToNot(BeNil())
			})
		})

		Context("when given an invalid repository URL", func() {
			BeforeEach(func() {
				url = repositoryURL + "-invalid"
			})

			It("returns an error", func() {
				Expect(checkoutErr).To(MatchError(MatchRegexp("failed to get repository '%s': repository not found'", url)))
			})
		})

		Context("when given an invalid reference", func() {
			BeforeEach(func() {
				reference = "invalid-reference"
			})

			It("returns an error", func() {
				Expect(checkoutErr).To(MatchError(MatchRegexp("failed to checkout '%s': unable to parse ref %s: reference not found", reference, reference)))
			})
		})

		Context("when the checkout times out", func() {
			var fetchTimeout time.Duration

			BeforeEach(func() {
				fetchTimeout = farosflags.FetchTimeout
				farosflags.FetchTimeout = 0 * time.Second
			})

			AfterEach(func() {
				farosflags.FetchTimeout = fetchTimeout
			})

			It("returns an error", func() {
				Expect(checkoutErr).To(MatchError(MatchRegexp("timed out getting repository '%s'", url)))
			})
		})
	})

	Describe("getFiles", func() {
		var files map[string]*gitstore.File
		var filesErr error

		JustBeforeEach(func() {
			files, filesErr = r.getFiles(gt)
		})

		var AssertSendsEvent = func(reason, kind, name, eventType string) {
			It(fmt.Sprintf("sends a %s event", reason), func() {
				events := &corev1.EventList{}
				m.Eventually(events, timeout).Should(testutils.WithItems(ContainElement(SatisfyAll(
					testutils.WithField("Reason", Equal(reason)),
					testutils.WithField("Type", Equal(eventType)),
					testutils.WithField("InvolvedObject.Kind", Equal(kind)),
					testutils.WithField("InvolvedObject.Name", Equal(name)),
				))))
			})
		}

		var AssertValidSubPath = func() {
			Context("with a subPath that contains files", func() {
				It("returns files", func() {
					Expect(files).ToNot(BeEmpty())
				})

				It("recursively finds files", func() {
					Expect(files).To(HaveLen(12))
				})

				It("filters files by extension", func() {
					for path := range files {
						Expect(path).To(SatisfyAny(
							HaveSuffix(".yaml"),
							HaveSuffix(".yml"),
							HaveSuffix(".json"),
						))
					}
				})

				It("does not return an error", func() {
					Expect(filesErr).To(BeNil())
				})
			})
		}

		var AssertSubPathWithoutTrailingSlash = func(prefix string, count int) {
			Context("with a subPath that does not end with a slash", func() {
				BeforeEach(func() {
					subPath = prefix
					reference = "51798af1c1374d1d375a0eb7a3e53dd67ac5d135"
				})

				It("returns files", func() {
					Expect(files).To(HaveLen(count))
				})

				It("does not return files from other directories with a similar prefix", func() {
					for path := range files {
						Expect(path).To(HavePrefix(prefix + "/"))
					}
				})

				It("filters files by extension", func() {
					for path := range files {
						Expect(path).To(SatisfyAny(
							HaveSuffix(".yaml"),
							HaveSuffix(".yml"),
							HaveSuffix(".json"),
						))
					}
				})

				It("does not return an error", func() {
					Expect(filesErr).To(BeNil())
				})
			})
		}

		var AssertSubPathWithLeadingSlash = func(prefix string, count int) {
			Context("subPath with leading slash", func() {
				BeforeEach(func() {
					subPath = prefix
					reference = "51798af1c1374d1d375a0eb7a3e53dd67ac5d135"
				})

				It("returns files", func() {
					Expect(files).To(HaveLen(count))
				})

				It("does not return files from directories with a similar prefix", func() {
					for path := range files {
						Expect(path).To(HavePrefix(strings.TrimPrefix(prefix, "/") + "/"))
					}
				})

				It("filters files by extension", func() {
					for path := range files {
						Expect(path).To(SatisfyAny(
							HaveSuffix(".yaml"),
							HaveSuffix(".yml"),
							HaveSuffix(".json"),
						))
					}
				})

				It("does not return any error", func() {
					Expect(filesErr).To(BeNil())
				})
			})
		}

		var AssertEmptySubPath = func(kind string) {
			Context("with a subPath that does not contain any files", func() {
				BeforeEach(func() {
					subPath = "non-existent-path"
				})

				It("does not return any files", func() {
					Expect(files).To(BeEmpty())
				})

				It("returns an error", func() {
					Expect(filesErr).To(MatchError(MatchRegexp("no files for subpath '%s'", subPath)))
				})

				AssertSendsEvent("CheckoutFailed", kind, "example", corev1.EventTypeWarning)
			})
		}

		Context("with a GitTrack", func() {
			const kind = "GitTrack"

			BeforeEach(func() {
				gt = testutils.ExampleGitTrack.DeepCopy()
			})

			AssertSendsEvent("CheckoutStarted", kind, "example", corev1.EventTypeNormal)
			AssertValidSubPath()
			AssertSubPathWithoutTrailingSlash("foo", 3)
			AssertSubPathWithoutTrailingSlash("foobar", 2)
			AssertSubPathWithLeadingSlash("/foo", 3)
			AssertSubPathWithLeadingSlash("/foobar", 2)
			AssertEmptySubPath(kind)
		})

		Context("with a ClusterGitTrack", func() {
			const kind = "ClusterGitTrack"

			BeforeEach(func() {
				gt = testutils.ExampleClusterGitTrack.DeepCopy()
			})

			AssertSendsEvent("CheckoutStarted", kind, "example", corev1.EventTypeNormal)
			AssertValidSubPath()
			AssertSubPathWithoutTrailingSlash("foo", 3)
			AssertSubPathWithoutTrailingSlash("foobar", 2)
			AssertSubPathWithLeadingSlash("/foo", 3)
			AssertSubPathWithLeadingSlash("/foobar", 2)
			AssertEmptySubPath(kind)
		})
	})

	Describe("objectsFrom", func() {
		var objects []*unstructured.Unstructured
		var parseErrs map[string]string

		BeforeEach(func() {
			gt = testutils.ExampleGitTrack.DeepCopy()
		})

		JustBeforeEach(func() {
			files, err := r.getFiles(gt)
			Expect(err).ToNot(HaveOccurred())
			objects, parseErrs = objectsFrom(files)
		})

		Context("with valid YAML files", func() {
			BeforeEach(func() {
				reference = "a14443638218c782b84cae56a14f1090ee9e5c9c"
			})

			It("returns objects", func() {
				Expect(objects).To(HaveLen(2))
			})

			It("does not return any errors", func() {
				Expect(parseErrs).To(BeEmpty())
			})
		})

		Context("with valid multi-document YAML files", func() {
			BeforeEach(func() {
				reference = "9bf412f0e893c8c1624bb1c523cfeca8243534bc"
			})

			It("returns all expected objects", func() {
				Expect(objects).To(HaveLen(5))
			})

			It("does not return any errors", func() {
				Expect(parseErrs).To(BeEmpty())
			})
		})

		Context("with invalid YAML files", func() {
			BeforeEach(func() {
				reference = "936b7ee3df1dbd61b1fc691b742fa5d5d3c0dced"
			})

			It("returns errors", func() {
				Expect(parseErrs).To(HaveKey("invalid_file.yaml"))
			})
		})

		PContext("with invalid YAMLs interleaved", func() {
			BeforeEach(func() {
				reference = "TODO"
			})

			It("returns the valid ones", func() {
				Expect(objects).ToNot(BeEmpty())
			})

			It("returns errors", func() {
				Expect(parseErrs).ToNot(BeEmpty())
			})
		})
	})
})
