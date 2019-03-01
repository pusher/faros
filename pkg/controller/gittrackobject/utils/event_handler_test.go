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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	testutils "github.com/pusher/faros/test/utils"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var _ = Describe("EventHandler Suite", func() {
	const timeout = 5 * time.Second

	var (
		testHandler = &EventToChannelHandler{
			EventsChan: make(chan event.GenericEvent, 1),
		}
		eventTest     unstructured.Unstructured
		expectedEvent = event.GenericEvent{
			Meta: &metav1.ObjectMeta{
				Name:      "example",
				Namespace: "default",
			},
			Object: &eventTest,
		}
	)

	BeforeEach(func() {
		content, err := runtime.NewTestUnstructuredConverter(apiequality.Semantic).ToUnstructured(testutils.ExampleDeployment.DeepCopy())
		Expect(err).NotTo(HaveOccurred())
		eventTest.SetUnstructuredContent(content)
	})

	Describe("when OnAdd is called", func() {
		It("should send an event to the eventStream", func() {
			testHandler.OnAdd(&eventTest)
			Eventually(testHandler.EventsChan, timeout).
				Should(Receive(Equal(expectedEvent)))
		})
	})

	Describe("when OnUpdate is called", func() {
		It("should send an event to the eventStream", func() {
			testHandler.OnUpdate(nil, &eventTest)
			Eventually(testHandler.EventsChan, timeout).
				Should(Receive(Equal(expectedEvent)))
		})
	})

	Describe("when OnDelete is called", func() {
		It("should send an event to the eventStream", func() {
			testHandler.OnDelete(&eventTest)
			Eventually(testHandler.EventsChan, timeout).
				Should(Receive(Equal(expectedEvent)))
		})
	})

	Describe("when an invalid object is used", func() {
		It("no event should be to the eventStream", func() {
			testHandler.OnAdd(nil)
			Expect(testHandler.EventsChan).To(BeEmpty())
		})
	})
})
