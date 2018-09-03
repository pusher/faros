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

package utils

import (
	"testing"

	"github.com/kubernetes-sigs/kubebuilder/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/event"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNamespacedPredicate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "NamespacedPredicate Suite", []Reporter{test.NewlineReporter{}})
}

var _ = Describe("NamespacedPredicate", func() {
	var (
		predicate    NamespacedPredicate
		createEvent  event.CreateEvent
		updateEvent  event.UpdateEvent
		deleteEvent  event.DeleteEvent
		genericEvent event.GenericEvent
	)

	BeforeEach(func() {
		predicate = NamespacedPredicate{Namespace: "predicate"}
	})

	Context("When the namespace of the event matches the Predicate's", func() {
		BeforeEach(func() {
			matchingMeta := &metav1.ObjectMeta{Name: "example", Namespace: "predicate"}
			createEvent = event.CreateEvent{Meta: matchingMeta}
			updateEvent = event.UpdateEvent{MetaNew: matchingMeta}
			deleteEvent = event.DeleteEvent{Meta: matchingMeta}
			genericEvent = event.GenericEvent{Meta: matchingMeta}
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
			mismatchMeta := &metav1.ObjectMeta{Name: "example", Namespace: "other"}
			createEvent = event.CreateEvent{Meta: mismatchMeta}
			updateEvent = event.UpdateEvent{MetaNew: mismatchMeta}
			deleteEvent = event.DeleteEvent{Meta: mismatchMeta}
			genericEvent = event.GenericEvent{Meta: mismatchMeta}
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
			otherMeta := &metav1.ObjectMeta{Name: "example", Namespace: "other"}
			createEvent = event.CreateEvent{Meta: otherMeta}
			updateEvent = event.UpdateEvent{MetaNew: otherMeta}
			deleteEvent = event.DeleteEvent{Meta: otherMeta}
			genericEvent = event.GenericEvent{Meta: otherMeta}
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
			emptyMeta := &metav1.ObjectMeta{Name: "example", Namespace: ""}
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
