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

package gittrackobject

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pusher/faros/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var (
	testHandler = &eventToChannelHandler{
		eventsChan: make(chan event.GenericEvent, 1),
	}
	unstructuredEventTest unstructured.Unstructured
	expectedEvent         = event.GenericEvent{
		Meta: &metav1.ObjectMeta{
			Name:      "example",
			Namespace: "default",
		},
		Object: &unstructuredEventTest,
	}
)

var _ = Describe("EventHandler Suite", func() {
	BeforeEach(func() {
		var err error
		unstructuredEventTest, err = utils.YAMLToUnstructured([]byte(exampleDeployment))
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("when OnAdd is called", func() {
		It("should send an event to the eventStream", func() {
			testHandler.OnAdd(&unstructuredEventTest)
			Eventually(testHandler.eventsChan, timeout).
				Should(Receive(Equal(expectedEvent)))
		})
	})

	Describe("when OnUpdate is called", func() {
		It("should send an event to the eventStream", func() {
			testHandler.OnUpdate(nil, &unstructuredEventTest)
			Eventually(testHandler.eventsChan, timeout).
				Should(Receive(Equal(expectedEvent)))
		})
	})

	Describe("when OnDelete is called", func() {
		It("should send an event to the eventStream", func() {
			testHandler.OnDelete(&unstructuredEventTest)
			Eventually(testHandler.eventsChan, timeout).
				Should(Receive(Equal(expectedEvent)))
		})
	})

	Describe("when an invalid object is used", func() {
		It("no event should be to the eventStream", func() {
			testHandler.OnAdd(nil)
			Expect(testHandler.eventsChan).To(BeEmpty())
		})
	})
})
