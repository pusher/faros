package gittrack

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/kubernetes-sigs/kubebuilder/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNamespacedPredicate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "NamespacedPredicate Suite", []Reporter{test.NewlineReporter{}})
}

var predicate NamespacedPredicate
var matchingObjectMeta = &metav1.ObjectMeta{
	Name:      "example",
	Namespace: "predicate",
}
var mismatchObjectMeta = &metav1.ObjectMeta{
	Name:      "example",
	Namespace: "other",
}
var createEvent event.CreateEvent
var updateEvent event.UpdateEvent
var deleteEvent event.DeleteEvent
var genericEvent event.GenericEvent

var _ = Describe("NamespacedPredicate", func() {
	BeforeEach(func() {
		predicate = NamespacedPredicate{Namespace: "predicate"}
	})

	Context("When the namespace of the event matches the Predicate's", func() {
		BeforeEach(func() {
			createEvent = event.CreateEvent{Meta: matchingObjectMeta}
			updateEvent = event.UpdateEvent{MetaNew: matchingObjectMeta}
			deleteEvent = event.DeleteEvent{Meta: matchingObjectMeta}
			genericEvent = event.GenericEvent{Meta: matchingObjectMeta}
		})

		Context("Create", func() {
			It("returns true", func() {
				Expect(predicate.Create(createEvent)).To(BeTrue())
			})
		})

		Context("Update", func() {
			It("returns true", func() {
				Expect(predicate.Update(updateEvent)).To(BeTrue())
			})
		})

		Context("Delete", func() {
			It("returns true", func() {
				Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			})
		})

		Context("Generic", func() {
			It("returns true", func() {
				Expect(predicate.Generic(genericEvent)).To(BeTrue())
			})
		})
	})

	Context("When the namespace of the event doesn't match the Predicate's", func() {
		BeforeEach(func() {
			createEvent = event.CreateEvent{Meta: mismatchObjectMeta}
			updateEvent = event.UpdateEvent{MetaNew: mismatchObjectMeta}
			deleteEvent = event.DeleteEvent{Meta: mismatchObjectMeta}
			genericEvent = event.GenericEvent{Meta: mismatchObjectMeta}
		})

		Context("Create", func() {
			It("returns false", func() {
				Expect(predicate.Create(createEvent)).To(BeFalse())
			})
		})

		Context("Update", func() {
			It("returns false", func() {
				Expect(predicate.Update(updateEvent)).To(BeFalse())
			})
		})

		Context("Delete", func() {
			It("returns false", func() {
				Expect(predicate.Delete(deleteEvent)).To(BeFalse())
			})
		})

		Context("Generic", func() {
			It("returns false", func() {
				Expect(predicate.Generic(genericEvent)).To(BeFalse())
			})
		})
	})

	Context("When the Predicate's namespace is the empty string", func() {
		BeforeEach(func() {
			predicate = NamespacedPredicate{Namespace: ""}
		})

		Context("Create", func() {
			It("returns true", func() {
				Expect(predicate.Create(createEvent)).To(BeTrue())
			})
		})

		Context("Update", func() {
			It("returns true", func() {
				Expect(predicate.Update(updateEvent)).To(BeTrue())
			})
		})

		Context("Delete", func() {
			It("returns true", func() {
				Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			})
		})

		Context("Generic", func() {
			It("returns true", func() {
				Expect(predicate.Generic(genericEvent)).To(BeTrue())
			})
		})
	})

	Context("When the Events' namespace is the empty string", func() {
		BeforeEach(func() {
			emptyMeta := &metav1.ObjectMeta{Name: "example"}
			createEvent = event.CreateEvent{Meta: emptyMeta}
			updateEvent = event.UpdateEvent{MetaNew: emptyMeta}
			deleteEvent = event.DeleteEvent{Meta: emptyMeta}
			genericEvent = event.GenericEvent{Meta: emptyMeta}
		})

		Context("Create", func() {
			It("returns true", func() {
				Expect(predicate.Create(createEvent)).To(BeTrue())
			})
		})

		Context("Update", func() {
			It("returns true", func() {
				Expect(predicate.Update(updateEvent)).To(BeTrue())
			})
		})

		Context("Delete", func() {
			It("returns true", func() {
				Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			})
		})

		Context("Generic", func() {
			It("returns true", func() {
				Expect(predicate.Generic(genericEvent)).To(BeTrue())
			})
		})
	})
})
