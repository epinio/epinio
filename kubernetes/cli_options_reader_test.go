package kubernetes_test

import (
	"github.com/spf13/cobra"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/suse/carrier/kubernetes"
)

var _ = Describe("CLIOptionsReader", func() {

	// We have two of each option/flag. Required because the
	// reader will modify the structure (set fields Value and
	// UserSpecified) when things are ok. The other definition is
	// for when defaults are seen, with proper initial values for
	// all fields. And to prevent concurrent access to the same
	// structure by concurrent tests.

	optionFlag := InstallationOption{
		Name:         "a_flag",
		DeploymentID: "",
		Default:      true,
		Type:         BooleanType,
	}

	optionFlagD := InstallationOption{
		Name:         "a_flag_d",
		DeploymentID: "",
		Default:      true,
		Type:         BooleanType,
	}

	optionString := InstallationOption{
		Name:         "a_text",
		DeploymentID: "",
		Default:      "",
		Type:         StringType,
	}

	optionStringD := InstallationOption{
		Name:         "a_text_d",
		DeploymentID: "",
		Default:      "",
		Type:         StringType,
	}

	optionInt := InstallationOption{
		Name:         "a_count",
		DeploymentID: "",
		Default:      -1,
		Type:         IntType,
	}

	optionIntD := InstallationOption{
		Name:         "a_count_d",
		DeploymentID: "",
		Default:      -1,
		Type:         IntType,
	}

	optionUnknown := InstallationOption{
		Name:         "without_a_cobra_flag",
		DeploymentID: "",
		Default:      -1,
		Type:         IntType,
	}

	dummyCmd := &cobra.Command{
		Use:   "dummy",
		Short: "test dummy",
		Long:  `this is a test dummy`,
	}

	dummyDefaultCmd := &cobra.Command{
		Use:   "dummy defaults",
		Short: "test dummy, flag defaults",
		Long:  `this is a test dummy for defaults`,
	}

	var options InstallationOptions = []InstallationOption{
		optionFlag,
		optionString,
		optionInt,
	}

	var optionsD InstallationOptions = []InstallationOption{
		optionFlagD,
		optionStringD,
		optionIntD,
	}

	options.AsCobraFlagsFor(dummyCmd)
	reader := NewCLIOptionsReader(dummyCmd)

	optionsD.AsCobraFlagsFor(dummyDefaultCmd)
	readerDefaults := NewCLIOptionsReader(dummyDefaultCmd)

	options = append(options, optionUnknown)

	// Fake having seen the flags on the command line.
	_ = dummyCmd.Flags().Set("a-flag", "false")
	_ = dummyCmd.Flags().Set("a-text", "some text")
	_ = dummyCmd.Flags().Set("a-count", "888")

	// Nothing to fake for dummyDefaultCmd, we want it to return
	// the flag defaults.

	Describe("Read", func() {
		When("handling an option without flag", func() {
			It("does nothing", func() {
				err := reader.Read(&optionUnknown)
				Expect(err).ToNot(HaveOccurred())
				Expect(optionUnknown.UserSpecified).To(BeFalse())
				Expect(optionUnknown.Value).To(BeNil())
			})
		})

		When("handling a boolean flag", func() {
			It("returns a boolean", func() {
				err := reader.Read(&optionFlag)
				Expect(err).ToNot(HaveOccurred())
				resultBool, ok := optionFlag.Value.(bool)
				Expect(ok).To(BeTrue())
				Expect(resultBool).To(BeFalse())
				Expect(optionFlag.UserSpecified).To(BeTrue())
			})

			It("does nothing for defaults", func() {
				err := readerDefaults.Read(&optionFlagD)
				Expect(err).ToNot(HaveOccurred())
				Expect(optionFlagD.Value).To(BeNil())
				Expect(optionFlagD.UserSpecified).To(BeFalse())
			})
		})

		When("handling a string flag", func() {
			It("returns a string", func() {
				err := reader.Read(&optionString)
				Expect(err).ToNot(HaveOccurred())
				resultString, ok := optionString.Value.(string)
				Expect(ok).To(BeTrue())
				Expect(resultString).To(Equal("some text"))
				Expect(optionString.UserSpecified).To(BeTrue())
			})

			It("does nothing for defaults", func() {
				err := readerDefaults.Read(&optionStringD)
				Expect(err).ToNot(HaveOccurred())
				Expect(optionStringD.Value).To(BeNil())
				Expect(optionStringD.UserSpecified).To(BeFalse())
			})
		})

		When("handling an integer flag", func() {
			It("returns an integer", func() {
				err := reader.Read(&optionInt)
				Expect(err).ToNot(HaveOccurred())
				resultInt, ok := optionInt.Value.(int)
				Expect(ok).To(BeTrue())
				Expect(resultInt).To(Equal(888))
				Expect(optionInt.UserSpecified).To(BeTrue())
			})

			It("does nothing for defaults", func() {
				err := readerDefaults.Read(&optionIntD)
				Expect(err).ToNot(HaveOccurred())
				Expect(optionIntD.Value).To(BeNil())
				Expect(optionIntD.UserSpecified).To(BeFalse())
			})
		})
	})
})
