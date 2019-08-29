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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gittrackutils "github.com/pusher/faros/pkg/controller/gittrack/utils"
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

	var setGitTrackReferenceFunc = func(repo, reference string) func(testutils.Object) testutils.Object {
		return func(obj testutils.Object) testutils.Object {
			gt, ok := obj.(farosv1alpha1.GitTrackInterface)
			if !ok {
				panic("expected GitTrackInterface")
			}
			setGitTrackReference(gt, repo, reference)
			return gt
		}
	}

	var setGitTrackSubPath = func(gt farosv1alpha1.GitTrackInterface, subPath string) {
		spec := gt.GetSpec()
		spec.SubPath = subPath
		gt.SetSpec(spec)
	}

	var setGitTrackSubPathFunc = func(subPath string) func(testutils.Object) testutils.Object {
		return func(obj testutils.Object) testutils.Object {
			gt, ok := obj.(farosv1alpha1.GitTrackInterface)
			if !ok {
				panic("expected GitTrackInterface")
			}
			setGitTrackSubPath(gt, subPath)
			return gt
		}
	}

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
		var gto farosv1alpha1.GitTrackObjectInterface
		var result handlerResult

		var AssertNoErrors = func() {
			It("should return no errors", func() {
				Expect(result.parseError).ToNot(HaveOccurred())
				Expect(result.gitError).ToNot(HaveOccurred())
				Expect(result.gcError).ToNot(HaveOccurred())
				Expect(result.upToDateError).ToNot(HaveOccurred())
			})
		}

		var AssertAppliedDiscoveredIgnored = func(r *handlerResult, applied, discovered, ignored int64) {
			It(fmt.Sprintf("sets the applied children count to %d", applied), func() {
				Expect(r.applied).To(Equal(applied))
			})

			It(fmt.Sprintf("sets the discovered children count to %d", discovered), func() {
				Expect(r.discovered).To(Equal(discovered))
			})

			It(fmt.Sprintf("sets the ignored children count to %d", ignored), func() {
				Expect(r.ignored).To(Equal(ignored))
			})
		}

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

		var AssertIgnoresWrongNamespaceChild = func(r *handlerResult) {
			It("does not create a GitTrackObject for the child", func() {
				m.Get(gto, consistentlyTimeout).ShouldNot(Succeed())
			})

			PIt("ignores the child resource", func() {
				key := fmt.Sprintf("%s/%s", gto.GetNamespace(), gto.GetName())
				value := fmt.Sprintf("namespace `%s` is not managed by this GitTrack", gto.GetNamespace())
				Expect(r.ignoredFiles).To(HaveKeyWithValue(key, value))
			})
		}

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

		var AssertValidChildren = func(result *handlerResult, kind string) {
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

			AssertNoErrors()
			AssertAppliedDiscoveredIgnored(result, 2, 2, 0)

			Context("sends events when checking out the repository", func() {
				AssertSendsEvent("CheckoutStarted", kind, "example", corev1.EventTypeNormal)
				AssertSendsEvent("CheckoutSuccessful", kind, "example", corev1.EventTypeNormal)
			})

			Context("sends events when creating the child objects", func() {
				AssertSendsEvent("CreateStarted", kind, "example", corev1.EventTypeNormal)
				AssertSendsEvent("CreateSuccessful", kind, "example", corev1.EventTypeNormal)
			})
		}

		var AssertMultiDocument = func() {
			Context("for the daemonset in the file", func() {
				BeforeEach(func() {
					gto = testutils.ExampleGitTrackObject.DeepCopy()
					gto.SetName("daemonset-fluentd")
				})

				AssertChild()
			})

			Context("for the configmap in the file", func() {
				BeforeEach(func() {
					gto = testutils.ExampleGitTrackObject.DeepCopy()
					gto.SetName("configmap-fluentd-config")
				})

				AssertChild()
			})

			AssertNoErrors()
		}

		var AssertClusterScopedResource = func() {
			Context("for the namespace file", func() {
				BeforeEach(func() {
					gto = testutils.ExampleClusterGitTrackObject.DeepCopy()
					gto.SetName("namespace-test")
				})

				AssertChild()
			})

			AssertNoErrors()
		}

		var AssertInvalidReference = func(kind string) {
			It("set a gitError", func() {
				Expect(result.gitError).To(HaveOccurred())
				Expect(result.gitError.Error()).To(Equal("failed to checkout 'does-not-exist': unable to parse ref does-not-exist: reference not found"))
			})

			It("sets a gitReason", func() {
				Expect(result.gitReason).To(Equal(gittrackutils.ErrorFetchingFiles))
			})

			Context("sends events when checking out the repository", func() {
				AssertSendsEvent("CheckoutFailed", kind, "example", corev1.EventTypeWarning)
			})
		}

		var AssertInvalidSubPath = func(kind string) {
			It("set a gitError", func() {
				Expect(result.gitError).To(HaveOccurred())
				Expect(result.gitError.Error()).To(Equal("no files for subpath 'does-not-exist'"))
			})

			It("sets a gitReason", func() {
				Expect(result.gitReason).To(Equal(gittrackutils.ErrorFetchingFiles))
			})

			Context("sends events when checking out the repository", func() {
				AssertSendsEvent("CheckoutFailed", kind, "example", corev1.EventTypeWarning)
			})
		}

		Context("with a GitTrack", func() {
			kind := "GitTrack"

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
				AssertValidChildren(&result, kind)
			})

			Context("with a multi-document YAML", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "9bf412f0e893c8c1624bb1c523cfeca8243534bc"), timeout).Should(Succeed())
				})

				AssertMultiDocument()
			})

			Context("with a cluster scoped resource", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "b17c0e0f45beca3f1c1e62a7f49fecb738c60d42"), timeout).Should(Succeed())
				})

				AssertClusterScopedResource()
			})

			Context("with an invalid reference", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "does-not-exist"), timeout).Should(Succeed())
				})

				AssertInvalidReference(kind)
			})

			Context("with an invalid SubPath", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackSubPathFunc("does-not-exist"), timeout).Should(Succeed())
				})

				AssertInvalidSubPath(kind)
			})

			Context("with files from another namespace", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackSubPathFunc("foo"), timeout).Should(Succeed())
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "4c31dbdd7103dc209c8bb21b75d78b3efafadc31"), timeout).Should(Succeed())
				})

				Context("for the deployment file", func() {
					BeforeEach(func() {
						gto = testutils.ExampleGitTrackObject.DeepCopy()
						gto.SetName("deployment-nginx")
						gto.SetNamespace("foo")
					})

					AssertIgnoresWrongNamespaceChild(&result)
				})

				Context("for the service file", func() {
					BeforeEach(func() {
						gto = testutils.ExampleGitTrackObject.DeepCopy()
						gto.SetName("service-nginx")
						gto.SetNamespace("foo")
					})

					AssertIgnoresWrongNamespaceChild(&result)
				})

				AssertAppliedDiscoveredIgnored(&result, 1, 3, 2)
			})
		})

		Context("with a ClusterGitTrack", func() {
			kind := "ClusterGitTrack"

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
				AssertValidChildren(&result, kind)
			})

			Context("with a multi-document YAML", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "9bf412f0e893c8c1624bb1c523cfeca8243534bc"), timeout).Should(Succeed())
				})

				AssertMultiDocument()
			})

			Context("with a cluster scoped resource", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "b17c0e0f45beca3f1c1e62a7f49fecb738c60d42"), timeout).Should(Succeed())
				})

				AssertClusterScopedResource()
			})

			Context("with an invalid reference", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "does-not-exist"), timeout).Should(Succeed())
				})

				AssertInvalidReference(kind)
			})

			Context("with an invalid SubPath", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackSubPathFunc("does-not-exist"), timeout).Should(Succeed())
				})

				AssertInvalidSubPath(kind)
			})
		})
	})
})
