package utils

import (
	"testing"

	. "github.com/benjamintf1/unmarshalledmatchers"
	"github.com/kubernetes-sigs/kubebuilder/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	emptyPatch                            = `[]`
	empty                                 = `{}`
	invalid                               = `{hello}`
	nameFoo                               = `{"name":"foo"}`
	nameBar                               = `{"name":"bar"}`
	nameFooWithExtra                      = `{"name":"foo","extra":"field"}`
	nameFooWithExtraAsArray               = `{"name":"foo","extra":["one","two","three"]}`
	nameFooWithExtraAsObject              = `{"name":"foo","extra":{"sub":"field"}}`
	nameFooWithExtraAsObjectWithTwoFields = `{"name":"foo","extra":{"sub":"field","second":"field"}}`
	nameFooWithBlacklisted                = `{"name":"foo","metadata":{"creationTimestamp":null}}`
	nameFooWithTimestamp                  = `{"name":"foo","metadata":{"creationTimestamp":"123"}}`
	nameBarWithExtra                      = `{"name":"bar","extra":"field"}`
	nameBazWithExtra                      = `{"name":"baz","extra":"extra"}`
	addExtra                              = `[{"op":"add","path":"/extra","value":"field"}]`
	addExtraSecondField                   = `[{"op":"add","path":"/extra/second","value":"field"}]`
	removeExtra                           = `[{"op":"remove","path":"/extra"}]`
	removeExtraSecondField                = `[{"op":"remove","path":"/extra/second"}]`
	replaceNameFoo                        = `[{"op":"replace","path":"/name","value":"foo"}]`
	replaceNameBazReplaceExtraExtra       = `[{"op":"replace","path":"/name","value":"baz"},{"op":"replace","path":"/extra","value":"extra"}]`
)

func TestThreeWayMerge(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Three Way Merge Suite", []Reporter{test.NewlineReporter{}})
}

var _ = Describe("createThreeWayJSONMergePatch", func() {
	It("should overwrite a field changed in current", func() {
		testCreateThreeWayJSONMergePatch(nameFoo, nameFoo, nameBar, replaceNameFoo)
	})

	It("should ignore extra field in current", func() {
		testCreateThreeWayJSONMergePatch(nameFoo, nameFoo, nameFooWithExtra, emptyPatch)
	})

	It("should remove extra field when removed from modified", func() {
		testCreateThreeWayJSONMergePatch(nameFooWithExtra, nameFoo, nameFooWithExtra, removeExtra)
	})

	It("should add a field when added to to modified", func() {
		testCreateThreeWayJSONMergePatch(nameFoo, nameFooWithExtra, nameFoo, addExtra)
	})

	Context("within a map", func() {
		It("should ignore extra field in current", func() {
			testCreateThreeWayJSONMergePatch(nameFooWithExtraAsObject, nameFooWithExtraAsObject, nameFooWithExtraAsObjectWithTwoFields, emptyPatch)
		})

		It("should remove extra field when removed from modified", func() {
			testCreateThreeWayJSONMergePatch(nameFooWithExtraAsObjectWithTwoFields, nameFooWithExtraAsObject, nameFooWithExtraAsObjectWithTwoFields, removeExtraSecondField)
		})

		It("should add a field when added to to modified", func() {
			testCreateThreeWayJSONMergePatch(nameFooWithExtraAsObject, nameFooWithExtraAsObjectWithTwoFields, nameFooWithExtraAsObject, addExtraSecondField)
		})
	})

	Context("when original is empty", func() {
		It("should overwrite a field changed in current", func() {
			testCreateThreeWayJSONMergePatch(empty, nameFoo, nameBar, replaceNameFoo)
		})

		It("should ignore extra field in current", func() {
			testCreateThreeWayJSONMergePatch(empty, nameFoo, nameFooWithExtra, emptyPatch)
		})

		It("should add a field when added to to modified", func() {
			testCreateThreeWayJSONMergePatch(empty, nameFooWithExtra, nameFoo, addExtra)
		})
	})

	It("should filter blacklisted paths", func() {
		testCreateThreeWayJSONMergePatch(nameFoo, nameFooWithBlacklisted, nameFooWithTimestamp, emptyPatch)
	})

	It("should converge to modifed when all are different", func() {
		testCreateThreeWayJSONMergePatch(nameFoo, nameBazWithExtra, nameBarWithExtra, replaceNameBazReplaceExtraExtra)
	})

	It("should return an error when invalid JSON is passed in", func() {
		testCreateThreeWayJSONMergePatchError(invalid, nameFoo, nameFooWithExtra)
		testCreateThreeWayJSONMergePatchError(nameFooWithExtra, invalid, nameFoo)
		testCreateThreeWayJSONMergePatchError(nameFoo, nameFooWithExtra, invalid)
	})
})

var testCreateThreeWayJSONMergePatch = func(original, modified, current, expect string) {
	patchBytes, err := createThreeWayJSONMergePatch(
		[]byte(original),
		[]byte(modified),
		[]byte(current),
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(patchBytes).To(MatchUnorderedJSON(expect, WithOrderedListKeys("path")))
}

var testCreateThreeWayJSONMergePatchError = func(original, modified, current string) {
	patchBytes, err := createThreeWayJSONMergePatch(
		[]byte(original),
		[]byte(modified),
		[]byte(current),
	)
	Expect(err).To(HaveOccurred())
	Expect(patchBytes).To(Equal([]byte{}))
}
