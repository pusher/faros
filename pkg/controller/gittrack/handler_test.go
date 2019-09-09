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
	farosflags "github.com/pusher/faros/pkg/flags"
	farosclient "github.com/pusher/faros/pkg/utils/client"
	testutils "github.com/pusher/faros/test/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

		recFn, _ := newReconciler(mgr)
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

		var AssertNoChild = func() {
			It("does not create a GitTrackObject for the child", func() {
				m.Get(gto, consistentlyTimeout).ShouldNot(Succeed())
			})
		}

		var AssertIgnoresWrongNamespaceChild = func(r *handlerResult) {

			AssertNoChild()

			It("ignores the child resource", func() {
				key := fmt.Sprintf("%s/%s", gto.GetNamespace(), gto.GetName())
				value := fmt.Sprintf("namespace `%s` is not managed by this GitTrack", gto.GetNamespace())
				Expect(r.ignoredFiles).To(HaveKeyWithValue(key, value))
			})
		}

		var AssertClusterGitTrackIgnoresNamespaced = func(r *handlerResult) {
			AssertNoChild()

			It("ignores the child resource", func() {
				key := fmt.Sprintf("%s/%s", gto.GetNamespace(), gto.GetName())
				value := fmt.Sprintf("namespaced resources cannot be managed by ClusterGitTrack")
				Expect(r.ignoredFiles).To(HaveKeyWithValue(key, value))
			})
		}

		var AssertClusterGitTrackHandlingDisabled = func(r *handlerResult) {
			AssertNoChild()

			It("ignores the child resource", func() {
				key := fmt.Sprintf("%s/%s", gto.GetNamespace(), gto.GetName())
				value := fmt.Sprintf("ClusterGitTrack handling disabled; ignoring")
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

		var AssertIgnoreInvalidFiles = func(numIgnored int) {
			It("adds the file to `ignoredFiles`", func() {
				Expect(result.ignoredFiles).To(HaveKeyWithValue("invalid_file.yaml", MatchRegexp("unable to parse 'invalid_file.yaml':")))
			})

			It("includes the invalid file in `ignored` count", func() {
				Expect(result.ignored).To(BeNumerically("==", numIgnored))
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

		var AssertClusterScopedResourceDisallowed = func() {
			Context("for the namespace file", func() {
				BeforeEach(func() {
					gto = testutils.ExampleClusterGitTrackObject.DeepCopy()
					gto.SetName("namespace-test")
				})
				AssertNoChild()

				It("ignores the child resource", func() {
					key := gto.GetName()
					value := "cluster scoped object managed from gittrack are not allowed"
					Expect(result.ignoredFiles).To(HaveKeyWithValue(key, value))
				})

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

		var AssertChildOwnedByOtherController = func(r *handlerResult) {
			var AssertIgnored = func(kind string) {
				var otherGt farosv1alpha1.GitTrackInterface
				var otherGto farosv1alpha1.GitTrackObjectInterface

				BeforeEach(func() {
					switch kind {
					case "GitTrack":
						otherGt = testutils.ExampleGitTrack.DeepCopy()
					case "ClusterGitTrack":
						otherGt = testutils.ExampleClusterGitTrack.DeepCopy()
					default:
						panic("this should not be reachable")
					}
					otherGt.SetName("othergittrack")
					m.Create(otherGt).Should(Succeed())
					m.Get(otherGt).Should(Succeed())

					gto = testutils.ExampleGitTrackObject.DeepCopy()
					gto.SetName("deployment-nginx")
					otherGto = testutils.ExampleGitTrackObject.DeepCopy()
					otherGto.SetName("deployment-nginx")
					truth := true
					otherGto.SetOwnerReferences([]metav1.OwnerReference{
						{
							APIVersion:         "faros.pusher.com/v1alpha1",
							Kind:               kind,
							Name:               otherGt.GetName(),
							UID:                otherGt.GetUID(),
							Controller:         &truth,
							BlockOwnerDeletion: &truth,
						},
					})
					m.Create(otherGto).Should(Succeed())
					m.Get(otherGto).Should(Succeed())
				})

				It("should not overwrite the existing child", func() {
					m.Consistently(gto, consistentlyTimeout).Should(Equal(otherGto))
				})

				It("should ignore the child", func() {
					key := fmt.Sprintf("%s/%s", gto.GetNamespace(), gto.GetName())
					value := fmt.Sprintf("child is already owned by a %s: %s", kind, otherGt.GetName())
					Expect(r.ignoredFiles).To(HaveKeyWithValue(key, value))
				})
			}

			Context("is owned by a GitTrack", func() {
				AssertIgnored("GitTrack")
			})

			Context("is owned by a ClusterGitTrack", func() {
				AssertIgnored("ClusterGitTrack")
			})
		}

		var AssertChildNameWithColon = func() {
			BeforeEach(func() {
				gto = testutils.ExampleClusterGitTrackObject.DeepCopy()
				gto.SetName("clusterrole-test-read-ns-pods-svcs")
			})

			It("replaces `:` with `-` in the name", func() {
				m.Get(gto, timeout).Should(Succeed())
			})
		}

		var AssertGitTrackUpdated = func(kind string) {
			BeforeEach(func() {
				By("Executing the handler on and older commit")
				result = r.handleGitTrack(gt)
				Expect(result.gcError).ToNot(HaveOccurred())
				Expect(result.gitError).ToNot(HaveOccurred())
				Expect(result.parseError).ToNot(HaveOccurred())
				Expect(result.upToDateError).ToNot(HaveOccurred())

				By("Creating existing GitTrackObjects and ClusterGitTrackObjects")
				gtoList := &farosv1alpha1.GitTrackObjectList{}
				m.Eventually(gtoList, timeout).Should(testutils.WithItems(HaveLen(2)))
				cgtoList := &farosv1alpha1.GitTrackObjectList{}
				m.Eventually(cgtoList, timeout).Should(testutils.WithItems(HaveLen(2)))
			})

			Context("and resources are added to the repository", func() {
				BeforeEach(func() {
					gto = testutils.ExampleGitTrackObject.DeepCopy()
					gto.SetName("ingress-example")
					m.Get(gto, consistentlyTimeout).ShouldNot(Succeed())
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "09d24c51c191b4caacd35cda23bd44c86f16edc6"), timeout).Should(Succeed())
				})

				It("creates the new resources", func() {
					m.Get(gto, timeout).Should(Succeed())
				})
			})

			Context("and resources are removed from the repository", func() {
				BeforeEach(func() {
					By("Executing the handler on an older commit")
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "4532b487a5aaf651839f5401371556aa16732a6e"), timeout).Should(Succeed())
					result = r.handleGitTrack(gt)
					Expect(result.gcError).ToNot(HaveOccurred())
					Expect(result.gitError).ToNot(HaveOccurred())
					Expect(result.parseError).ToNot(HaveOccurred())
					Expect(result.upToDateError).ToNot(HaveOccurred())

					By("Ensuring the desired GTO was created")
					gto = testutils.ExampleGitTrackObject.DeepCopy()
					gto.SetName("configmap-deleted-config")
					m.Get(gto, consistentlyTimeout).Should(Succeed())

					gtoList := &farosv1alpha1.GitTrackObjectList{}
					m.Eventually(gtoList, timeout).Should(testutils.WithItems(HaveLen(3)))

					By("Setting the reference to a commit after the resource was removed")
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "28928ccaeb314b96293e18cc8889997f0f46b79b"), timeout).Should(Succeed())
				})

				It("deletes the newly removed resource", func() {
					m.Get(gto, timeout).ShouldNot(Succeed())
				})

				It("does not remove any other resources", func() {
					gtoList := &farosv1alpha1.GitTrackObjectList{}
					m.Eventually(gtoList, timeout).Should(testutils.WithItems(HaveLen(2)))
				})
			})

			Context("and a resource is updated", func() {
				var originalSpec farosv1alpha1.GitTrackObjectSpec
				var serviceVersion string
				var serviceGto farosv1alpha1.GitTrackObjectInterface

				BeforeEach(func() {
					gto = testutils.ExampleGitTrackObject.DeepCopy()
					gto.SetName("deployment-nginx")
					m.Get(gto).Should(Succeed())
					originalSpec = gto.GetSpec()
					m.Eventually(gto, timeout).Should(testutils.WithField("Spec", Equal(originalSpec)))

					serviceGto = testutils.ExampleGitTrackObject.DeepCopy()
					serviceGto.SetName("service-nginx")
					m.Get(serviceGto).Should(Succeed())
					serviceVersion = serviceGto.GetResourceVersion()

					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "448b39a21d285fcb5aa4b718b27a3e13ffc649b3"), timeout).Should(Succeed())
				})

				It("updates the resource spec as expected", func() {
					m.Eventually(gto, timeout).Should(testutils.WithField("Spec", Not(Equal(originalSpec))))
				})

				It("does not update other resources", func() {
					m.Consistently(serviceGto, consistentlyTimeout).Should(testutils.WithField("ObjectMeta.ResourceVersion", Equal(serviceVersion)))
				})

				Context("should send successful update events", func() {
					AssertSendsEvent("UpdateSuccessful", kind, "example", corev1.EventTypeNormal)
				})

				It("should not send udpate failure events", func() {
					eventList := &corev1.EventList{}
					m.Consistently(eventList, consistentlyTimeout).ShouldNot(testutils.WithItems(ContainElement(testutils.WithField("Reason", Equal("UpdateFailed")))))
				})
			})

			Context("and the new reference is invalid", func() {
				var existingGtos *farosv1alpha1.GitTrackObjectList
				var existingCGtos *farosv1alpha1.ClusterGitTrackObjectList

				BeforeEach(func() {
					existingGtos = &farosv1alpha1.GitTrackObjectList{}
					m.Eventually(existingGtos, timeout).Should(testutils.WithItems(HaveLen(2)))
					existingCGtos = &farosv1alpha1.ClusterGitTrackObjectList{}
					m.Eventually(existingCGtos, timeout).Should(testutils.WithItems(HaveLen(0)))
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "invalid-ref"), timeout).Should(Succeed())
				})

				It("should not modify any of the existing GitTrackObjects", func() {
					gtoList := &farosv1alpha1.GitTrackObjectList{}
					m.Consistently(gtoList, consistentlyTimeout).Should(testutils.WithField("Items", Equal(existingGtos.Items)))
				})

				It("should not modify any of the existing ClusterGitTrackObjects", func() {
					gtoList := &farosv1alpha1.ClusterGitTrackObjectList{}
					m.Consistently(gtoList, consistentlyTimeout).Should(testutils.WithField("Items", Equal(existingCGtos.Items)))
				})

				It("should set a gitError in the result", func() {
					Expect(result.gitError).To(HaveOccurred())
					Expect(result.gitError.Error()).To(Equal("failed to checkout 'invalid-ref': unable to parse ref invalid-ref: reference not found"))
				})

				It("should set a gitReason in the result", func() {
					Expect(result.gitReason).To(Equal(gittrackutils.ErrorFetchingFiles))
				})

				Context("should send an event for failing to checkout the repository", func() {
					AssertSendsEvent("CheckoutFailed", kind, "example", corev1.EventTypeWarning)
				})
			})

			Context("and the subPath does not exist", func() {
				var existingGtos *farosv1alpha1.GitTrackObjectList
				var existingCGtos *farosv1alpha1.ClusterGitTrackObjectList

				BeforeEach(func() {
					existingGtos = &farosv1alpha1.GitTrackObjectList{}
					m.Eventually(existingGtos, timeout).Should(testutils.WithItems(HaveLen(2)))
					existingCGtos = &farosv1alpha1.ClusterGitTrackObjectList{}
					m.Eventually(existingCGtos, timeout).Should(testutils.WithItems(HaveLen(0)))
					m.UpdateWithFunc(gt, setGitTrackSubPathFunc("invalid-subpath"), timeout).Should(Succeed())
				})

				It("should not modify any of the existing GitTrackObjects", func() {
					gtoList := &farosv1alpha1.GitTrackObjectList{}
					m.Consistently(gtoList, consistentlyTimeout).Should(testutils.WithField("Items", Equal(existingGtos.Items)))
				})

				It("should not modify any of the existing ClusterGitTrackObjects", func() {
					gtoList := &farosv1alpha1.ClusterGitTrackObjectList{}
					m.Consistently(gtoList, consistentlyTimeout).Should(testutils.WithField("Items", Equal(existingCGtos.Items)))
				})

				It("should set a gitError in the result", func() {
					Expect(result.gitError).To(HaveOccurred())
					Expect(result.gitError.Error()).To(Equal("no files for subpath 'invalid-subpath'"))
				})

				It("should set a gitReason in the result", func() {
					Expect(result.gitReason).To(Equal(gittrackutils.ErrorFetchingFiles))
				})

				Context("should send an event for failing to checkout the repository", func() {
					AssertSendsEvent("CheckoutFailed", kind, "example", corev1.EventTypeWarning)
				})
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

			Context("with invalid files", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "936b7ee3df1dbd61b1fc691b742fa5d5d3c0dced"), timeout).Should(Succeed())
				})

				AssertIgnoreInvalidFiles(4)
			})

			Context("with a cluster scoped resource", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "b17c0e0f45beca3f1c1e62a7f49fecb738c60d42"), timeout).Should(Succeed())
				})

				AssertClusterScopedResourceDisallowed()
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

			PContext("when a child is owner by another controller", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "4c31dbdd7103dc209c8bb21b75d78b3efafadc31"), timeout).Should(Succeed())
				})

				AssertChildOwnedByOtherController(&result)
			})

			Context("when a child name contains colons", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "241786090da55894dca4e91e3f5023c024d3d9a8"), timeout).Should(Succeed())
				})

				AssertChildNameWithColon()
			})

			Context("when the GitTrack is updated", func() {
				AssertGitTrackUpdated(kind)
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

			Context("with invalid files", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "936b7ee3df1dbd61b1fc691b742fa5d5d3c0dced"), timeout).Should(Succeed())
				})

				AssertIgnoreInvalidFiles(1)
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

			PContext("when a child is owner by another controller", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "4c31dbdd7103dc209c8bb21b75d78b3efafadc31"), timeout).Should(Succeed())
				})

				AssertChildOwnedByOtherController(&result)
			})

			Context("when a child name contains colons", func() {
				BeforeEach(func() {
					m.UpdateWithFunc(gt, setGitTrackReferenceFunc(repositoryURL, "241786090da55894dca4e91e3f5023c024d3d9a8"), timeout).Should(Succeed())
				})

				AssertChildNameWithColon()
			})

			Context("when the ClusterGitTrack is updated", func() {
				AssertGitTrackUpdated(kind)
			})

			Context("when the ClusterGitTrack is excluded from managing namespaced object", func() {
				BeforeEach(func() {
					r.clusterGitTrackMode = farosflags.CGTMExcludeNamespaced
					gto = testutils.ExampleGitTrackObject.DeepCopy()
					gto.SetName("deployment-nginx")
				})
				AssertClusterGitTrackIgnoresNamespaced(&result)
			})

			Context("when ClusterGitTrack handling is disabled", func() {
				BeforeEach(func() {
					r.clusterGitTrackMode = farosflags.CGTMDisabled
					gto = testutils.ExampleGitTrackObject.DeepCopy()
					gto.SetName("deployment-nginx")
				})
				AssertClusterGitTrackHandlingDisabled(&result)
			})
		})
	})
})
