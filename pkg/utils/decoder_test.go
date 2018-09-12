package utils_test

import (
	"testing"

	"github.com/kubernetes-sigs/kubebuilder/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pusher/faros/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDecoder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Decoder Suite", []Reporter{test.NewlineReporter{}})
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

var mixedList = roleBinding + pdb

var _ = Describe("YAMLToUnstructured", func() {
	It("should convert the roleBinding to an unstructured roleBindung", func() {
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
		Expect(len(listItems)).To(Equal(2))
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
	It("should return a slice of Unstructured objects", func() {
		s, err := YAMLToUnstructuredSlice([]byte(mixedList))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(s)).To(Equal(2))
		Expect(s[0].GetKind()).To(Equal("RoleBinding"))
		Expect(s[1].GetKind()).To(Equal("PodDisruptionBudget"))
	})
})
