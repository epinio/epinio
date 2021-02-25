package kubernetes_test

import (
	"bytes"
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/suse/carrier/kubernetes"
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

	optionSpecified := InstallationOption{
		Name:          "Option",
		Value:         "User Specified Dummy",
		Description:   "This option got a user specified value",
		Type:          StringType,
		UserSpecified: true,
	}

	optionDefault := InstallationOption{
		Name:        "Option",
		Value:       "",
		Default:     "Hello World",
		Description: "This is a very needed option",
		Type:        StringType,
	}

	optionDefaultKeep := InstallationOption{
		Name:        "Option",
		Value:       "",
		Default:     "Hello World",
		Description: "This is a very needed option",
		Type:        StringType,
	}

	Describe("Read", func() {
		It("ignores an option with a user-specified value", func() {
			err := reader.Read(&optionSpecified)
			Expect(err).ToNot(HaveOccurred())

			// Verify that there was neither prompt nor other output
			prompt, err := ioutil.ReadAll(stdout)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(prompt)).To(Equal(""))

			// Verify that the option value was not touched
			resultStr, ok := optionSpecified.Value.(string)
			Expect(ok).To(BeTrue())
			Expect(resultStr).To(Equal("User Specified Dummy"))
			Expect(optionSpecified.UserSpecified).To(BeTrue())
		})

		It("prompts the user for input on stdin", func() {
			stdin.Write([]byte("userDefinedValue\n"))
			err := reader.Read(&option)
			Expect(err).ToNot(HaveOccurred())

			prompt, err := ioutil.ReadAll(stdout)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(prompt)).To(ContainSubstring("This is a very needed option"))

			resultStr, ok := option.Value.(string)
			Expect(ok).To(BeTrue())
			Expect(resultStr).To(Equal("userDefinedValue"))
			Expect(option.UserSpecified).To(BeTrue())
		})

		It("shows the default in the prompt", func() {
			stdin.Write([]byte("userDefinedValue\n"))
			err := reader.Read(&optionDefault)
			Expect(err).ToNot(HaveOccurred())

			prompt, err := ioutil.ReadAll(stdout)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(prompt)).To(ContainSubstring("Hello World"))

			resultStr, ok := optionDefault.Value.(string)
			Expect(ok).To(BeTrue())
			Expect(resultStr).To(Equal("userDefinedValue"))
			Expect(optionDefault.UserSpecified).To(BeTrue())
		})

		It("keeps the default when no value is entered by the user", func() {
			stdin.Write([]byte("\n"))
			err := reader.Read(&optionDefaultKeep)
			Expect(err).ToNot(HaveOccurred())

			prompt, err := ioutil.ReadAll(stdout)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(prompt)).To(ContainSubstring("Hello World"))

			resultStr, ok := optionDefaultKeep.Value.(string)
			Expect(ok).To(BeTrue())
			Expect(resultStr).To(Equal("Hello World"))
			Expect(optionDefaultKeep.UserSpecified).To(BeTrue())
		})

		When("the option is BooleanType", func() {
			var option InstallationOption

			BeforeEach(func() {
				option = InstallationOption{
					Name:        "Option",
					Value:       "",
					Description: "This is a boolean option",
					Type:        BooleanType,
				}
			})

			It("returns a boolean", func() {
				stdin.Write([]byte("y\n"))
				err := reader.Read(&option)
				Expect(err).ToNot(HaveOccurred())

				resultBool, ok := option.Value.(bool)
				Expect(ok).To(BeTrue())
				Expect(resultBool).To(BeTrue())
				Expect(option.UserSpecified).To(BeTrue())
			})

			It("asks again if the answer is not 'y' or 'n'", func() {
				stdin.Write([]byte("other\ny\n"))
				err := reader.Read(&option)
				Expect(err).ToNot(HaveOccurred())

				prompt, err := ioutil.ReadAll(stdout)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(prompt)).To(
					ContainSubstring("It's either 'y' or 'n', please try again"))

				resultBool, ok := option.Value.(bool)
				Expect(ok).To(BeTrue())
				Expect(resultBool).To(BeTrue())
				Expect(option.UserSpecified).To(BeTrue())
			})
		})

		When("the option is IntType", func() {
			var option InstallationOption

			BeforeEach(func() {
				option = InstallationOption{
					Name:        "Option",
					Value:       "",
					Description: "This is an integer option",
					Type:        IntType,
				}
			})

			It("returns an integer", func() {
				stdin.Write([]byte("55\n"))
				err := reader.Read(&option)
				Expect(err).ToNot(HaveOccurred())

				resultInt, ok := option.Value.(int)
				Expect(ok).To(BeTrue())
				Expect(resultInt).To(Equal(55))
				Expect(option.UserSpecified).To(BeTrue())
			})

			It("asks again if the answer is not an integer", func() {
				stdin.Write([]byte("other\n66\n"))
				err := reader.Read(&option)
				Expect(err).ToNot(HaveOccurred())

				prompt, err := ioutil.ReadAll(stdout)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(prompt)).To(
					ContainSubstring("Please provide an integer value"))

				resultInt, ok := option.Value.(int)
				Expect(ok).To(BeTrue())
				Expect(resultInt).To(Equal(66))
				Expect(option.UserSpecified).To(BeTrue())
			})
		})

		When("the option is bogus", func() {
			var option InstallationOption

			BeforeEach(func() {
				option = InstallationOption{
					Name:        "Option",
					Value:       "",
					Description: "This is a bogus option with a bogus type",
					Type:        22,
				}
			})

			It("returns an error", func() {
				err := reader.Read(&option)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Internal error: option Type not supported"))
			})
		})
	})
})
