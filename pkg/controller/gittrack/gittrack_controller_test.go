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
	"fmt"
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	"github.com/pusher/faros/pkg/controller/gittrack/metrics"
	gittrackutils "github.com/pusher/faros/pkg/controller/gittrack/utils"
	farosflags "github.com/pusher/faros/pkg/flags"
	farosclient "github.com/pusher/faros/pkg/utils/client"
	testevents "github.com/pusher/faros/test/events"
	testutils "github.com/pusher/faros/test/utils"
	gitstore "github.com/pusher/git-store"
	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client
var mgr manager.Manager
var instance *farosv1alpha1.GitTrack
var requests chan reconcile.Request
var stop chan struct{}
var r reconcile.Reconciler

var key = types.NamespacedName{Name: "example", Namespace: "default"}
var expectedRequest = reconcile.Request{NamespacedName: key}

const timeout = time.Second * 5
const filePathRegexp = "^[a-zA-Z0-9/\\-\\.]*\\.(?:yaml|yml|json)$"
const doesNotExistPath = "does-not-exist"
const repeatedReference = "448b39a21d285fcb5aa4b718b27a3e13ffc649b3"

var _ = Describe("GitTrack Suite", func() {
	var createInstance = func(gt *farosv1alpha1.GitTrack, ref string) {
		gt.Spec.Reference = ref
		err := c.Create(context.TODO(), gt)
		Expect(err).NotTo(HaveOccurred())
	}

	var waitForInstanceCreated = func(key types.NamespacedName) {
		request := reconcile.Request{NamespacedName: key}
		// wait for reconcile for creating the GitTrack resource
		Eventually(requests, timeout).Should(Receive(Equal(request)))
		// wait for reconcile for updating the GitTrack resource's status
		Eventually(requests, timeout).Should(Receive(Equal(request)))
		obj := &farosv1alpha1.GitTrack{}
		Eventually(func() error {
			err := c.Get(context.TODO(), key, obj)
			if err != nil {
				return err
			}
			if len(obj.Status.Conditions) == 0 {
				return fmt.Errorf("Status not updated")
			}
			return nil
		}, timeout).Should(Succeed())
	}

	var reasonFilter = func(reason string) func(v1.Event) bool {
		return func(e v1.Event) bool { return e.Reason == reason }
	}

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

		var recFn reconcile.Reconciler
		r = newReconciler(mgr)
		recFn, requests = SetupTestReconcile(r)
		Expect(add(mgr, recFn)).NotTo(HaveOccurred())
		stop = StartTestManager(mgr)
		instance = &farosv1alpha1.GitTrack{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "default",
			},
			Spec: farosv1alpha1.GitTrackSpec{
				Repository: repositoryURL,
			},
		}
	})

	AfterEach(func() {
		close(stop)
		testutils.DeleteAll(cfg, timeout,
			&farosv1alpha1.GitTrackList{},
			&farosv1alpha1.GitTrackObjectList{},
			&farosv1alpha1.ClusterGitTrackObjectList{},
			&v1.EventList{},
		)
	})

	Context("When a GitTrack resource is created", func() {
		Context("with a valid Spec", func() {
			BeforeEach(func() {
				createInstance(instance, "a14443638218c782b84cae56a14f1090ee9e5c9c")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("updates its status", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				two, zero := int64(2), int64(0)
				Expect(instance.Status.ObjectsDiscovered).To(Equal(two))
				Expect(instance.Status.ObjectsApplied).To(Equal(two))
				Expect(instance.Status.ObjectsIgnored).To(Equal(zero))
				Expect(instance.Status.ObjectsInSync).To(Equal(zero))

				deployGto := &farosv1alpha1.GitTrackObject{}
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployGto)
				}, timeout).Should(Succeed())

				now := metav1.NewTime(time.Now())
				deployGto.Status.Conditions = []farosv1alpha1.GitTrackObjectCondition{
					{
						Type:               farosv1alpha1.ObjectInSyncType,
						Status:             v1.ConditionTrue,
						LastTransitionTime: now,
						LastUpdateTime:     now,
					},
				}
				Expect(c.Update(context.TODO(), deployGto)).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				Expect(instance.Status.ObjectsInSync).To(Equal(int64(1)))
			})

			It("sets the status conditions", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				conditions := instance.Status.Conditions
				Expect(len(conditions)).To(Equal(4))
				parseErrorCondition := conditions[0]
				gitErrorCondition := conditions[1]
				gcErrorCondition := conditions[2]
				upToDateCondiiton := conditions[3]
				Expect(parseErrorCondition.Type).To(Equal(farosv1alpha1.FilesParsedType))
				Expect(gitErrorCondition.Type).To(Equal(farosv1alpha1.FilesFetchedType))
				Expect(gcErrorCondition.Type).To(Equal(farosv1alpha1.ChildrenGarbageCollectedType))
				Expect(upToDateCondiiton.Type).To(Equal(farosv1alpha1.ChildrenUpToDateType))
			})

			Context("sets the status metrics", func() {
				var setsMetric = func(status string, value float64) {
					It(fmt.Sprintf("sets status `%s` to %f", status, value), func() {
						var gauge prometheus.Gauge
						Eventually(func() error {
							var err error
							gauge, err = metrics.ChildStatus.GetMetricWith(map[string]string{
								"name":      instance.GetName(),
								"namespace": instance.GetNamespace(),
								"status":    status,
							})
							return err
						}, timeout).Should(Succeed())
						var metric dto.Metric
						Expect(gauge.Write(&metric)).NotTo(HaveOccurred())
						Expect(metric.GetGauge().GetValue()).To(Equal(value))
					})
				}

				setsMetric("discovered", 2.0)
				setsMetric("applied", 2.0)
				setsMetric("ignored", 0.0)
				setsMetric("inSync", 0.0)
			})

			It("creates GitTrackObjects", func() {
				deployGto := &farosv1alpha1.GitTrackObject{}
				serviceGto := &farosv1alpha1.GitTrackObject{}
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployGto)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, serviceGto)
				}, timeout).Should(Succeed())
			})

			It("sets ownerReferences for created GitTrackObjects", func() {
				deployGto := &farosv1alpha1.GitTrackObject{}
				serviceGto := &farosv1alpha1.GitTrackObject{}
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployGto)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, serviceGto)
				}, timeout).Should(Succeed())
				Expect(len(deployGto.OwnerReferences)).To(Equal(1))
				Expect(len(serviceGto.OwnerReferences)).To(Equal(1))
			})

			It("sets LastAppliedAnnotations for created GitTrackObjects", func() {
				deployGto := &farosv1alpha1.GitTrackObject{}
				serviceGto := &farosv1alpha1.GitTrackObject{}
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployGto)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, serviceGto)
				}, timeout).Should(Succeed())
				Expect(deployGto.GetAnnotations()).To(HaveKey(farosclient.LastAppliedAnnotation))
				Expect(serviceGto.GetAnnotations()).To(HaveKey(farosclient.LastAppliedAnnotation))
			})

			It("sends events about checking out configured Git repository", func() {
				events := &v1.EventList{}
				Eventually(func() error { return c.List(context.TODO(), events) }, timeout).Should(Succeed())
				startEvents := testevents.Select(events.Items, reasonFilter("CheckoutStarted"))
				successEvents := testevents.Select(events.Items, reasonFilter("CheckoutSuccessful"))
				Expect(startEvents).ToNot(BeEmpty())
				Expect(successEvents).ToNot(BeEmpty())
				for _, e := range append(startEvents, successEvents...) {
					Expect(e.InvolvedObject.Kind).To(Equal("GitTrack"))
					Expect(e.InvolvedObject.Name).To(Equal("example"))
					Expect(e.Type).To(Equal(string(v1.EventTypeNormal)))
				}
			})

			It("sends events about creating GitTrackObjects", func() {
				events := &v1.EventList{}
				Eventually(func() error { return c.List(context.TODO(), events) }, timeout).Should(Succeed())
				startEvents := testevents.Select(events.Items, reasonFilter("CreateStarted"))
				successEvents := testevents.Select(events.Items, reasonFilter("CreateSuccessful"))
				Expect(startEvents).ToNot(BeEmpty())
				Expect(successEvents).ToNot(BeEmpty())
				for _, e := range append(startEvents, successEvents...) {
					Expect(e.InvolvedObject.Kind).To(Equal("GitTrack"))
					Expect(e.InvolvedObject.Name).To(Equal("example"))
					Expect(e.Type).To(Equal(string(v1.EventTypeNormal)))
				}
			})
		})

		Context("with multi-document YAML", func() {
			BeforeEach(func() {
				createInstance(instance, "9bf412f0e893c8c1624bb1c523cfeca8243534bc")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("creates GitTrackObjects", func() {
				dsGto, cmGto := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "daemonset-fluentd", Namespace: "default"}, dsGto)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "configmap-fluentd-config", Namespace: "default"}, cmGto)
				}, timeout).Should(Succeed())
			})
		})

		Context("with a cluster scoped resource", func() {
			BeforeEach(func() {
				createInstance(instance, "b17c0e0f45beca3f1c1e62a7f49fecb738c60d42")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("creates ClusterGitTrackObject", func() {
				nsCGto := &farosv1alpha1.ClusterGitTrackObject{}
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "namespace-test", Namespace: ""}, nsCGto)
				}, timeout).Should(Succeed())
			})
		})

		Context("with an invalid Reference", func() {
			BeforeEach(func() {
				createInstance(instance, doesNotExistPath)
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("updates the FilesFetched condition", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				// TODO: don't rely on ordering
				c := instance.Status.Conditions[1]
				Expect(c.Type).To(Equal(farosv1alpha1.FilesFetchedType))
				Expect(c.Status).To(Equal(v1.ConditionFalse))
				Expect(c.LastUpdateTime).NotTo(BeNil())
				Expect(c.LastTransitionTime).NotTo(BeNil())
				Expect(c.LastUpdateTime).To(Equal(c.LastTransitionTime))
				Expect(c.Reason).To(Equal(string(gittrackutils.ErrorFetchingFiles)))
				Expect(c.Message).To(Equal("failed to checkout 'does-not-exist': unable to parse ref does-not-exist: reference not found"))
			})

			It("sends a CheckoutFailed event", func() {
				events := &v1.EventList{}
				Eventually(func() error { return c.List(context.TODO(), events) }, timeout).Should(Succeed())
				failedEvents := testevents.Select(events.Items, reasonFilter("CheckoutFailed"))
				Expect(failedEvents).ToNot(BeEmpty())
				for _, e := range failedEvents {
					Expect(e.InvolvedObject.Kind).To(Equal("GitTrack"))
					Expect(e.InvolvedObject.Name).To(Equal("example"))
					Expect(e.Type).To(Equal(string(v1.EventTypeWarning)))
				}
			})
		})

		Context("with an invalid SubPath", func() {
			BeforeEach(func() {
				instance.Spec.SubPath = doesNotExistPath
				createInstance(instance, "master")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("updates the FilesFetched condition", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				// TODO: don't rely on ordering
				c := instance.Status.Conditions[1]
				Expect(c.Type).To(Equal(farosv1alpha1.FilesFetchedType))
				Expect(c.Status).To(Equal(v1.ConditionFalse))
				Expect(c.LastUpdateTime).NotTo(BeNil())
				Expect(c.LastTransitionTime).NotTo(BeNil())
				Expect(c.LastUpdateTime).To(Equal(c.LastTransitionTime))
				Expect(c.Reason).To(Equal(string(gittrackutils.ErrorFetchingFiles)))
				Expect(c.Message).To(Equal("no files for subpath 'does-not-exist'"))
			})

			It("sends a CheckoutFailed event", func() {
				events := &v1.EventList{}
				Eventually(func() error { return c.List(context.TODO(), events) }, timeout).Should(Succeed())
				failedEvents := testevents.Select(events.Items, reasonFilter("CheckoutFailed"))
				Expect(failedEvents).ToNot(BeEmpty())
				for _, e := range failedEvents {
					Expect(e.InvolvedObject.Kind).To(Equal("GitTrack"))
					Expect(e.InvolvedObject.Name).To(Equal("example"))
					Expect(e.Type).To(Equal(v1.EventTypeWarning))
				}
			})
		})

		Context("with files from an unmanaged namespace", func() {
			BeforeEach(func() {
				instance.Spec.SubPath = "foo"
				createInstance(instance, "4c31dbdd7103dc209c8bb21b75d78b3efafadc31")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("ignores the files", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())

				// TODO: don't rely on ordering
				c := instance.Status.Conditions[3]
				Expect(c.Type).To(Equal(farosv1alpha1.ChildrenUpToDateType))
				Expect(c.Status).To(Equal(v1.ConditionTrue))
				Expect(c.LastUpdateTime).NotTo(BeNil())
				Expect(c.LastTransitionTime).NotTo(BeNil())
				Expect(c.LastUpdateTime).To(Equal(c.LastTransitionTime))
				Expect(c.Reason).To(Equal(string(gittrackutils.ChildrenUpdateSuccess)))

				Expect(instance.Status.ObjectsIgnored).To(Equal(int64(2)))
			})

			It("adds a message to the ignoredFiles status", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				Expect(instance.Status.IgnoredFiles).To(HaveKeyWithValue("foo/deployment-nginx", "namespace `foo` is not managed by this Faros"))
				Expect(instance.Status.IgnoredFiles).To(HaveKeyWithValue("foo/service-nginx", "namespace `foo` is not managed by this Faros"))
			})

			It("includes the ignored files in ignoredObjects count", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				Expect(instance.Status.IgnoredFiles).To(HaveLen(int(instance.Status.ObjectsIgnored)))
			})
		})

		Context("with a child owned by another controller", func() {
			truth := true
			var existingChild *farosv1alpha1.GitTrackObject
			BeforeEach(func() {
				existingChild = &farosv1alpha1.GitTrackObject{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "deployment-nginx",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "faros.pusher.com/v1alpha1",
								Kind:               "GitTrack",
								Name:               doesNotExistPath,
								UID:                "12345",
								Controller:         &truth,
								BlockOwnerDeletion: &truth,
							},
						},
					},
					Spec: farosv1alpha1.GitTrackObjectSpec{
						Name: "nginx",
						Kind: "Deployment",
						Data: []byte("kind: Deployment"),
					},
				}
				err := c.Create(context.TODO(), existingChild)
				Expect(err).ToNot(HaveOccurred())

				createInstance(instance, "4c31dbdd7103dc209c8bb21b75d78b3efafadc31")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("should not overwrite the existing child", func() {
				deployGto := &farosv1alpha1.GitTrackObject{}
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployGto)
				}, timeout).Should(Succeed())

				o := deployGto.ObjectMeta
				Expect(o.OwnerReferences).To(Equal(existingChild.ObjectMeta.OwnerReferences))
				Expect(o.Name).To(Equal(existingChild.ObjectMeta.Name))
				Expect(o.Namespace).To(Equal(existingChild.ObjectMeta.Namespace))

				Expect(deployGto.Spec).To(Equal(existingChild.Spec))
			})

			It("should ignore the GitTrackObject", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())

				// TODO: don't rely on ordering
				c := instance.Status.Conditions[3]
				Expect(c.Type).To(Equal(farosv1alpha1.ChildrenUpToDateType))
				Expect(c.Status).To(Equal(v1.ConditionTrue))
				Expect(c.LastUpdateTime).NotTo(BeNil())
				Expect(c.LastTransitionTime).NotTo(BeNil())
				Expect(c.LastUpdateTime).To(Equal(c.LastTransitionTime))
				Expect(c.Reason).To(Equal(string(gittrackutils.ChildrenUpdateSuccess)))

				Expect(instance.Status.ObjectsIgnored).To(Equal(int64(3)))
			})
		})

		Context("with a child resource that has a name that contains `:`", func() {
			BeforeEach(func() {
				createInstance(instance, "241786090da55894dca4e91e3f5023c024d3d9a8")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("replaces `:` with `-`", func() {
				clusterRoleGto := &farosv1alpha1.ClusterGitTrackObject{}
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "clusterrole-test-read-ns-pods-svcs"}, clusterRoleGto)
				}, timeout).Should(Succeed())
				Expect(clusterRoleGto.Name).To(Equal("clusterrole-test-read-ns-pods-svcs"))
			})
		})

		Context("in a different namespace", func() {
			var ns *v1.Namespace
			BeforeEach(func() {
				ns = &v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "not-default",
					},
				}
				Expect(c.Create(context.TODO(), ns)).NotTo(HaveOccurred())
				instance.Namespace = "not-default"
				createInstance(instance, "a14443638218c782b84cae56a14f1090ee9e5c9c")
			})

			AfterEach(func() {
				Expect(c.Delete(context.TODO(), ns)).NotTo(HaveOccurred())
			})

			It("should not reconcile it", func() {
				Eventually(requests, timeout).ShouldNot(Receive())
			})
		})
	})

	Context("When a GitTrack resource is updated", func() {
		Context("and resources are added to the repository", func() {
			BeforeEach(func() {
				createInstance(instance, "28928ccaeb314b96293e18cc8889997f0f46b79b")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("creates the new resources", func() {
				before, after := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				Expect(instance.Spec.Reference).To(Equal("28928ccaeb314b96293e18cc8889997f0f46b79b"))

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "ingress-example", Namespace: "default"}, before)
				}, timeout).ShouldNot(Succeed())

				instance.Spec.Reference = "09d24c51c191b4caacd35cda23bd44c86f16edc6"
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "ingress-example", Namespace: "default"}, after)
				}, timeout).Should(Succeed())
			})
		})

		Context("and resources are removed from the repository", func() {
			BeforeEach(func() {
				createInstance(instance, "4532b487a5aaf651839f5401371556aa16732a6e")
				// Wait for client cache to expire
				waitForInstanceCreated(key)

				// Check the instance created
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				Expect(instance.Spec.Reference).To(Equal("4532b487a5aaf651839f5401371556aa16732a6e"))

				// Check the configmap to be deleted was created
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "configmap-deleted-config", Namespace: "default"}, &farosv1alpha1.GitTrackObject{})
				}, timeout).Should(Succeed())

				// Update the repository
				instance.Spec.Reference = "28928ccaeb314b96293e18cc8889997f0f46b79b"
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())

				// Wait for cache to sync
				waitForInstanceCreated(key)
			})

			It("deletes the removed resources", func() {
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "configmap-deleted-config", Namespace: "default"}, &farosv1alpha1.GitTrackObject{})
				}, timeout).ShouldNot(Succeed())
			})

			It("doesn't delete any other resources", func() {
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "configmap-deleted-config", Namespace: "default"}, &farosv1alpha1.GitTrackObject{})
				}, timeout).ShouldNot(Succeed())

				gtos := &farosv1alpha1.GitTrackObjectList{}
				err := c.List(context.TODO(), gtos, client.InNamespace(instance.Namespace))
				Expect(err).ToNot(HaveOccurred())
				Expect(len(gtos.Items)).To(Equal(2))
			})
		})

		Context("and resources in the repository are updated", func() {
			BeforeEach(func() {
				createInstance(instance, "a14443638218c782b84cae56a14f1090ee9e5c9c")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("updates the updated resources", func() {
				before, after := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				Expect(instance.Spec.Reference).To(Equal("a14443638218c782b84cae56a14f1090ee9e5c9c"))

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, before)
				}, timeout).Should(Succeed())

				instance.Spec.Reference = repeatedReference
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				Eventually(func() error {
					err = c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, after)
					if err != nil {
						return nil
					}
					if reflect.DeepEqual(after.Spec, before.Spec) {
						return fmt.Errorf("deployment not updated yet")
					}
					return nil
				}, timeout).Should(Succeed())
				Expect(after.Spec).ToNot(Equal(before.Spec))
			})

			It("doesn't modify any other resources", func() {
				before, after := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				Expect(instance.Spec.Reference).To(Equal("a14443638218c782b84cae56a14f1090ee9e5c9c"))

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, before)
				}, timeout).Should(Succeed())

				instance.Spec.Reference = repeatedReference
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, after)
				}, timeout).Should(Succeed())
				Expect(after.Spec).To(Equal(before.Spec))
			})

			It("sends events about updating resources", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				instance.Spec.Reference = repeatedReference
				Expect(c.Update(context.TODO(), instance)).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error {
					events := &v1.EventList{}
					err := c.List(context.TODO(), events)
					if err != nil {
						return err
					}
					if testevents.None(events.Items, reasonFilter("UpdateSuccessful")) {
						return fmt.Errorf("events hasn't been sent yet")
					}
					return nil
				}, timeout*2).Should(Succeed())
				events := &v1.EventList{}
				Eventually(func() error { return c.List(context.TODO(), events) }, timeout).Should(Succeed())
				successEvents := testevents.Select(events.Items, reasonFilter("UpdateSuccessful"))
				failedEvents := testevents.Select(events.Items, reasonFilter("UpdateFailed"))
				Expect(successEvents).ToNot(BeEmpty())
				Expect(failedEvents).To(BeEmpty())
				for _, e := range successEvents {
					Expect(e.InvolvedObject.Kind).To(Equal("GitTrack"))
					Expect(e.InvolvedObject.Name).To(Equal("example"))
					Expect(e.Type).To(Equal(string(v1.EventTypeNormal)))
					Expect(e.Reason).To(Equal("UpdateSuccessful"))
				}
			})

			It("updates the time to deploy metric", func() {
				// Reset the metric before testing
				metrics.TimeToDeploy.Reset()

				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				Expect(instance.Spec.Reference).To(Equal("a14443638218c782b84cae56a14f1090ee9e5c9c"))

				// Update the reference
				instance.Spec.Reference = repeatedReference
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				Eventually(func() error {
					labels := map[string]string{
						"name":       instance.GetName(),
						"namespace":  instance.GetNamespace(),
						"repository": instance.Spec.Repository,
					}
					histObserver := metrics.TimeToDeploy.With(labels)
					hist := histObserver.(prometheus.Histogram)
					var timeToDeploy dto.Metric
					hist.Write(&timeToDeploy)
					if timeToDeploy.GetHistogram().GetSampleCount() != uint64(4) {
						return fmt.Errorf("metrics not updated")
					}
					return nil
				}, timeout).Should(Succeed())
			})
		})

		Context("and the subPath has changed", func() {
		})

		Context("and the reference is invalid", func() {
			BeforeEach(func() {
				createInstance(instance, "a14443638218c782b84cae56a14f1090ee9e5c9c")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("does not update any of the resources", func() {
				deployBefore, deployAfter := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				serviceBefore, serviceAfter := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployBefore)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, serviceBefore)
				}, timeout).Should(Succeed())
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())

				instance.Spec.Reference = doesNotExistPath
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployAfter)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, serviceAfter)
				}, timeout).Should(Succeed())

				Expect(deployBefore).To(Equal(deployAfter))
				Expect(serviceBefore).To(Equal(serviceAfter))
			})

			It("updates the FilesFetched condition", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				instance.Spec.Reference = doesNotExistPath
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error {
					err = c.Get(context.TODO(), key, instance)
					if err != nil {
						return err
					}
					c := instance.Status.Conditions[1]
					if c.Status != v1.ConditionFalse {
						return fmt.Errorf("condition hasn't updated yet")
					}
					return nil
				}, timeout).Should(Succeed())
				// TODO: don't rely on ordering
				c := instance.Status.Conditions[1]
				Expect(c.Type).To(Equal(farosv1alpha1.FilesFetchedType))
				Expect(c.LastUpdateTime).NotTo(BeNil())
				Expect(c.LastTransitionTime).NotTo(BeNil())
				Expect(c.LastUpdateTime).To(Equal(c.LastTransitionTime))
				Expect(c.Reason).To(Equal(string(gittrackutils.ErrorFetchingFiles)))
				Expect(c.Message).To(Equal("failed to checkout 'does-not-exist': unable to parse ref does-not-exist: reference not found"))
			})

			It("sends a CheckoutFailed event", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				instance.Spec.Reference = doesNotExistPath
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error {
					events := &v1.EventList{}
					err := c.List(context.TODO(), events)
					if err != nil {
						return err
					}
					if testevents.None(events.Items, reasonFilter("CheckoutFailed")) {
						return fmt.Errorf("events hasn't been sent yet")
					}
					return nil
				}, timeout).Should(Succeed())
				events := &v1.EventList{}
				Eventually(func() error { return c.List(context.TODO(), events) }, timeout).Should(Succeed())
				failedEvents := testevents.Select(events.Items, reasonFilter("CheckoutFailed"))
				Expect(failedEvents).ToNot(BeEmpty())
				for _, e := range failedEvents {
					Expect(e.InvolvedObject.Kind).To(Equal("GitTrack"))
					Expect(e.InvolvedObject.Name).To(Equal("example"))
					Expect(e.Type).To(Equal(v1.EventTypeWarning))
				}
			})
		})

		Context("and the subPath is invalid", func() {
			BeforeEach(func() {
				createInstance(instance, "a14443638218c782b84cae56a14f1090ee9e5c9c")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("does not update any of the resources", func() {
				deployBefore, deployAfter := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}
				serviceBefore, serviceAfter := &farosv1alpha1.GitTrackObject{}, &farosv1alpha1.GitTrackObject{}

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployBefore)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, serviceBefore)
				}, timeout).Should(Succeed())
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())

				instance.Spec.SubPath = doesNotExistPath
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "deployment-nginx", Namespace: "default"}, deployAfter)
				}, timeout).Should(Succeed())
				Eventually(func() error {
					return c.Get(context.TODO(), types.NamespacedName{Name: "service-nginx", Namespace: "default"}, serviceAfter)
				}, timeout).Should(Succeed())

				Expect(deployBefore).To(Equal(deployAfter))
				Expect(serviceBefore).To(Equal(serviceAfter))
			})

			It("updates the FilesFetched condition", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				instance.Spec.SubPath = doesNotExistPath
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error {
					err = c.Get(context.TODO(), key, instance)
					if err != nil {
						return err
					}
					c := instance.Status.Conditions[1]
					if c.Status != v1.ConditionFalse {
						return fmt.Errorf("condition hasn't updated yet")
					}
					return nil
				}, timeout).Should(Succeed())
				// TODO: don't rely on ordering
				c := instance.Status.Conditions[1]
				Expect(c.Type).To(Equal(farosv1alpha1.FilesFetchedType))
				Expect(c.LastUpdateTime).NotTo(BeNil())
				Expect(c.LastTransitionTime).NotTo(BeNil())
				Expect(c.LastUpdateTime).To(Equal(c.LastTransitionTime))
				Expect(c.Reason).To(Equal(string(gittrackutils.ErrorFetchingFiles)))
				Expect(c.Message).To(Equal("no files for subpath 'does-not-exist'"))
			})

			It("sends a CheckoutFailed event", func() {
				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				instance.Spec.SubPath = doesNotExistPath
				err := c.Update(context.TODO(), instance)
				Expect(err).ToNot(HaveOccurred())
				// Wait for reconcile for update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// Wait for reconcile for status update
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				filter := func(e v1.Event) bool {
					return e.Reason == "CheckoutFailed" && e.Message == "No files for SubPath 'does-not-exist'"
				}
				Eventually(func() error {
					events := &v1.EventList{}
					err := c.List(context.TODO(), events)
					if err != nil {
						return err
					}
					if testevents.None(events.Items, filter) {
						return fmt.Errorf("events hasn't been sent yet")
					}
					return nil
				}, timeout).Should(Succeed())
				events := &v1.EventList{}
				Eventually(func() error { return c.List(context.TODO(), events) }, timeout).Should(Succeed())
				failedEvents := testevents.Select(events.Items, filter)
				Expect(failedEvents).ToNot(BeEmpty())
				for _, e := range failedEvents {
					Expect(e.InvolvedObject.Kind).To(Equal("GitTrack"))
					Expect(e.InvolvedObject.Name).To(Equal("example"))
					Expect(e.Message).To(Equal("No files for SubPath 'does-not-exist'"))
					Expect(e.Type).To(Equal(string(v1.EventTypeWarning)))
				}
			})
		})
	})

	Context("When a GitTrack resource is deleted", func() {
	})

	Context("When a GitTrack has a DeployKey, the Reconciler should", func() {
		var reconciler *ReconcileGitTrack
		var s *v1.Secret
		var keyRef farosv1alpha1.GitTrackDeployKey
		var expectedKey []byte

		keysMustBeSetErr := errors.New("if using a deploy key, both SecretName and Key must be set")
		secretNotFoundErr := errors.New("failed to look up secret nonExistSecret: Secret \"nonExistSecret\" not found")

		BeforeEach(func() {
			var ok bool
			reconciler, ok = r.(*ReconcileGitTrack)
			Expect(ok).To(BeTrue())

			keyRef = farosv1alpha1.GitTrackDeployKey{
				SecretName: "foosecret",
				Key:        "privatekey",
			}

			expectedKey = []byte("PrivateKey")
			s = &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foosecret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"privatekey": expectedKey,
				},
			}
			Expect(c.Create(context.TODO(), s)).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			c.Delete(context.TODO(), s)
		})

		It("do nothing if the secret name and key are empty", func() {
			key, err := reconciler.fetchGitCredentials("default", farosv1alpha1.GitTrackDeployKey{})
			Expect(err).NotTo(HaveOccurred())
			Expect(key).To(BeNil())
		})

		It("get the key from the secret", func() {
			key, err := reconciler.fetchGitCredentials("default", keyRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(key.secret).To(Equal(expectedKey))
		})

		It("return an error if the secret doesn't exist", func() {
			keyRef.SecretName = "nonExistSecret"
			key, err := reconciler.fetchGitCredentials("default", keyRef)
			Expect(err).To(Equal(secretNotFoundErr))
			Expect(key).To(BeNil())
		})

		It("return an error if the secret name isnt set, but the key is", func() {
			keyRef.SecretName = ""
			key, err := reconciler.fetchGitCredentials("default", keyRef)
			Expect(err).To(Equal(keysMustBeSetErr))
			Expect(key).To(BeNil())
		})

		It("return an error if the key isnt set, but the secret name is", func() {
			keyRef.Key = ""
			key, err := reconciler.fetchGitCredentials("default", keyRef)
			Expect(err).To(Equal(keysMustBeSetErr))
			Expect(key).To(BeNil())
		})
	})

	Context("When getting files from a repository", func() {
		/*
			foo
			├── bar
			│   ├── non-yaml-file.txt
			│   └── service.yaml
			├── deployment.yaml
			└── namespace.yaml
		*/

		getsFilesFromRepo("foo", 3)
		getsFilesFromRepo("foo/", 3)
		getsFilesFromRepo("/foo/", 3)
		getsFilesFromRepo("foo/bar", 1)
		getsFilesFromRepo("foobar", 2)
		getsFilesFromRepo("foobar/", 2)
	})

	Context(fmt.Sprintf("with invalid files"), func() {
		BeforeEach(func() {
			createInstance(instance, "936b7ee3df1dbd61b1fc691b742fa5d5d3c0dced")
			waitForInstanceCreated(key)
		})

		It("adds a message to the ignoredFiles status", func() {
			Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
			Expect(instance.Status.IgnoredFiles).To(HaveKeyWithValue("invalid_file.yaml", "unable to parse 'invalid_file.yaml': unable to unmarshal JSON: Object 'Kind' is missing in '{\"I\":\"a;m an \\\"invalid Kubernetes manifest.)\"}'\n"))
		})

		It("includes the invalid file in ignoredObjects count", func() {
			Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
			Expect(instance.Status.IgnoredFiles).To(HaveLen(int(instance.Status.ObjectsIgnored)))
		})
	})

	Context("When a list of ignored GVRs is supplied", func() {
		BeforeEach(func() {
			reconciler, ok := r.(*ReconcileGitTrack)
			Expect(ok).To(BeTrue())
			reconciler.ignoredGVRs = make(map[schema.GroupVersionResource]interface{})
			deploymentGVR := schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			}
			reconciler.ignoredGVRs[deploymentGVR] = nil

			createInstance(instance, "a14443638218c782b84cae56a14f1090ee9e5c9c")
			// Wait for client cache to expire
			waitForInstanceCreated(key)
		})

		It("ignores the deployment files", func() {
			Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())

			// TODO: don't rely on ordering
			c := instance.Status.Conditions[3]
			Expect(c.Type).To(Equal(farosv1alpha1.ChildrenUpToDateType))
			Expect(c.Status).To(Equal(v1.ConditionTrue))
			Expect(c.LastUpdateTime).NotTo(BeNil())
			Expect(c.LastTransitionTime).NotTo(BeNil())
			Expect(c.LastUpdateTime).To(Equal(c.LastTransitionTime))
			Expect(c.Reason).To(Equal(string(gittrackutils.ChildrenUpdateSuccess)))

			Expect(instance.Status.ObjectsIgnored).To(Equal(int64(1)))
		})

		It("adds a message to the ignoredFiles status", func() {
			Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
			Expect(instance.Status.IgnoredFiles).To(HaveKeyWithValue("default/deployment-nginx", "resource `deployments.apps/v1` ignored globally by flag"))
		})

		It("includes the ignored files in ignoredObjects count", func() {
			Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
			Expect(instance.Status.IgnoredFiles).To(HaveLen(int(instance.Status.ObjectsIgnored)))
		})
	})

	Context("listObjectsByName", func() {
		var reconciler *ReconcileGitTrack
		var children map[string]farosv1alpha1.GitTrackObjectInterface

		BeforeEach(func() {
			var ok bool
			reconciler, ok = r.(*ReconcileGitTrack)
			Expect(ok).To(BeTrue())

			createInstance(instance, "b17c0e0f45beca3f1c1e62a7f49fecb738c60d42")
			// Wait for client cache to expire
			waitForInstanceCreated(key)

			var err error
			children, err = reconciler.listObjectsByName(instance)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return 6 child objects", func() {
			Expect(children).Should(HaveLen(6))
		})

		It("should return 5 namespaced objects", func() {
			var count int
			for _, obj := range children {
				if _, ok := obj.(*farosv1alpha1.GitTrackObject); ok {
					count++
				}
			}
			Expect(count).To(Equal(5))
		})

		It("should return 1 non-namespaced resource", func() {
			var count int
			for _, obj := range children {
				if _, ok := obj.(*farosv1alpha1.ClusterGitTrackObject); ok {
					count++
				}
			}
			Expect(count).To(Equal(1))
		})

		It("should key all items by their NamespacedName", func() {
			for key, obj := range children {
				Expect(key).Should(Equal(obj.GetNamespacedName()))
			}
		})
	})
})

