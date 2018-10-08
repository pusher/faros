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

package flags

import (
	"testing"

	"github.com/kubernetes-sigs/kubebuilder/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestFlagSet(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "FlagSet Suite", []Reporter{test.NewlineReporter{}})
}

var _ = Describe("FlagSet Suite", func() {
	Context("ParseIgnoredResources with valid GVR strings", func() {
		BeforeEach(func() {
			ignoredResources = []string{"deployments.apps/v1", "gittracks.faros.pusher.com/v1alpha1"}
		})

		It("doesn't error", func() {
			_, err := ParseIgnoredResources()
			Expect(err).NotTo(HaveOccurred())
		})

		It("parses deployment.apps/v1 into a GVR", func() {
			gvr := schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			}
			gvrs, _ := ParseIgnoredResources()
			_, ok := gvrs[gvr]
			Expect(ok).To(BeTrue())
		})

		It("parses gittrack.faros.pusher.com/v1alpha1 into a GVR", func() {
			gvr := schema.GroupVersionResource{
				Group:    "faros.pusher.com",
				Version:  "v1alpha1",
				Resource: "gittracks",
			}
			gvrs, _ := ParseIgnoredResources()
			_, ok := gvrs[gvr]
			Expect(ok).To(BeTrue())
		})
	})
})
