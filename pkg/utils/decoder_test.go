package utils_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pusher/faros/pkg/utils"
	"github.com/pusher/faros/test/reporters"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDecoder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Decoder Suite", reporters.Reporters())
}

var roleBinding = `---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kubernetes-dashboard-minimal
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kubernetes-dashboard-minimal
subjects:
- kind: ServiceAccount
  name: kubernetes-dashboard
  namespace: kube-system
`

var pdb = `---
apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  name: kubernetes-dashboard
  namespace: kube-system
spec:
  minAvailable: 0
  selector:
    matchLabels:
      k8s-app: kubernetes-dashboard
`

var cm = `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: hello-world
  namespace: default
data:
  file1.yml: |-
    hello: world
  file2.yml: |-
    ---
    world: hello
`

var broken = `---
metadata:
  name: { broken ]
  namespace: default
`

var mixedList = roleBinding + pdb

var _ = Describe("YAMLToUnstructured", func() {
	It("should convert the roleBinding to an unstructured roleBinding", func() {
		obj, err := YAMLToUnstructured([]byte(roleBinding))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(obj.GetKind()).To(Equal("RoleBinding"))
		Expect(obj.GetName()).To(Equal("kubernetes-dashboard-minimal"))
		Expect(obj.GetNamespace()).To(Equal("kube-system"))
		Expect(obj.IsList()).To(BeFalse())
	})

	It("should split a mixed list into a list of unstructureds", func() {
		obj, err := YAMLToUnstructured([]byte(mixedList))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(obj.IsList()).To(BeTrue())
		listItems := []runtime.Object{}
		obj.EachListItem(func(obj runtime.Object) error {
			listItems = append(listItems, obj)
			return nil
		})
		Expect(listItems).To(HaveLen(2))
		rb, ok := listItems[0].(*unstructured.Unstructured)
		Expect(ok).To(BeTrue())
		Expect(rb.GetKind()).To(Equal("RoleBinding"))
		pdb, ok := listItems[1].(*unstructured.Unstructured)
		Expect(ok).To(BeTrue())
		Expect(pdb.GetKind()).To(Equal("PodDisruptionBudget"))
	})

	It("should only split on unindented triple dashes", func() {
		obj, err := YAMLToUnstructured([]byte(cm))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(obj.GetKind()).To(Equal("ConfigMap"))
		Expect(obj.GetName()).To(Equal("hello-world"))
		Expect(obj.GetNamespace()).To(Equal("default"))
		Expect(obj.IsList()).To(BeFalse())
	})
})

var _ = Describe("YAMLToUnstructuredSlice", func() {
	Context("with a single valid YAML document with a separator", func() {
		It("returns a slice of one Unstructured object", func() {
			s, _ := YAMLToUnstructuredSlice([]byte(pdb))
			Expect(s).To(HaveLen(1))
			Expect(s[0].GetKind()).To(Equal("PodDisruptionBudget"))
		})

		It("does not return any error", func() {
			_, err := YAMLToUnstructuredSlice([]byte(pdb))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("with a single valid YAML document without a separator", func() {
		It("returns a slice of one Unstructured object", func() {
			s, _ := YAMLToUnstructuredSlice([]byte(pdb[4:]))
			Expect(s).To(HaveLen(1))
		})

		It("does not return any error", func() {
			_, err := YAMLToUnstructuredSlice([]byte(pdb[4:]))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("with a list of only valid YAML documents", func() {
		It("returns a slice of Unstructured objects", func() {
			s, _ := YAMLToUnstructuredSlice([]byte(mixedList))
			Expect(s).To(HaveLen(2))
			Expect(s[0].GetKind()).To(Equal("RoleBinding"))
			Expect(s[1].GetKind()).To(Equal("PodDisruptionBudget"))
		})

		It("does not return any error", func() {
			_, err := YAMLToUnstructuredSlice([]byte(mixedList))
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Context("with a list that has invalid YAML in the beginning of it", func() {
		It("returns an empty slice", func() {
			s, _ := YAMLToUnstructuredSlice([]byte(broken + pdb))
			Expect(s).To(BeEmpty())
		})

		It("returns an error", func() {
			_, err := YAMLToUnstructuredSlice([]byte(broken + pdb))
			Expect(err).To(HaveOccurred())
		})
	})

	Context("with a list that has invalid YAML in the middle of it", func() {
		It("returns the valid ones (up til the invalid one)", func() {
			s, _ := YAMLToUnstructuredSlice([]byte(roleBinding + broken + pdb))
			Expect(s).To(HaveLen(1))
			Expect(s[0].GetKind()).To(Equal("RoleBinding"))
		})

		It("returns an error", func() {
			_, err := YAMLToUnstructuredSlice([]byte(roleBinding + broken + pdb))
			Expect(err).To(HaveOccurred())
		})
	})

	Context("with a list that has invalid YAML at the end of it", func() {
		It("returns the valid ones", func() {
			s, _ := YAMLToUnstructuredSlice([]byte(roleBinding + pdb + broken))
			Expect(s).To(HaveLen(2))
			Expect(s[0].GetKind()).To(Equal("RoleBinding"))
			Expect(s[1].GetKind()).To(Equal("PodDisruptionBudget"))
		})

		It("returns an error", func() {
			_, err := YAMLToUnstructuredSlice([]byte(roleBinding + broken + pdb))
			Expect(err).To(HaveOccurred())
		})
	})
})