var getsFilesFromRepo = func(path string, count int) {
	Context(fmt.Sprintf("With subPath %s", path), func() {
		var files map[string]*gitstore.File
		var gt *farosv1alpha1.GitTrack

		BeforeEach(func() {
			var err error
			var reconciler *ReconcileGitTrack
			var ok bool
			reconciler, ok = r.(*ReconcileGitTrack)
			Expect(ok).To(BeTrue())
			gt = &farosv1alpha1.GitTrack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: farosv1alpha1.GitTrackSpec{
					SubPath:    path,
					Repository: repositoryURL,
					DeployKey:  farosv1alpha1.GitTrackDeployKey{},
					Reference:  "51798af1c1374d1d375a0eb7a3e53dd67ac5d135",
				},
			}

			Expect(c.Create(context.TODO(), gt)).NotTo(HaveOccurred())
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test",
					Namespace: "default",
				},
			}
			Eventually(requests, timeout).Should(Receive(Equal(req)))

			files, err = reconciler.getFiles(gt)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			Expect(c.Delete(context.TODO(), gt)).NotTo(HaveOccurred())
		})

		It("Filters files by SubPath", func() {
			for filePath := range files {
				Expect(filePath).To(HavePrefix(strings.TrimPrefix(path, "/")))
			}
		})

		It("Filters files by file extension", func() {
			for filePath := range files {
				Expect(filePath).To(MatchRegexp(filePathRegexp))
			}
		})

		It("Fetches all files recursively from the SubPath", func() {
			Expect(files).To(HaveLen(count))
		})

	})
}
