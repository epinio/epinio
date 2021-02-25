package kubernetes_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/suse/carrier/kubernetes"
)

var _ = Describe("DefaultOptionsReader", func() {
	reader := NewDefaultOptionsReader()

	Describe("Read", func() {
		// The tests here are essentially a copy of the tests
		// for `SetDefault` in files `options(_test).go`.  Any
		// new tests there should be replicated here.

		optionDynamic := InstallationOption{
			Name:         "TheName",
			DeploymentID: "SomeDeployment",
			Default:      "World",
			DynDefaultFunc: func(o *InstallationOption) error {
				o.Value = "Hello"
				return nil
			},
		}

		optionStatic := InstallationOption{
			Name:         "TheName",
			DeploymentID: "SomeDeployment",
			Default:      "World",
		}

		optionError := InstallationOption{
			Name:         "TheName",
			DeploymentID: "SomeDeployment",
			DynDefaultFunc: func(o *InstallationOption) error {
				o.Value = "Hello"
				return errors.New("an error")
			},
		}

		It("prefers the DynDefaultFunc over a static Default", func() {
			Expect(reader.Read(&optionDynamic)).To(BeNil())
			Expect(optionDynamic.Value).To(Equal("Hello"))
		})

		It("uses a static Default", func() {
			Expect(reader.Read(&optionStatic)).To(BeNil())
			Expect(optionStatic.Value).To(Equal("World"))
		})

		It("reports errors returned from the DynDefaultFunc", func() {
			err := reader.Read(&optionError)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("an error"))
		})
	})
})
