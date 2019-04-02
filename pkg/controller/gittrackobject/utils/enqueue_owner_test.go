package utils_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	. "github.com/pusher/faros/pkg/controller/gittrackobject/utils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("EnqueueRequestForOwner Suite", func() {
	var queue workqueue.RateLimitingInterface
	var enqueue *EnqueueRequestForOwner
	var createEvt event.CreateEvent
	var updateEvt event.UpdateEvent
	var deleteEvt event.DeleteEvent
	var genericEvt event.GenericEvent

	var shouldEnqueue = func() {
		It("should enqueue a request", func() {
			Expect(queue.Len()).To(Equal(1))
		})
	}

	var shouldNotEnqueue = func() {
		It("should not enqueue a request", func() {
			Expect(queue.Len()).To(Equal(0))
		})
	}

	var enqueuedItemShouldBeReconcileRequest = func() {
		It("should enqueue a reconcile.Request", func() {
			item, shutdown := queue.Get()
			Expect(shutdown).To(BeFalse())
			_, ok := item.(reconcile.Request)
			Expect(ok).To(BeTrue())
		})
	}

	var nonNamespacedEnqueuedRequestShouldBeFor = func() {
		It("reconcile.Request should be for test", func() {
			item, shutdown := queue.Get()
			Expect(shutdown).To(BeFalse())
			req := item.(reconcile.Request)
			Expect(req.Name).To(Equal("test"))
		})
	}

	var namespacedEnqueuedRequestShouldBeFor = func() {
		It("reconcile.Request should be for biz/test", func() {
			item, shutdown := queue.Get()
			Expect(shutdown).To(BeFalse())
			req := item.(reconcile.Request)
			Expect(req.String()).To(Equal("biz/test"))
		})
	}

	var namespacedEvents = func(obj *corev1.Pod, fns ...func()) {
		BeforeEach(func() {
			Expect(obj).ToNot(BeNil())
		})

		Context("with a create event", func() {
			BeforeEach(func() {
				createEvt = event.CreateEvent{
					Object: obj.DeepCopy(),
					Meta:   obj.DeepCopy().GetObjectMeta(),
				}
				enqueue.Create(createEvt, queue)
			})

			for _, fn := range fns {
				fn()
			}
		})

		Context("with an update event", func() {
			BeforeEach(func() {
				updateEvt = event.UpdateEvent{
					ObjectNew: obj.DeepCopy(),
					MetaNew:   obj.DeepCopy().GetObjectMeta(),
					ObjectOld: obj.DeepCopy(),
					MetaOld:   obj.DeepCopy().GetObjectMeta(),
				}
				enqueue.Update(updateEvt, queue)
			})

			for _, fn := range fns {
				fn()
			}
		})

		Context("with a delete event", func() {
			BeforeEach(func() {
				deleteEvt = event.DeleteEvent{
					Object: obj.DeepCopy(),
					Meta:   obj.DeepCopy().GetObjectMeta(),
				}
				enqueue.Delete(deleteEvt, queue)
			})

			for _, fn := range fns {
				fn()
			}
		})

		Context("with a generic event", func() {
			BeforeEach(func() {
				genericEvt = event.GenericEvent{
					Object: obj.DeepCopy(),
					Meta:   obj.DeepCopy().GetObjectMeta(),
				}
				enqueue.Generic(genericEvt, queue)
			})

			for _, fn := range fns {
				fn()
			}
		})
	}

	var nonNamespacedEvents = func(obj *rbacv1.ClusterRoleBinding, fns ...func()) {
		BeforeEach(func() {
			Expect(obj).ToNot(BeNil())
		})

		Context("with a create event", func() {
			BeforeEach(func() {
				createEvt = event.CreateEvent{
					Object: obj.DeepCopy(),
					Meta:   obj.DeepCopy().GetObjectMeta(),
				}
				enqueue.Create(createEvt, queue)
			})

			for _, fn := range fns {
				fn()
			}
		})

		Context("with an update event", func() {
			BeforeEach(func() {
				updateEvt = event.UpdateEvent{
					ObjectNew: obj.DeepCopy(),
					MetaNew:   obj.DeepCopy().GetObjectMeta(),
					ObjectOld: obj.DeepCopy(),
					MetaOld:   obj.DeepCopy().GetObjectMeta(),
				}
				enqueue.Update(updateEvt, queue)
			})

			for _, fn := range fns {
				fn()
			}
		})

		Context("with a delete event", func() {
			BeforeEach(func() {
				deleteEvt = event.DeleteEvent{
					Object: obj.DeepCopy(),
					Meta:   obj.DeepCopy().GetObjectMeta(),
				}
				enqueue.Delete(deleteEvt, queue)
			})

			for _, fn := range fns {
				fn()
			}
		})

		Context("with a generic event", func() {
			BeforeEach(func() {
				genericEvt = event.GenericEvent{
					Object: obj.DeepCopy(),
					Meta:   obj.DeepCopy().GetObjectMeta(),
				}
				enqueue.Generic(genericEvt, queue)
			})

			for _, fn := range fns {
				fn()
			}
		})
	}

	BeforeEach(func() {
		s := scheme.Scheme
		s.AddKnownTypes(schema.GroupVersion{
			Group:   "faros.pusher.com",
			Version: "v1alpha1",
		},
			&farosv1alpha1.GitTrackObject{},
			&farosv1alpha1.ClusterGitTrackObject{},
		)

		queue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "test")
		enqueue = &EnqueueRequestForOwner{
			NamespacedEnqueueRequestForOwner: &handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &farosv1alpha1.GitTrackObject{},
			},
			NonNamespacedEnqueueRequestForOwner: &handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &farosv1alpha1.ClusterGitTrackObject{},
			},
		}
		err := enqueue.InjectScheme(s)
		Expect(err).ToNot(HaveOccurred())
		err = enqueue.InjectMapper(testrestmapper.TestOnlyStaticRESTMapper(s))
		Expect(err).ToNot(HaveOccurred())
	})

	Context("with a non-namespaced object", func() {
		ownerReferences := []metav1.OwnerReference{
			{
				APIVersion: "faros.pusher.com/v1alpha1",
				Kind:       "ClusterGitTrackObject",
				Name:       "test",
			},
		}
		crb := &rbacv1.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ClusterRoleBinding",
				APIVersion: "rbac.authorization.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            "baz",
				OwnerReferences: ownerReferences,
			},
		}

		Context("with controller true", func() {
			t := true
			trueCRB := crb.DeepCopy()
			trueCRB.OwnerReferences[0].Controller = &t

			nonNamespacedEvents(
				trueCRB,
				shouldEnqueue,
				enqueuedItemShouldBeReconcileRequest,
				nonNamespacedEnqueuedRequestShouldBeFor,
			)

		})
		Context("with controller false", func() {
			nonNamespacedEvents(
				crb,
				shouldNotEnqueue,
			)
		})
	})

	Context("with a namespaced object", func() {
		ownerReferences := []metav1.OwnerReference{
			{
				APIVersion: "faros.pusher.com/v1alpha1",
				Kind:       "GitTrackObject",
				Name:       "test",
			},
		}
		pod := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "biz",
				Name:            "baz",
				OwnerReferences: ownerReferences,
			},
		}

		Context("with controller true", func() {
			t := true
			truePod := pod.DeepCopy()
			truePod.OwnerReferences[0].Controller = &t

			namespacedEvents(
				truePod,
				shouldEnqueue,
				enqueuedItemShouldBeReconcileRequest,
				namespacedEnqueuedRequestShouldBeFor,
			)

		})

		Context("with controller false", func() {
			namespacedEvents(
				pod,
				shouldNotEnqueue,
			)
		})
	})
})
