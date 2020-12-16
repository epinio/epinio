package kubernetes_test

import (
	"bytes"
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/suse/carrier/cli/kubernetes"
)

var _ = Describe("InteractiveOptionsReader", func() {
	stdin := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	reader := NewInteractiveOptionsReader(stdout, stdin)

	option := InstallationOption{
		Name:        "Option",
		Value:       "",
		Description: "This is a very needed option",
		Type:        StringType,
	}

	Describe("Read", func() {
		It("prompts the user for input on stdin", func() {
			stdin.Write([]byte("userDefinedValue\n"))
			result, err := reader.Read(option)
			Expect(err).ToNot(HaveOccurred())

			prompt, err := ioutil.ReadAll(stdout)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(prompt)).To(ContainSubstring("This is a very needed option"))

			resultStr, ok := result.(string)
			Expect(ok).To(BeTrue())
			Expect(resultStr).To(Equal("userDefinedValue"))
		})

		When("the option is BooleanType", func() {
			option := InstallationOption{
				Name:        "Option",
				Value:       "",
				Description: "This is a boolean option",
				Type:        BooleanType,
			}

			It("returns a boolean", func() {
				stdin.Write([]byte("y\n"))
				result, err := reader.Read(option)
				Expect(err).ToNot(HaveOccurred())

				resultStr, ok := result.(bool)
				Expect(ok).To(BeTrue())
				Expect(resultStr).To(BeTrue())
			})

			It("asks again if the answer is not 'y' or 'n'", func() {
				stdin.Write([]byte("other\ny\n"))
				result, err := reader.Read(option)
				Expect(err).ToNot(HaveOccurred())

				prompt, err := ioutil.ReadAll(stdout)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(prompt)).To(
					ContainSubstring("It's either 'y' or 'n', please try again"))

				resultBool, ok := result.(bool)
				Expect(ok).To(BeTrue())
				Expect(resultBool).To(BeTrue())
			})
		})
	})
})
