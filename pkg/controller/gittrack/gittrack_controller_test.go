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
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	"github.com/pusher/faros/pkg/controller/gittrack/metrics"
	gittrackutils "github.com/pusher/faros/pkg/controller/gittrack/utils"
	farosflags "github.com/pusher/faros/pkg/flags"
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

var _ = Describe("GitTrack Suite", func() {
	var c client.Client
	var mgr manager.Manager
	var instance farosv1alpha1.GitTrackInterface
	var requests chan reconcile.Request
	var stop chan struct{}
	var r reconcile.Reconciler

	var key = types.NamespacedName{Name: "example", Namespace: "default"}
	var expectedRequest = reconcile.Request{NamespacedName: key}

	const timeout = time.Second * 5
	const filePathRegexp = "^[a-zA-Z0-9/\\-\\.]*\\.(?:yaml|yml|json)$"
	const doesNotExistPath = "does-not-exist"
	const repeatedReference = "448b39a21d285fcb5aa4b718b27a3e13ffc649b3"

	var createInstance = func(gt farosv1alpha1.GitTrackInterface, ref string) {
		spec := gt.GetSpec()
		spec.Reference = ref
		gt.SetSpec(spec)
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

	BeforeEach(func() {
		// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
		// channel when it is finished.
		farosflags.Namespace = "default"

		var err error
		cfg.RateLimiter = flowcontrol.NewFakeAlwaysRateLimiter()
		mgr, err = manager.New(cfg, manager.Options{
			Namespace:          farosflags.Namespace,
			MetricsBindAddress: "0", // Disable serving metrics while testing
		})
		Expect(err).NotTo(HaveOccurred())
		c = mgr.GetClient()

		var recFn reconcile.Reconciler
		var opts *reconcileGitTrackOpts
		r, opts = newReconciler(mgr)
		recFn, requests = SetupTestReconcile(r)
		Expect(add(mgr, recFn, opts)).NotTo(HaveOccurred())
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
			&farosv1alpha1.ClusterGitTrackList{},
			&farosv1alpha1.GitTrackObjectList{},
			&farosv1alpha1.ClusterGitTrackObjectList{},
			&v1.EventList{},
		)
		farosflags.Namespace = ""
	})

	Context("When a GitTrack resource is created", func() {
		Context("in a different namespace", func() {
			var ns *v1.Namespace
			BeforeEach(func() {
				ns = &v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "not-default",
					},
				}
				Expect(c.Create(context.TODO(), ns)).NotTo(HaveOccurred())
				instance.SetNamespace("not-default")
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
		Context("and resources in the repository are updated", func() {
			BeforeEach(func() {
				createInstance(instance, "a14443638218c782b84cae56a14f1090ee9e5c9c")
				// Wait for client cache to expire
				waitForInstanceCreated(key)
			})

			It("updates the time to deploy metric", func() {
				// Reset the metric before testing
				metrics.TimeToDeploy.Reset()

				Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
				Expect(instance.GetSpec().Reference).To(Equal("a14443638218c782b84cae56a14f1090ee9e5c9c"))

				// Update the reference
				spec := instance.GetSpec()
				spec.Reference = repeatedReference
				instance.SetSpec(spec)
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
						"repository": instance.GetSpec().Repository,
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
			Expect(instance.GetStatus().IgnoredFiles).To(HaveKeyWithValue("invalid_file.yaml", "unable to parse 'invalid_file.yaml': unable to unmarshal JSON: Object 'Kind' is missing in '{\"I\":\"a;m an \\\"invalid Kubernetes manifest.)\"}'\n"))
		})

		It("includes the invalid file in ignoredObjects count", func() {
			Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
			Expect(instance.GetStatus().IgnoredFiles).To(HaveLen(int(instance.GetStatus().ObjectsIgnored)))
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
			c := instance.GetStatus().Conditions[3]
			Expect(c.Type).To(Equal(farosv1alpha1.ChildrenUpToDateType))
			Expect(c.Status).To(Equal(v1.ConditionTrue))
			Expect(c.LastUpdateTime).NotTo(BeNil())
			Expect(c.LastTransitionTime).NotTo(BeNil())
			Expect(c.LastUpdateTime).To(Equal(c.LastTransitionTime))
			Expect(c.Reason).To(Equal(string(gittrackutils.ChildrenUpdateSuccess)))

			Expect(instance.GetStatus().ObjectsIgnored).To(Equal(int64(1)))
		})

		It("adds a message to the ignoredFiles status", func() {
			Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
			Expect(instance.GetStatus().IgnoredFiles).To(HaveKeyWithValue("default/deployment-nginx", "resource `deployments.apps/v1` ignored globally by flag"))
		})

		It("includes the ignored files in ignoredObjects count", func() {
			Eventually(func() error { return c.Get(context.TODO(), key, instance) }, timeout).Should(Succeed())
			Expect(instance.GetStatus().IgnoredFiles).To(HaveLen(int(instance.GetStatus().ObjectsIgnored)))
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

var _ = Describe("ClusterGitTrack Suite", func() {
	var c client.Client
	var mgr manager.Manager
	var instance farosv1alpha1.GitTrackInterface
	var requests chan reconcile.Request
	var stop chan struct{}
	var r reconcile.Reconciler

	const timeout = time.Second * 5
	const filePathRegexp = "^[a-zA-Z0-9/\\-\\.]*\\.(?:yaml|yml|json)$"
	const doesNotExistPath = "does-not-exist"
	const repeatedReference = "448b39a21d285fcb5aa4b718b27a3e13ffc649b3"

	var createInstance = func(gt farosv1alpha1.GitTrackInterface, ref string) {
		spec := gt.GetSpec()
		spec.Reference = ref
		gt.SetSpec(spec)
		err := c.Create(context.TODO(), gt)
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		farosflags.Namespace = "default"

		var err error
		cfg.RateLimiter = flowcontrol.NewFakeAlwaysRateLimiter()
		mgr, err = manager.New(cfg, manager.Options{
			Namespace:          farosflags.Namespace,
			MetricsBindAddress: "0", // Disable serving metrics while testing
		})
		Expect(err).NotTo(HaveOccurred())
		c = mgr.GetClient()

		var recFn reconcile.Reconciler
		var opts *reconcileGitTrackOpts
		r, opts = newReconciler(mgr)
		recFn, requests = SetupTestReconcile(r)
		opts.clusterGitTrackMode = farosflags.CGTMDisabled
		Expect(add(mgr, recFn, opts)).NotTo(HaveOccurred())
		stop = StartTestManager(mgr)
		instance = &farosv1alpha1.ClusterGitTrack{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "",
			},
			Spec: farosv1alpha1.GitTrackSpec{
				Repository: repositoryURL,
			},
		}
	})

	Context("When a ClusterGitTrack resource is created", func() {
		BeforeEach(func() {
			createInstance(instance, "a14443638218c782b84cae56a14f1090ee9e5c9c")
		})
		It("should not reconcile it", func() {
			Eventually(requests, timeout).ShouldNot(Receive())
		})
	})

	AfterEach(func() {
		close(stop)
		testutils.DeleteAll(cfg, timeout,
			&farosv1alpha1.GitTrackList{},
			&farosv1alpha1.ClusterGitTrackList{},
			&farosv1alpha1.GitTrackObjectList{},
			&farosv1alpha1.ClusterGitTrackObjectList{},
			&v1.EventList{},
		)
		farosflags.Namespace = ""
	})
})
