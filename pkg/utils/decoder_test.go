package utils_test

import (
	"testing"

	"github.com/kubernetes-sigs/kubebuilder/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pusher/faros/pkg/utils"
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

var roleBindingExpect = `{"apiVersion":"rbac.authorization.k8s.io/v1","kind":"RoleBinding","metadata":{"name":"kubernetes-dashboard-minimal","namespace":"kube-system"},"roleRef":{"apiGroup":"rbac.authorization.k8s.io","kind":"Role","name":"kubernetes-dashboard-minimal"},"subjects":[{"kind":"ServiceAccount","name":"kubernetes-dashboard","namespace":"kube-system"}]}`

var _ = Describe("YAMLToJSON", func() {
	It("should convert the rolebinding without error", func() {
		rolebindingJSON, err := YAMLToJSON([]byte(roleBinding))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(rolebindingJSON).Should(Equal([]byte(roleBindingExpect)))
	})
})
